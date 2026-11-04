package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterCRUDUpdatesBothProjectsIdempotently(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "operator")
	wiring := filepath.Join(app, "cmd/server/wiring.go")
	manifestPath := filepath.Join(root, "platform/tools/gateway-services.json")
	if err := os.MkdirAll(filepath.Dir(wiring), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	source := "// ── register:repo ── 新模块仓储在此行后注册(make register 工具锚点,勿删)\n// ── register:service ── 新模块服务在此行后注册(make register 工具锚点,勿删)\n// register:rest-arg ── 新模块服务实参在此行后追加(make register 工具锚点,勿删)\n"
	manifest := gatewayManifest{
		Parameters:      "// register:param ── 新模块服务形参在此行后注册(make register 工具锚点,勿删)",
		Implementations: map[string]string{},
		Services:        map[string]string{},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wiring, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}
	if err := registerCRUD("product", app); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(wiring)
	if err != nil {
		t.Fatal(err)
	}
	if err := registerCRUD("product", app); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(wiring)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || strings.Count(string(second), "data.NewProductRepo") != 1 {
		t.Fatal("re-registering a CRUD module duplicated operator construction")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var out gatewayManifest
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Services["ProductService"] != "productService" || out.Implementations["productService"] != "ProductService" || strings.Count(out.Parameters, "productService *service.ProductService,") != 1 {
		t.Fatal("registration did not declare the shared admin/operator transport contract")
	}
}
