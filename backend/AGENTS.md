# 后端开发指南

## 服务边界

`admin` 是 Hertz HTTP API，`operator` 是 Kratos gRPC 业务服务。两者直接位于 backend 下，共用 Go module，但有独立入口、二进制和容器。浏览器只调用 admin。

backend 仅有 `admin/`、`operator/`、`platform/` 三个顶级目录。`go.mod`、Makefile、Buf 和 Compose 等全局入口文件保留在 backend 根目录。

- admin：`admin/{main.go,router.go,router_gen.go,biz,internal,idl/proto}`，biz/handler 放 HTTP 处理与 gRPC 代理，biz/model 生成独立 API protobuf 类型，biz/router 放 hz 路由，biz/mw 放鉴权审计；internal/bootstrap 管启动生命周期，internal/transport 管通用 HTTP 编解码。只依赖 gRPC 客户端、共享安全库和 Redis，不导入 operator 的数据包。HTTP 服务契约在 `admin/idl/proto/admin/v1/*.proto`；`.hz` 和 `hz.yaml` 配置官方 Hertz 生成器。
- operator：`operator/internal/{data,service,server}`；GORM、事务、任务、业务服务均在这里。
- platform：`platform/pkg/` 放共享库，`platform/proto/` 和 `platform/pb/` 放协议源码及生成契约，`platform/tools/`、`platform/scripts/`、`platform/deploy/` 放公共工具和部署资产。platform 的运行库不可反向依赖 admin/operator 的 internal 包。
- 协议：operator 的请求/响应、业务 DTO、枚举及 gRPC 方法只在 `platform/proto/operator/v1/*.proto`（`operator.v1`）维护，生成到 `platform/pb/operator/`；admin API 的业务 DTO、分页、请求/响应和 HTTP service 独立维护在 `admin/idl/proto/admin/v1/*.proto`（`admin.v1`），生成到 `admin/biz/model/admin/v1/`。禁止两套业务消息互相 import 或使用类型别名。仅 Google Timestamp/Empty/FieldMask 等标准 protobuf 类型可以共用。handler 用生成的显式 Go 字段转换适配两套契约，禁止 JSON、bytes、反射拷贝或 wire 序列化中转。
- 启动：`platform/pkg/runtimeconfig`，Nacos 双文件配置规则见 `../docs/service_split.md`。
- HTTP 路由由 `hz update` 从 admin IDL 的 `api.get/post/...` 生成到 `admin/biz/router/*_generated.go`。参数绑定、校验、响应和 SSE 使用原生 Hertz Handler 与独立 API protobuf 类型；`platform/tools/httpgen` 生成独立 API HTTP 契约、显式 API/RPC 转换及绑定辅助函数，并将 API HTTP 注解投影给 OpenAPI/TS，不代替 hz 注册路由。禁止手写业务路由、使用 Kratos HTTP router 或 net/http 通配适配器。admin 原生管理启动、Nacos 注册和优雅退出，不创建 Kratos.App。
- `platform/pkg/rpcbridge` 只传递 admin 校验后的操作者及允许的 HTTP 元数据；不要从浏览器转发可信身份头。
- `GatewayService` 是内部数据访问 RPC，返回权限/租户事实；admin 决定允许或拒绝。operator 的租户数据隔离、归属判断和业务配额保留。

## 生成与装配

DI 手写在 operator `cmd/server/wiring.go`，不用 Wire。在 backend 使用 `make api`、`make models`，不使用生成旧 app/service 布局的脚手架命令。

`make api` 通过 `platform/tools/codegen.py` 生成版本化 RPC 与模型，然后调用 `make hz`，执行官方 `hz update` 和 `platform/tools/gatewaygen.py`（admin gRPC 客户端与 operator 实现注册）。仅修改 HTTP 契约可执行 `make hz` 或 `make -C admin hz`；新增业务服务需同时更新 `platform/tools/gateway-services.json`。HTTP 文件、SSE 编码及入口权限在 admin；业务和数据访问在 operator。

HTTP 路由只维护在 admin IDL。路径使用 hz 的 `:id`，`admin.http.body` 指定完整请求 `"*"` 或命名字段，附加 HTTP 入口用 `admin.http.rpc` 指定其转发的原 RPC；字段类型由 API 自己的 message 定义。生成器校验路由和字段映射，保留 API/RPC 的可选值、oneof、列表、map 及枚举语义；API 字段编号可以独立演进。OpenAPI/TS 只投影 admin IDL，不向 operator 回写 HTTP 注解，React 使用 `generated/admin/v1`。`model_*.proto` 使用 GORM 标签定义持久化模型，由 `make models` 生成原生 Go 模型。错误枚举由 `platform/tools/errorgen` 处理。详见 `admin/README.md`；禁止手改生成文件。

## 数据与编码规范

- 持久化模型定义在 `platform/proto/operator/v1/model_*.proto`，通过 `make models` 生成到 `platform/pb/operator/`；不要手写或改动生成模型。
- Repository 使用原生 GORM 与 `operator/internal/data/store/` 的生成类型化操作；过滤条件使用 GORM Scope，FieldMask 保持部分更新语义。
- 新租户模型保留 tenant_id 字段；store 的查询和写入边界必须强制 ViewerContext、租户归属和数据范围，禁止依赖 HTTP 门禁替代数据层隔离。
- 业务调用传递 `context.Context`，操作者通过 `auth.FromContext` 读取。
- 错误使用领域 protobuf 错误码；日志必须保留原始错误。不把业务校验误当访问权限删除。
- 搜索 contains，CRUD `{data: ...}` 与自定义 RPC 扁平请求体规则保持。
- Go struct 多字段逐行；业务状态和固定文案用命名常量；禁止 typed 对象经 JSON/map 中转。

## 启动和验证

从 backend 目录执行 `go run ./operator/cmd/server` / `go run ./admin`。环境变量仅配置 Nacos 地址、命名空间、分组、集群及认证；其他配置必须来自 Nacos 的 `server.yaml`、`data.yaml`。任一文件缺失、为空或读取失败均阻断启动，不使用 `-c`、本地 YAML 或内嵌默认配置。

提交前执行 `go test ./...` 和两个入口的 `go build`。需要外部服务的测试必须说明测试条件；不能把新编译失败当既有问题。
