package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	adminbootstrap "go-admin/admin/internal/bootstrap"
	"go-admin/platform/pkg/runtimeconfig"
	"go-admin/platform/pkg/serviceid"
)

var version = "1.0.0"

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	boot, closeConfig, err := runtimeconfig.NewContext(ctx, serviceid.AdminService, version)
	if err != nil {
		return err
	}
	defer closeConfig()
	servers, cleanup, err := adminbootstrap.New(boot, register)
	if err != nil {
		return err
	}
	defer cleanup()
	return adminbootstrap.Run(ctx, boot, servers)
}
func main() {
	if err := run(); err != nil {
		slog.Error("admin stopped", "error", err)
		os.Exit(1)
	}
}
