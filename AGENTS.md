# AGENTS.md — React / admin / operator

本分支只维护 React。此前 Vue 两端与三端门禁已按项目要求移除。

## 布局与职责

- `frontend/platform-admin/`：平台管理后台；`frontend/merchant-admin/`：商户管理后台。两套 React 19 + Ant Design 应用独立启动、构建和部署，共享 `frontend/src/`。开发前读 `frontend/AGENTS.md`。
- `backend/admin/`：Hertz HTTP API（7788）；负责 JWT、Redis 会话吊销、RBAC、租户入口门禁、字段权限及审计。
- `backend/operator/`：Kratos gRPC（7790）；负责业务、GORM、Redis、对象存储，不提供 HTTP 监听。
- `backend/platform/`：公共依赖、共享协议和生成代码、配置安全库、代码生成及部署工具。backend 下只保留 admin、operator、platform 三个顶级目录。
- 两个后端项目共用 `backend/go.mod`，各自维护独立的 API/RPC protobuf 契约、构建和部署。
- operator 的 gRPC 数据结构、方法和持久化模型定义在 `backend/platform/proto/operator/v1/*.proto`（`operator.v1`），生成到 `backend/platform/pb/operator/`。API 使用独立的 `backend/admin/idl/proto/admin/v1/*.proto`（`admin.v1`）定义 HTTP 路由及自己的请求、响应、分页和业务 DTO，生成到 `backend/admin/biz/model/admin/v1/`。两套业务 protobuf 不共用、不互相 import、不使用类型别名；handler 通过显式逐字段转换调用 operator gRPC。前端只使用 `generated/admin/v1`。
- `backend/platform/pkg/runtimeconfig/`：环境变量启动、Nacos 配置管理和服务发现。
- `backend/platform/pkg/apidocs/`：HTTP OpenAPI 与策略资产。

开发后端前读 `backend/AGENTS.md`；部署与环境变量见 `docs/service_split.md`。

## 铁律

1. 不吞错：catch 必须记录原始错误对象或重新抛出；纯本地 best-effort 例外须注释原因。
2. 搜索用 contains，ID 不做模糊搜索。CRUD 请求体包 `{ data: {...} }`；`body: "*"` 自定义 RPC 收扁平请求。
3. 访问令牌及 HTTP 权限在 admin 校验。operator 接收可信网关上下文，不重复挂 JWT/RBAC 中间件；数据归属和租户隔离仍是数据层约束。
4. operator 仅限内部网络可达，不对浏览器/公网发布 gRPC 端口。
5. 启动参数从环境变量读取，不增加命令行参数。环境变量只提供 Nacos 连接信息；应用配置只从 Nacos 的 `server.yaml`、`data.yaml` 获取，不使用环境变量覆盖、本地 YAML 或内嵌配置回退。Nacos 基础设施配置变更后重启进程。
6. 不手改 protobuf/GORM 生成代码。使用本仓库 Makefile 生成，保持 admin/operator/platform 三个顶级目录。operator 使用 Wire 生成依赖注入；维护 provider 集合及 `cmd/server/wire.go`，运行 `make wire`，不手改 `wire_gen.go`。
7. 修改 operator 消息或 RPC 后在 backend 运行 `make api`（含 `make hz` 和 gatewaygen）；修改 API 消息或 HTTP 路由/绑定用 `make hz`，实际执行官方 `hz update`。OpenAPI 用 `make openapi`，React TS 用 `make ts`。HTTP 路由、请求及返回类型必须在 proto 定义，详见 `backend/admin/README.md`。
8. 已部署数据库新增端点需要更新内嵌 OpenAPI、重启 operator，然后管理页「接口同步」；新增菜单用「菜单同步」。

## 验证

- React：`npm run typecheck`、`npm run build`（在 React 目录）。
- 后端：`go test ./...`、分别构建 `./admin`、`./operator/cmd/server`。
- 不引入或忽略新的门禁错误。

## 专题

已有资源加字段先读 `docs/field_change_guide.md`；新增资源读 `docs/crud_module_guide/`；表单关闭行为读 `docs/form_drawer_close_guard.md`。
认证、多租户、数据范围与服务配置分别参考 `docs/authentication.md`、`docs/tenant_isolation.md`、`docs/data_scope_design.md` 和 `docs/service_split.md`。
