//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-admin/operator/internal/data"
	"go-admin/operator/internal/server"
	"go-admin/operator/internal/service"
)

// initApp generates the operator dependency graph and resource cleanup path.
func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	panic(wire.Build(data.ProviderSet, service.ProviderSet, server.ProviderSet, newApp))
}
