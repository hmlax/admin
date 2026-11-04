package bootstrap

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/middleware/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type cancellableEvents struct {
	operatorV1.EventServiceClient
	stopped chan struct{}
}

func (c *cancellableEvents) Subscribe(ctx context.Context, _ *operatorV1.EventSubscriptionRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[operatorV1.EventFrame], error) {
	return &grpc.GenericClientStream[operatorV1.EventSubscriptionRequest, operatorV1.EventFrame]{ClientStream: &eventClientStream{
		ctx:     ctx,
		stopped: c.stopped,
	}}, nil
}

type eventClientStream struct {
	ctx     context.Context // grpc.ClientStream.Context requires retaining the stream's lifetime.
	stopped chan struct{}
	sent    bool
}

func (s *eventClientStream) Header() (metadata.MD, error) { return nil, nil }
func (s *eventClientStream) Trailer() metadata.MD         { return nil }
func (s *eventClientStream) CloseSend() error             { return nil }
func (s *eventClientStream) Context() context.Context     { return s.ctx }
func (s *eventClientStream) SendMsg(any) error            { return nil }
func (s *eventClientStream) RecvMsg(message any) error {
	if !s.sent {
		s.sent = true
		message.(*operatorV1.EventFrame).Comment = []byte("connected")
		return nil
	}
	<-s.ctx.Done()
	close(s.stopped)
	return s.ctx.Err()
}

func TestNativeSSEFlushesAndCancelsOnDisconnect(t *testing.T) {
	srv, err := NewHTTPServer("127.0.0.1:0", "", &conf.Bootstrap{})
	if err != nil {
		t.Fatal(err)
	}
	client := &cancellableEvents{stopped: make(chan struct{})}
	checker := auth.NewAccessTokenCheckerFromFuncs(func(_ context.Context, token string, _ bool) (bool, *operatorV1.UserTokenPayload) {
		return token == "allowed", &operatorV1.UserTokenPayload{UserId: 42}
	}, nil)
	RegisterEventServiceHertzServer(srv.router, newEventHTTPService(client, checker, operatorV1.NewGatewayServiceClient(nil)))
	done := make(chan error, 1)
	go func() { done <- srv.Start(t.Context()) }()
	t.Cleanup(func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := srv.Stop(shutdown); err != nil {
			t.Errorf("stop SSE: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve SSE: %v", err)
			}
		case <-shutdown.Done():
			t.Error("SSE listener did not stop")
		}
	})
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.endpoint.String()+"/events?stream=42", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer allowed")
	httpClient := &http.Client{Timeout: 3 * time.Second}
	defer httpClient.CloseIdleConnections()
	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || line != ": connected\n" {
		t.Fatalf("first SSE frame = %d %q", response.StatusCode, line)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("browser disconnect did not cancel operator stream")
	}
}

func TestNativeLifecycleClosesSiblingListenerOnStartupFailure(t *testing.T) {
	servers := make([]*HTTPServer, 0, 2)
	for range 2 {
		srv, err := NewHTTPServer("127.0.0.1:0", "", &conf.Bootstrap{})
		if err != nil {
			t.Fatal(err)
		}
		servers = append(servers, srv)
	}
	failure := errors.New("startup hook failed")
	servers[1].hertz.OnRun = append(servers[1].hertz.OnRun, func(context.Context) error { return failure })
	boot := bootstrap.NewContextWithParam(t.Context(), &conf.AppInfo{AppId: "admin"}, &conf.Bootstrap{}, bLogger.NopLogger())
	if err := Run(t.Context(), boot, servers); !errors.Is(err, failure) {
		t.Fatalf("startup error = %v", err)
	}
	for _, srv := range servers {
		listener, err := net.Listen("tcp", srv.listener.Addr().String())
		if err != nil {
			t.Fatalf("listener was leaked: %v", err)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
