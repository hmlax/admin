package main

import (
	"go-admin/admin/biz/handler"
	"go-admin/admin/biz/router"
	"go-admin/admin/internal/transport"
)

// customizedRegister mounts non-business routes; business routes belong in IDL.
func customizedRegister(h *transport.Router, services *handler.Services) {
	if services.EnableSwagger {
		router.RegisterSwagger(h)
	}
}
