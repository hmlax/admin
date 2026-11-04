# 01 · 架构全景：一次请求的完整路径

> 持久化实现已于 2026-11-04 统一到 Kratos + GORM。本文保留原设计与历史实现记录；当前代码路径、模型生成和租户约束以 [后端 CRUD 指南](../crud_module_guide/backend.md) 与 [租户隔离](../tenant_isolation.md) 为准。

> 前置：无。读完你会知道：代码都在哪、一个请求从浏览器到数据库经过了什么、有哪些横切系统在起作用。

## 1. Monorepo 布局

```text
backend/
├── admin/       Hertz HTTP/SSE 入口、独立 HTTP IDL、biz/handler/model/router/mw
├── operator/    Kratos gRPC 业务、Ent、任务；入口 cmd/server
└── platform/    公共 pkg、operator proto/pb、代码生成及部署工具
frontend/admin/react/  React 19 + Ant Design + ProComponents + TanStack Query
```

admin 与 operator 共用 `backend/go.mod`，API 和 RPC 各自维护业务 protobuf。完整布局见 [服务拆分指南](../service_split.md)。

## 2. 一次请求的完整路径

以用户管理页查询为例：

```text
React :15888
  → Vite /admin 代理
  → Hertz admin HTTP :7788：hz 路由、参数绑定、JWT/会话、RBAC、租户入口门禁
  → handler：API 请求逐字段转为 RPC 请求
  → operator gRPC :7790：恢复可信网关身份
  → Service → Repository → Ent 租户隔离与数据范围 → 数据库
  → RPC 响应转为 API 响应、字段权限裁剪、返回 React
```

React 开发配置为 `VITE_API_URL=/`，Vite 按 `/admin` 前缀代理到 admin，路径不做 rewrite。operator 的 gRPC 仅限可信内部网络，不发布给浏览器。

- admin 负责 HTTP 访问令牌、会话吊销、接口权限、租户入口、字段权限和入口审计。
- operator 实现业务逻辑、数据归属与行级隔离，接收可信身份上下文。
- RPC DTO、API DTO 与 Ent entity 分别维护；API/RPC 显式转换由生成器生成，持久化映射需注册时间和枚举转换器。

另有两条通道：

| 通道 | 位置 | 用途 |
|---|---|---|
| SSE `:7789/events` | admin | 独立鉴权的实时推送入口 |
| Asynq worker / Redis | operator | 异步任务、定时任务、广播扇出 |

## 3. 横切系统地图

| 系统 | 一句话 | 实现位置 | 深读 |
|---|---|---|---|
| 认证 | JWT（access 内存 + refresh HttpOnly Cookie），登录口令应用层 AES 加密传输 | `platform/pkg/middleware/auth`、`internal/service` 认证服务 | 第 5 章 |
| 授权（接口级） | authz 引擎判定"该用户能否调该接口"，策略存 DB 可热更新 | `platform/pkg/middleware/auth` 引擎接入 | 第 5 章 |
| 租户隔离 | 行级 `tenant_id` 谓词，读写在数据层强制；HTTP 层另有 Api 表 `(path,method)` 闸门 | ent 隐私层 + `platform/pkg/middleware` | 第 6 章 |
| 数据范围 | 角色级行过滤（五档），令牌承载聚合结果 | ent 隐私层（逐表 opt-in） | [data_scope_design.md](../data_scope_design.md) |
| 字段级权限 | 角色黑名单字段，服务端响应裁剪 + 前端隐藏 | admin `biz/handler` + React access store | [frontend_authority.md](../frontend_authority.md) |
| 审计日志 | 六类日志全覆盖，含 SQL 级 data access 采集与词法脱敏 | `platform/pkg/middleware/logging`、driver 包装器 | [audit-log-producer-design.md](../audit-log-producer-design.md) |
| 异步任务 | asynq 调度（cron/周期），Redis 队列 | `internal/service` 任务桥 | [task_system.md](../task_system.md) |
| 脚本系统 | Lua/JS 脚本级插件：实体钩子/定时任务/事件/HTTP 出站 | `platform/pkg/scripting` + 管理页 | [script_system.md](../script_system.md) |
| SSE 推送 | 服务端推送（站内信通知）；流 ID=userId、连接独立鉴权、事件 best-effort | `internal/service`（生产）+ React `transport/sse` 模块 | [sse_architecture.md](../sse_architecture.md) |
| 对象存储 | SeaweedFS（S3 兼容），预签名上传下载，元数据落库 | `internal/data` 文件模块 | [backend_file_upload.md](../backend_file_upload.md) |
| AI 模块 | 模型提供商（表驱动）/ 流式对话 / 知识库 RAG（pgvector）/ 用量配额 / 脚本与任务集成 | `platform/pkg/ai` + `internal/service` ai 服务族 + React `app/ai` | [ai_module.md](../ai_module.md)、第 10 章 |

## 4. 生成代码与手写代码的边界

这个仓库大量依赖代码生成。**分不清这条边界，改错文件是新人最常见的翻车点：**

| 目录 | 产生方式 | 能否手改 |
|---|---|---|
| `backend/platform/pb/**` | `make api` 从 operator proto 生成 | ❌ 改 proto 重新生成 |
| `backend/operator/internal/data/ent/**`（除 `schema/`） | `make models` 从 schema 生成 | ❌ 改 schema 重新生成 |
| `backend/admin/biz/model/**`、`biz/router/*_generated.go` | `make hz` 从 admin IDL 生成 | ❌ |
| React `src/api/generated/**` | `make ts` 从 admin IDL 生成 | ❌ |
| `backend/operator/internal/data/ent/schema/**` | 手写 | ✅ |
| `backend/platform/proto/operator/**` | 手写 proto | ✅ |
| `internal/service/`、`internal/data/*_repo.go` | 手写（模式化样板） | ✅ |
| `cmd/server/wire.go` / `wire_gen.go` | Wire 注入器声明 / 生成的 DI 装配（各层 `providers.go`） | ✅ |

生成链路的完整机制在[第 3 章](./03-codegen-chain.md)，实战在第 4 章。

下一步：[02 · 从零跑起来](./02-get-it-running.md)。
