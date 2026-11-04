# 后端项目结构说明

> 持久化实现已于 2026-11-04 统一到 Kratos + GORM。本文保留原设计与历史实现记录；当前代码路径、模型生成和租户约束以 [后端 CRUD 指南](crud_module_guide/backend.md) 与 [租户隔离](tenant_isolation.md) 为准。

```text
backend/
├── admin/
│   ├── cmd/server/              # Hertz 原生 HTTP / SSE 入口
│   ├── internal/server/         # 权限校验、gRPC 代理、协议编解码
│   └── Makefile
├── operator/
│   ├── cmd/server/              # Kratos gRPC 入口与手写 wiring
│   ├── internal/
│   │   ├── data/                # Ent、仓储、事务
│   │   ├── server/              # gRPC 适配器与内部网关
│   │   └── service/             # 业务、任务、对象存储
│   └── Makefile
├── platform/                    # 两个服务的公共依赖
│   ├── proto/                   # 统一协议源码
│   │   └── operator/v1/*.proto # 全部结构、RPC、HTTP 映射及 model_*.proto
│   ├── pb/operator/             # 共享 DTO、客户端、服务端、GORM 模型
│   ├── pkg/                     # 共享安全、配置、协议上下文等
│   │   ├── runtimeconfig/       # 环境变量、Nacos 配置和注册发现
│   │   ├── rpcbridge/           # 可信网关身份和 HTTP 元数据
│   │   ├── apiroutes/           # proto 路由元数据
│   │   └── apidocs/             # 内嵌 OpenAPI、RBAC 策略
│   ├── tools/                   # HTTP/模型生成、代理装配及 CRUD 注册
│   ├── scripts/                 # 部署、隔离验证
│   ├── deploy/                  # Nacos 发布模板
│   ├── third_party/             # 第三方协议
│   ├── sql/                     # 数据库资产
│   ├── artifacts/               # 本地验证产物（不入库）
│   └── service.mk               # 两个应用共用构建目标
├── buf.yaml                     # operator.v1 与第三方注解模块
├── buf.gen.yaml                 # Go 生成模板
├── buf.admin.openapi.gen.yaml   # HTTP 文档模板
├── buf.react.admin.typescript.gen.yaml
├── Makefile
├── go.mod
├── go.sum
├── docker-compose.yaml
└── Dockerfile
```

backend 只有 admin、operator、platform 三个顶级目录。两个应用分别构建和部署，共用根目录的 Go module 和 platform 的公共契约与运行库。admin 验证 HTTP 权限后通过类型化 gRPC 调用 operator；operator 只开放内部 gRPC，保留租户数据隔离与业务约束。platform 的运行库不反向导入服务的 internal 包。

开发规范见 [backend/AGENTS.md](../backend/AGENTS.md)，环境变量、Nacos、生成与部署见 [服务拆分指南](service_split.md)。
