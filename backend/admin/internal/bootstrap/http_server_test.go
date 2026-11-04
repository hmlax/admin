package bootstrap

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"io"
	"net/http"
	"testing"
	"time"

	apiV1 "go-admin/admin/biz/model/admin/v1"
	operatorV1 "go-admin/platform/pb/operator"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestHertzBridgePreservesNotFoundJSON(t *testing.T) {
	server, err := NewHTTPServer("127.0.0.1:0", "", &conf.Bootstrap{})
	if err != nil {
		t.Fatal(err)
	}
	server.router.Any("/*path", func(_ context.Context, c *app.RequestContext) {
		failRequest(c, operatorV1.ErrorIdentityNotFound("user not found"))
	})
	done := make(chan error, 1)
	go func() { done <- server.Start(t.Context()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Errorf("stop Hertz server: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Hertz server exited: %v", err)
			}
		case <-ctx.Done():
			t.Error("Hertz server did not stop")
		}
	})
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
	for _, path := range []string{"/", "/admin/v1/users/99"} {
		response, err := client.Get(server.endpoint.String() + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		var failure apiV1.HTTPErrorResponse
		if err := protojson.Unmarshal(body, &failure); err != nil {
			t.Fatalf("GET %s must return one JSON error: %q: %v", path, body, err)
		}
		if response.StatusCode != http.StatusNotFound || failure.GetCode() != 404 || failure.GetReason() != "NOT_FOUND" {
			t.Errorf("GET %s = %d %v, want structured 404", path, response.StatusCode, &failure)
		}
	}
}
