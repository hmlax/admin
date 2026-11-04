# 02 · 从零跑起来：本地开发环境

> 持久化实现已于 2026-11-04 统一到 Kratos + GORM。本文保留原设计与历史实现记录；当前代码路径、模型生成和租户约束以 [后端 CRUD 指南](../crud_module_guide/backend.md) 与 [租户隔离](../tenant_isolation.md) 为准。

前置：[架构全景](./01-architecture-overview.md)。当前启动配置以 [服务拆分指南](../service_split.md) 为准。

## 1. 准备工具和依赖

Go 版本见 `backend/go.mod`；前端在 `frontend/admin/react/` 使用 Node.js 和 pnpm 或 npm。Docker 用于数据库、Redis、对象存储及 Nacos 等基础设施。Go 代码生成工具通过 backend 的 `make init` 安装。

已部署 infra-docker 的本机可复用现有基础设施，见 [现有 infra 部署](../service_split.md#使用现有-infra-docker-部署)。独立搭建时，`backend/docker-compose.libs.yaml` 提供数据库、Redis 和对象存储；Nacos 需按服务拆分指南另行准备。不要在相同端口重复启动已有基础设施。

## 2. 发布 Nacos 配置

应用配置只从 Nacos 的 `server.yaml`、`data.yaml` 读取。模板位于 `backend/platform/deploy/infra/nacos/*.yaml.example`，应用不会直接加载这些模板。

本地运行进程时，将 Nacos 中数据库、Redis、对象存储及服务发现地址配置为本机实际可达的地址。不要使用已删除的本地 `configs/` 目录或 `-c` 参数。

每个后端终端设置相同的 Nacos 连接信息：

```bash
export NACOS_ADDR=http://127.0.0.1:8848
export NACOS_GROUP=GO_ADMIN
export NACOS_USERNAME=your-nacos-user
export NACOS_PASSWORD=your-nacos-password
```

namespace、cluster 和认证细节见 [配置来源](../service_split.md#配置来源)。配置发布后重启对应服务。

## 3. 生成代码并启动两个后端

在 backend 执行：

```bash
go mod download
make api openapi ts
```

修改 Ent schema 后执行 `make models`。然后在两个已设置 Nacos 连接的终端分别启动：

```bash
# 终端 1（backend）
go run ./operator/cmd/server
```

```bash
# 终端 2（backend）
go run ./admin
```

HTTP 文档地址为 `http://localhost:7788/docs`。operator 实现业务及数据库访问，admin 提供 HTTP/SSE；两者均须启动。

## 4. 启动 React

在仓库根目录执行：

```bash
cd frontend/admin/react
pnpm install
pnpm dev
```

也可使用 `npm install`、`npm run dev`。默认访问 `http://localhost:15888`，实际端口以 Vite 日志为准。HTTP `/admin` 请求由 Vite 代理到 7788，SSE 使用配置中的 7789 地址。

## 5. 端口与排障

| 服务 | 默认端口 | 用途 |
|---|---|---|
| React | 15888 | 本地开发页面 |
| admin HTTP | 7788 | HTTP API 与文档 |
| admin SSE | 7789 | 实时推送 |
| operator gRPC | 7790 | 可信内部调用 |

- 启动报 Nacos 配置缺失：检查 namespace/group、认证及两份配置内容。
- admin 调用 operator 失败：检查 operator 启动状态、Nacos 注册和实际可达的 gRPC 地址。
- `hz`、`buf` 或 protoc 插件找不到：检查安装结果以及 Go bin 是否在 PATH 中。
- 修改协议后前端缺少新字段：在 backend 执行 `make api openapi ts`，再运行 React typecheck。

下一步：[03 · 代码生成链路](./03-codegen-chain.md)。
