// Package httpmeta carries HTTP request facts across native transports and RPC clients.
package httpmeta

import (
	"context"
	"net/http"

	"github.com/go-kratos/kratos/v2/transport"
)

type contextKey struct{}

// Exchange contains request-scoped wire metadata, never a framework router or handler.
type Exchange struct {
	RequestValue  *http.Request
	Response      http.Header
	OperationName string
	Route         string
}

func NewContext(ctx context.Context, exchange *Exchange) context.Context {
	return context.WithValue(ctx, contextKey{}, exchange)
}

func FromContext(ctx context.Context) (*Exchange, bool) {
	value, ok := ctx.Value(contextKey{}).(*Exchange)
	return value, ok && value != nil
}

func (e *Exchange) Request() *http.Request          { return e.RequestValue }
func (e *Exchange) Operation() string               { return e.OperationName }
func (e *Exchange) PathTemplate() string            { return e.Route }
func (e *Exchange) RequestHeader() transport.Header { return header(e.RequestValue.Header) }
func (e *Exchange) ReplyHeader() transport.Header   { return header(e.Response) }

type header http.Header

func (h header) Get(key string) string { return http.Header(h).Get(key) }
func (h header) Set(key, value string) { http.Header(h).Set(key, value) }
func (h header) Add(key, value string) { http.Header(h).Add(key, value) }
func (h header) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
func (h header) Values(key string) []string { return http.Header(h).Values(key) }
