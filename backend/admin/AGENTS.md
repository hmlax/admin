# admin — Hertz

- 项目入口在根目录 `main.go`，路由接线为 `router_gen.go → biz/router/register.go`。使用 `go run .`，不要只运行 main.go。
- `biz/handler`：HTTP 处理、独立 API protobuf 参数绑定、显式字段转换与 operator gRPC 代理。
- `biz/model/admin/v1`：从本项目 IDL 生成的独立 API protobuf；不得 import `platform/pb/operator`、使用类型别名或把 RPC 消息直接作为 API 请求/响应。service protobuf 保留在 `../platform/proto/operator/v1`，通过显式转换调用。标准 Google protobuf 消息可以共用。
- `biz/router`：从 HTTP IDL 经官方 `hz update` 生成的服务路由。`router.go` 仅扩展文档等非业务路由。
- `biz/mw`：JWT、RBAC、租户入口与审计；`internal/transport`：通用 HTTP 编解码及路径元数据；`internal/bootstrap`：Nacos、客户端、监听及优雅退出。
- HTTP 契约只维护 `idl/proto/admin/v1/*.proto`。在 backend 执行 `make hz`，更新 operator 消息/RPC 时执行 `make api`，OpenAPI/TS 用 `make openapi ts`。禁止手改生成文件；API 与 RPC 各自拥有业务 DTO。
- 与 operator 共用 backend/go.mod。验证使用 `go test ./admin/... ./platform/tools/httpgen/...`、`go build -o /tmp/go-admin-check ./admin`；完整后端验证遵循上级 AGENTS.md。
