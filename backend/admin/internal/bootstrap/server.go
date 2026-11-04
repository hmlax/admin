package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	hertz "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/go-kratos/kratos/v2/registry"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/redis/go-redis/v9"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-admin/admin/biz/handler"
	apiV1 "go-admin/admin/biz/model/admin/v1"
	"go-admin/admin/biz/mw"
	"go-admin/admin/biz/router"
	"go-admin/admin/internal/transport"
	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/rpcbridge"
	"go-admin/platform/pkg/runtimeconfig"
	"go-admin/platform/pkg/security"
	"go-admin/platform/pkg/serviceid"
	"google.golang.org/grpc"
)

// HTTPServer owns a native Hertz router and listener for its entire lifetime.
type HTTPServer struct {
	hertz    *hertz.Hertz
	router   *transport.Router
	listener net.Listener
	endpoint *url.URL
	cancel   context.CancelFunc
}

func NewHTTPServer(address, advertised string, cfg *conf.Bootstrap, middleware ...app.HandlerFunc) (*HTTPServer, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	endpoint, err := runtimeconfig.Endpoint("http", listener.Addr().String(), advertised)
	if err != nil {
		return nil, errors.Join(err, listener.Close())
	}
	base, cancel := context.WithCancel(context.Background())
	router, err := transport.NewRouter(cfg, listener, func(net.Conn) context.Context { return base }, middleware...)
	if err != nil {
		cancel()
		return nil, errors.Join(err, listener.Close())
	}
	return &HTTPServer{
		hertz:    router.Hertz,
		router:   router,
		listener: listener,
		endpoint: endpoint,
		cancel:   cancel,
	}, nil
}
func (s *HTTPServer) Start(context.Context) error    { return s.hertz.Run() }
func (s *HTTPServer) Stop(ctx context.Context) error { s.cancel(); return s.hertz.Shutdown(ctx) }
func (s *HTTPServer) Endpoint() (*url.URL, error)    { return s.endpoint, nil }

type RegisterRoutes func(*transport.Router, *handler.Services)

func New(ctx *bootstrap.Context, register RegisterRoutes) ([]*HTTPServer, func(), error) {
	cfg := ctx.GetConfig()
	settings := runtimeconfig.SettingsFromContext(ctx)
	var cleanup []func()
	closeAll := func() {
		for i := len(cleanup) - 1; i >= 0; i-- {
			cleanup[i]()
		}
	}
	target := strings.TrimSpace(settings.OperatorEndpoint)
	if target == "" {
		target = "127.0.0.1:7790"
	}
	opts := []kgrpc.ClientOption{
		kgrpc.WithEndpoint(target),
		kgrpc.WithTimeout(cfg.GetServer().GetGrpc().GetTimeout().AsDuration()),
		kgrpc.WithMiddleware(rpcbridge.Client()),
		kgrpc.WithUnaryInterceptor(rpcbridge.ReplyHeaders()),
		kgrpc.WithOptions(grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(70<<20), grpc.MaxCallSendMsgSize(70<<20))),
	}
	if reg := runtimeconfig.Registrar(ctx); reg != nil {
		discovery, ok := reg.(registry.Discovery)
		if !ok {
			return nil, nil, fmt.Errorf("registry does not support discovery")
		}
		if strings.TrimSpace(settings.OperatorEndpoint) == "" {
			opts[0] = kgrpc.WithEndpoint("discovery:///" + serviceid.NewDiscoveryName(serviceid.OperatorService) + ".grpc")
		}
		opts = append(opts, kgrpc.WithDiscovery(discovery))
	}
	conn, err := kgrpc.DialInsecure(ctx.Context(), opts...)
	if err != nil {
		return nil, nil, err
	}
	cleanup = append(cleanup, func() {
		if err := conn.Close(); err != nil {
			ctx.GetLogger().Error(ctx.Context(), err.Error())
		}
	})
	redisConfig := cfg.GetData().GetRedis()
	cache := redis.NewClient(&redis.Options{
		Addr:     redisConfig.GetAddr(),
		Password: redisConfig.GetPassword(),
		DB:       int(redisConfig.GetDb()),
	})
	cleanup = append(cleanup, func() {
		if err := cache.Close(); err != nil {
			ctx.GetLogger().Error(ctx.Context(), err.Error())
		}
	})
	tokenCache := security.NewUserTokenCache(ctx, cache)
	authenticator := security.NewAuthenticator(ctx, tokenCache)
	checker := security.NewTokenChecker(ctx, authenticator, operatorV1.ClientType_admin)
	gateway := operatorV1.NewGatewayServiceClient(conn)
	middlewares, err := mw.NewRestMiddleware(ctx, checker, mw.NewTenantChecker(gateway), gateway)
	if err != nil {
		closeAll()
		return nil, nil, err
	}
	api, err := NewHTTPServer(cfg.GetServer().GetRest().GetAddr(), settings.HTTPAdvertiseAddr, cfg, middlewares...)
	if err != nil {
		closeAll()
		return nil, nil, err
	}
	apiRouter := api.router
	cleanup = append(cleanup, func() {
		api.cancel()
		if err := api.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Warn("close HTTP listener", "error", err)
		}
	})
	services := handler.NewServices(conn, checker)
	services.EnableSwagger = cfg.GetServer().GetRest().GetEnableSwagger()
	register(apiRouter, services)
	servers := []*HTTPServer{api}
	if sseCfg := cfg.GetServer().GetSse(); sseCfg != nil && sseCfg.GetAddr() != "" {
		if sseCfg.GetPath() != "" && sseCfg.GetPath() != apiV1.EventServiceSubscribeHTTPPath {
			closeAll()
			return nil, nil, fmt.Errorf("SSE path is defined by admin/idl/proto/admin/v1/events.proto; run make hz to regenerate the contract")
		}
		stream, err := NewHTTPServer(sseCfg.GetAddr(), settings.SSEAdvertiseAddr, cfg)
		if err != nil {
			closeAll()
			return nil, nil, err
		}
		router.RegisterEventServiceHertzServer(stream.router, services.EventService)
		cleanup = append(cleanup, func() {
			stream.cancel()
			if err := stream.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				slog.Warn("close SSE listener", "error", err)
			}
		})
		servers = append(servers, stream)
	}
	return servers, closeAll, nil
}
