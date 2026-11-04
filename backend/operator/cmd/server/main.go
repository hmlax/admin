package main

import (
	"go-admin/platform/pkg/runtimeconfig"
	"go-admin/platform/pkg/serviceid"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/transport/asynq"
)

var version = "1.0.0"

// workerLifecycle starts and drains tasks without advertising a network endpoint.
// Asynq's empty endpoint is not a gRPC address and must not enter Nacos.
type workerLifecycle struct {
	transport.Server
}

// go build -ldflags "-X main.version=x.y.z"

func newApp(ctx *bootstrap.Context, gs *grpc.Server, as *asynq.Server) *kratos.App {
	servers := []transport.Server{gs}
	if as != nil {
		servers = append(servers, &workerLifecycle{Server: as})
	}
	opts := runtimeconfig.Options(ctx)
	opts = append(opts, kratos.Server(servers...))
	return kratos.New(opts...)
}
func runApp() error {
	return runtimeconfig.Run(serviceid.OperatorService, version, initApp)
}

func main() {
	if err := runApp(); err != nil {
		panic(err)
	}
}
