# Go 后端

backend 只有 `admin/`、`operator/`、`platform/` 三个顶级目录。两个应用共用根目录的 Go module，分别编译、启动和部署：

- `admin/`：Hertz HTTP（7788）和 SSE（7789），负责 JWT、Redis 吊销、RBAC、租户入口、字段权限、审计。
- `operator/`：Kratos gRPC（7790），负责业务、GORM、任务、对象存储；仅内部网络可达。
- `platform/`：公共依赖、共享协议、生成代码、配置安全库以及代码生成和部署工具。
- `platform/proto/operator/v1/*.proto`：operator 内部数据结构和 gRPC 方法；`admin/idl/proto/admin/v1/*.proto`：独立 API 数据结构、请求/响应与 HTTP 路由。
- `platform/pb/operator/`：RPC 消息、gRPC 客户端/服务端；`admin/biz/model/admin/v1/`：独立 API protobuf；`platform/pkg/apidocs/`：内嵌 OpenAPI 与策略资产。
- `platform/pkg/runtimeconfig/`：环境变量启动、Nacos 配置管理和服务发现。

## 启动

Go 版本以 `go.mod` 为准。先启动 PostgreSQL、Redis 和 S3 兼容存储，在 Nacos 同一分组发布 `server.yaml`、`data.yaml`，仅将 Nacos 连接信息放在环境变量中。

```sh
go run ./operator/cmd/server
# 另一个终端使用同一 Nacos 连接配置
go run ./admin
```

环境变量只配置 Nacos 连接。监听、数据库、Redis、JWT、加密密钥、对象存储、SSO 等应用配置全部读取 Nacos `server.yaml`、`data.yaml`，不读取应用环境变量覆盖或本地回退。配置变更后重启进程。完整环境变量、Nacos、部署和隔离验证见 [服务拆分指南](../docs/service_split.md)。

## 生成与构建

以下命令在 backend 目录执行。`make plugin` 安装 protobuf 插件；需要 buf CLI。

```sh
make api              # protobuf、Hertz HTTP/SSE 绑定及类型化 gRPC 代理
make openapi          # HTTP OpenAPI，含文件和事件流的响应定义
make ts               # React TypeScript 客户端
make models  # GORM 模型；DI 为手写 wiring，不使用 Wire
make register ENTITY=product
make build_only       # admin/bin/admin 与 operator/bin/operator
```

API 接口以 `admin/idl/proto/admin/v1` 为准，RPC 以 `platform/proto/operator/v1` 为准，handler 显式转换两套独立 DTO。admin 通过生成的 gRPC 客户端调用 operator，Hertz 处理 HTTP/SSE 路由、权限、参数绑定及编码；operator 仅注册并监听 gRPC。修改后执行 `make api openapi ts`；不要手改生成代码。内部 RPC 命名空间为 `operator.v1`，两个服务配套更新；已有数据库新增端点需重启 operator 后执行「接口同步」。

## 部署与验证

复制 `.env.example` 为 `.env` 并设置 Nacos 连接，并在 Nacos 配置实际连接和密钥，然后执行 `docker compose up --build -d`。operator 的 gRPC 不映射到宿主机。PM2 部署使用 `make pm2-deploy`，两个应用均从 Nacos 读取应用配置。

```sh
go test ./...
go build -o admin/bin/admin ./admin
go build -o operator/bin/operator ./operator/cmd/server
```

详细开发规则见 [AGENTS.md](AGENTS.md)。React 独立位于 `../frontend/admin/react`，执行 `npm run typecheck`、`npm run build` 验证。
