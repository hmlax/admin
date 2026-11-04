# 平台后台 / 商户后台 / admin / operator 服务拆分

本分支只保留 React。backend 只有 admin、operator、platform 三个顶级目录；两个应用共用根目录的 `go.mod`，分别维护独立的 API/RPC protobuf，分别编译、启动和部署。

```text
平台 React :15888 ─┐
商户 React :15889 ─┴→ admin HTTP :7788 → operator gRPC :7790 → 数据库 / 对象存储 / 任务
                         SSE :7789 ← gRPC 事件流 ← Redis 发布订阅
```

## 边界

`admin` 使用 Hertz 原生路由、参数绑定、中间件和 HTTP/SSE 监听，保留 URL、JSON、CRUD `{data: ...}`、错误格式、刷新 Cookie、multipart 和 Range 下载行为。它负责 JWT 验签、Redis 会话吊销、RBAC、租户入口门禁、字段权限和 HTTP 审计。普通请求以类型化 protobuf RPC 调用 operator，不以 JSON/bytes 包装业务数据。

`operator` 使用 Kratos，仅监听 gRPC。原业务、GORM、种子数据、对象存储和 Asynq 任务迁到这里；不再启动 REST/SSE 服务，也不重复挂 HTTP 的 JWT/RBAC 中间件。它恢复网关传入的操作者、租户、数据范围上下文，保留数据层租户隔离、归属检查与业务约束。gRPC 端口仅限可信内部网络访问；Compose 不映射该端口到宿主机。

网关只转发允许的 HTTP 元数据，可信身份来自 API 验证结果，不接受浏览器传入的身份头。刷新 Cookie 和 MFA 审计用户名通过 gRPC 响应元数据返回。API 审计经内部类型化 RPC 在 operator 落库。

SSE 订阅在 admin 验证令牌和 stream=userId。operator 向 Redis 发布事件，各实例订阅同一用户频道，因此普通业务调用与事件订阅可以落到不同 operator 实例。事件仍为实时 best-effort 推送，持久消息由原业务存储负责。

## 目录与 HTTP 契约

```text
backend/
  admin/                    # Hertz HTTP API 应用
    main.go                 # Hertz 项目入口
    router.go               # 文档等非业务路由扩展
    router_gen.go           # 统一路由接线
    biz/
      handler/              # 类型化 HTTP 处理与 gRPC 代理
      model/                # 独立 API protobuf 模型
      router/               # hz 路由与服务注册
      mw/                   # JWT/RBAC、租户门禁及审计
    internal/
      bootstrap/            # Nacos、监听、依赖与优雅退出
      transport/            # 通用绑定、响应、文件与 SSE 编码
    idl/proto/admin/v1/     # HTTP service、hz 路由与请求体绑定
    .hz                     # hz 项目标记
    hz.yaml                 # 路由生成模板
  operator/                 # Kratos gRPC 应用
    cmd/server/
    internal/               # service、data、server
  platform/                 # 公共依赖和工具
    pkg/                    # 共享安全、审计、配置和 RPC 上下文
    proto/                  # 统一协议源码
      operator/v1/*.proto   # operator 的消息、gRPC 和持久化模型
    pb/operator/     # operator RPC 消息、接口和客户端
    tools/                  # 协议和模型生成、CRUD 注册
    scripts/                # 部署与隔离验证
    deploy/                 # Nacos 发布模板
    third_party/            # 代码生成所需第三方协议
    sql/                    # 数据库资产
    artifacts/              # 本地验证产物（不入库）
    service.mk              # 共用构建目标
  go.mod                    # 共用依赖，两项目独立构建
```

共享运行库不导入 admin/operator 的 internal 包。业务和数据访问留在 operator；admin 的 handler 将独立 API 模型显式转换为 RPC 模型，再通过内部 gRPC 客户端调用。

operator 的数据结构和 RPC 定义平铺在 `platform/proto/operator/v1/*.proto`，统一包名 `operator.v1`。operator 注册并实现这些 gRPC 方法，admin 通过同一包下的客户端调用业务服务，再以 Hertz 对接前端。HTTP service 独立定义在 `admin/idl/proto/admin/v1/*.proto`：`api.get/post/...` 声明路由，`admin.http.body` 声明请求体绑定，请求/响应和业务 DTO 全部使用 admin 自己的消息，生成到 admin/biz/model/admin/v1。两套业务 proto 不互相 import，不使用类型别名；operator proto 不保存 HTTP 路由。

官方 `hz update` 根据 `admin/hz.yaml` 生成路由注册；`platform/tools/httpgen` 校验契约并生成 typed 参数/响应绑定和 OpenAPI/TS 的临时描述符投影；`platform/tools/httpgen` 生成独立模型间的显式字段转换及类型化客户端代理；`platform/tools/gatewaygen.py` 生成依赖装配和 operator 注册。文件、SSE 只在 transport 层编码。内部 gRPC 全名为 `operator.v1.*`；HTTP `/admin/v1` URL、JSON 字段和 CRUD 包装保持兼容，React 使用独立的 `generated/admin/v1` 客户端。已有数据库更新内嵌 OpenAPI 后重启 operator，再执行管理页「接口同步」。

## 配置来源

环境变量仅提供 Nacos 连接信息。admin 和 operator 从同一 namespace/group 固定读取 `server.yaml`、`data.yaml`；应用配置不接受环境变量覆盖，不读取本地配置目录或内嵌默认配置。两份文件任意一份缺失、为空、无权限、解析失败或关键配置不完整，进程均会停止启动。

| 环境变量 | 含义 |
|---|---|
| `NACOS_ADDR` | 必填，逗号分隔 Nacos 地址，例如 `http://infra-nacos:8848` |
| `NACOS_NAMESPACE_ID` | 命名空间 ID，public 使用空串 |
| `NACOS_GROUP` / `NACOS_CLUSTER` | 配置/注册分组和集群，Compose 默认 `GO_ADMIN` / `DEFAULT` |
| `NACOS_USERNAME` / `NACOS_PASSWORD` | Nacos 认证账号 |
| `NACOS_CONTEXT_PATH` | 可选，默认 `/nacos` |
| `NACOS_LOG_DIR` / `NACOS_CACHE_DIR` | 可选 SDK 工作目录，默认系统临时目录 |

`server.yaml` 保存 HTTP/gRPC/SSE 监听、超时、CORS、任务队列、日志及运行选项；`data.yaml` 保存数据库、Redis、对象存储、JWT 和加密密钥。两份文件组合成 Bootstrap 配置；`runtime` 段补充应用专用选项：

- `runtime.crypto_key`：16/24/32 字节应用加密密钥，两个进程使用同一值，并与 React 的 VITE_AES_KEY 保持一致。
- `runtime.operator_endpoint`：可选直接 gRPC 地址；留空通过 Nacos 发现 `operator.grpc`。
- `runtime.http_advertise_addr` / `sse_advertise_addr` / `grpc_advertise_addr`：可选显式注册地址；默认自动使用可达的本机地址。
- `runtime.instance_id`：可选实例 ID；通常留空由框架生成。
- `runtime.sso`：OIDC issuer、client_id、client_secret、redirect_url、scopes、auto_create、tenant_code。
- `runtime.audit_archive_dir` / `audit_retention_days` / `backup_retention_days`：归档与备份保留策略；备份保留天数为 0 时关闭清理。
- `runtime.notification_delivery_stale_minutes` / `allow_private_webhook` / `chrome_path` / `print_no_sandbox`：通知及打印运行选项。

字段与完整结构见 `backend/platform/deploy/infra/nacos/server.yaml.example`、`data.yaml.example`。这些是 Nacos 发布模板，运行中的应用只访问 Nacos。Nacos 配置按启动快照读取，变更后重启对应进程。Nacos HTTP 端口与 SDK gRPC 端口（通常 HTTP+1000）均须可达。

## 启动

在 Nacos 发布上述两份配置，然后在 backend 设置 Nacos 连接并分别运行：

```sh
export NACOS_ADDR=http://127.0.0.1:8848
export NACOS_GROUP=GO_ADMIN
export NACOS_USERNAME=your-nacos-user
export NACOS_PASSWORD=your-nacos-password
go run ./operator/cmd/server
# 另一个终端使用相同 Nacos 连接
go run ./admin
```

HTTP 默认配置为 7788、SSE 为 7789、内部 gRPC 为 7790。逻辑服务名为 admin/operator，Nacos 注册为 `admin.http`、`operator.grpc`。admin 不启动任务 worker；operator 不启动 HTTP。平台 React 在 `frontend` 执行 `npm run dev:platform`，商户 React 先配置 `merchant-admin/.env.development.local` 的 `VITE_MERCHANT_CODE` 再执行 `npm run dev:merchant`；两端代理均访问 7788/7789。两端分别构建到 `platform-admin/dist`、`merchant-admin/dist`，启动和 Docker 部署见 [前端说明](../frontend/README.md)。

## 生成与迁移

backend 执行 `make api`、`make openapi`、`make ts` 生成 Go 协议、HTTP/SSE 绑定、代理、OpenAPI 和 React 客户端。`make api` 包含 `make hz`，后者实际调用官方 `hz update`；仅修改 HTTP 路由/绑定时执行 `make hz` 或 `make -C admin hz`。IDL 示例、单文件 hz 命令和依赖导出见 [admin 开发说明](../backend/admin/README.md)。GORM 使用 `make models`；新增资源登记用 `make register ENTITY=...`。

已有数据库新增端点仍需更新内嵌 OpenAPI、重启 operator、管理页「接口同步」；菜单用「菜单同步」。验证使用 `go test ./...` 和两入口构建；React 使用 `npm run typecheck`、`npm run build`。

## 使用现有 infra-docker 部署

`backend/docker-compose.infra.yaml` 只启动 admin/operator，复用外部 `infra-network`，不启动或替换现有基础设施。HTTP/SSE 只映射本机 7788/7789；operator 不发布宿主机端口。本机专用数据库为 `gwa_admin`，账号为 `gwa_operator`，Redis 12 号库保存数据，13 号库保存任务。

`backend/.env.infra` 只保存 Nacos 连接信息，已加入 Git 忽略。原应用连接及密钥已移入同样忽略的 `backend/platform/deploy/infra/nacos/server.yaml`、`data.yaml`，这两份文件仅用于首次发布，且从 Docker 构建上下文中排除。Nacos 账号需要本项目分组的配置和服务权限。

```sh
cd backend
make infra-build
make infra-config  # 显式发布本地 server.yaml / data.yaml
make infra-up      # 从 Nacos 读取配置，不自动覆盖线上配置
make infra-verify
make infra-test-tenant # 套餐/租户创建、登录、权限、配额和隔离完整验收，保留新的测试账号
```

配置发布端默认连接 `http://127.0.0.1:8848`，可用 `NACOS_PUBLISH_URL` 覆盖；容器使用 `.env.infra` 的 `NACOS_ADDR`。验证脚本需 PyYAML 6.0.2（`python3 -m pip install PyYAML==6.0.2`），从 Nacos 读取 Redis/加密配置，验证注册发现、登录、刷新、SSE 和注销。若默认 Python 未安装该依赖，可用 `make PYTHON=/path/to/python3 infra-verify` 选择已安装的解释器。

完整的套餐与租户自动化使用相同连接方式，执行 `make infra-test-tenant`；详细覆盖范围与生成凭据位置见 [租户生命周期自动化](tenant_lifecycle_e2e.md)。

查看状态/日志使用 `docker compose --env-file .env.infra -f docker-compose.infra.yaml ps` / `logs --tail=100 admin operator`；停止使用 `make infra-down`，保留数据卷及现有 infra 容器。

## 可重复的隔离验证

- Nacos：`docker compose -f platform/scripts/test/nacos-compose.yaml -p gwa-split-test up -d`，设置 `NACOS_TEST_ADDR=127.0.0.1:18848` 后运行 `go test -tags=integration ./platform/pkg/runtimeconfig -run TestNacosConfigurationAndDiscovery`；完成后对同一 fixture 执行 `down`。测试独立分组、配置和注册项均会清理。
- 服务链路：先分别构建到 `/tmp/gwa-split-operator`、`/tmp/gwa-split-admin`，启动 `platform/scripts/test/service-compose.yaml`（项目名 `gwa-service-smoke`，`up -d --wait`），运行 `python3 platform/scripts/test/service-smoke.py`，最后对同一 fixture 执行 `down -v`。脚本只为独立测试库建表、补齐标准角色权限，应用启动不执行 DDL。验证覆盖 Nacos 双文件配置、服务发现、登录、刷新 Cookie、独立 SSE 端口的实时刷新及断开取消、CRUD 和注销吊销。
- 并发检查：`go test -race ./admin/... ./platform/pkg/rpcbridge ./platform/pkg/runtimeconfig ./platform/pkg/security`。

Hertz 原生监听和优雅退出参见 [官方 Engine 文档](https://www.cloudwego.io/docs/hertz/tutorials/basic-feature/engine/)，SSE 流写入参见 [官方响应 Writer 扩展](https://www.cloudwego.io/docs/hertz/tutorials/framework-exten/response_writer/)，Nacos SDK 参见 [官方 Go SDK 文档](https://nacos.io/docs/latest/manual/user/go-sdk/usage/)。

## Go Admin 命名迁移

项目显示名称为 `Go Admin`，Go module、Compose 项目名及 Redis 前缀统一为 `go-admin`，新的 Nacos 示例分组为 `GO_ADMIN`。第三方依赖保留其已发布的包名与模块地址。

已有部署可继续显式使用原 Nacos 分组；切换到 `GO_ADMIN` 前，先将现有 `server.yaml`、`data.yaml` 配置复制到该分组并保持密钥不变，再同步更新 admin/operator 的 `NACOS_GROUP` 并重启。不要在空的新分组直接启动。

Redis 前缀切换需同时更新全部 admin/operator 实例，切换前启用维护窗口并让旧登录限流计数及一次性验证码过期；新版本使用新的验证码、缓存与发布订阅命名空间。现有数据库业务记录不因代码改名自动重写。已有 TOTP 密钥保持有效，新绑定的认证器发行方显示为 `Go Admin`。

React 生产环境默认使用同源 `/` API 与 `/events` SSE 地址，部署时将这两类请求分别代理到 admin 的 HTTP/SSE 端口。

后端 HTTP 项目现为 `backend/admin`，本地构建产物为 `admin/bin/admin`，Compose 服务和镜像为 `admin` / `go-admin/admin`，Nacos 注册名为 `admin.http`。升级部署时将原 HTTP 服务替换为 admin，并同步反向代理的上游服务名；配置文档、端口以及 `/admin/v1` HTTP 契约沿用现有设置。`make api` 仍是协议生成目标。

## admin 的 Hertz 原生运行链路

`admin/main.go` 从 Nacos 创建共享配置上下文后，启动 Hertz HTTP/SSE 服务；进程负责接收退出信号、注册/注销 Nacos、取消长连接请求、等待全部监听退出及释放 gRPC/Redis。operator 继续使用 Kratos gRPC。

路由和 request/response 类型以 proto 为唯一来源。`make hz` 使用 `hz update` 生成 `admin/biz/router/*_generated.go` 路由，使用共享工具生成 `bindings_generated.go` 和 `admin/biz/model/admin/v1/*_http.pb.go` HTTP 契约与 handler 中显式 API/RPC 转换；不手改这些文件。路径按 hz `:id` 语法维护，授权和审计元数据仍使用既有 `{id}` 模板。CRUD JSON、path/query 的类型与优先级、refresh Cookie、多值响应头、multipart、二进制 Range 和 SSE 帧保持原有协议。

Hertz 原生中间件校验 JWT/Redis 吊销、租户状态与配额、平台入口和 RBAC，并保留审计及字段裁剪。HTTP 请求事实通过 `platform/pkg/httpmeta` 交给审计和内部 RPC；只有 allowlist HTTP 元数据和校验后的操作者可以传给 operator。

## 两套 React 前端

`frontend/platform-admin` 是平台入口，`frontend/merchant-admin` 是商户入口，各有独立的 entry、HTML、Vite/env、路由和页面清单。共享组件和类型化 API 客户端位于 `frontend/src`，`@app/` 别名按应用选择清单，商户构建排除平台专属页面。

商户入口由部署绑定固定商户编号，生产通过同源 `/app-config.json` 初始化；nginx 同时为 HTTP 与 SSE 注入固定 `X-Tenant-Code`，平台入口清除该头。缺失或非法编号会拒绝启动。商户登录不接收可编辑的租户编号，后端入口门禁验证账号与商户相符。浏览器存储和刷新 Cookie 按应用/商户隔离，商户菜单继续取服务端角色与套餐的交集。
