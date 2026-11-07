package main

import (
	"go-admin/platform/pkg/runtimeconfig"
	"go-admin/platform/pkg/serviceid"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

var version = "1.0.0"

func newApp(ctx *bootstrap.Context, server *grpc.Server) *kratos.App {
	options := runtimeconfig.Options(ctx)
	options = append(options, kratos.Server(server))
	return kratos.New(options...)
}

func runApp() error { return runtimeconfig.Run(serviceid.OperatorService, version, initApp) }
func main() {
	if err := runApp(); err != nil {
		panic(err)
	}
}
