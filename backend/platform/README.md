# 公共依赖

`admin`（Hertz HTTP API）和 `operator`（Kratos gRPC service）共用本目录的依赖和工具，Go module 仍位于 `backend/go.mod`。

- `pkg/`：配置、注册发现、安全、审计、RPC 上下文及其他共享库。
- `proto/operator/v1/*.proto`：全部数据结构、gRPC 方法和 HTTP 映射；`model_*.proto` 定义持久化模型。
- `pb/operator/`：通过 backend Makefile 生成的共享消息、客户端、服务端接口和原生 GORM 模型。
- `tools/`、`third_party/`：代码生成器、注册工具及生成所需的第三方协议。
- `scripts/`、`deploy/`、`sql/`：公共部署、验证和数据库资产。
- `artifacts/`：本地验证产物，已排除出 Git 和镜像。
- `service.mk`：两个服务共用的构建目标。

从 `backend` 运行 `make api openapi ts`、`make models` 和 `make build_only`。共享运行库不导入两个服务的 internal 包，业务规则和数据库访问保留在 operator。
