# 04 · 第一个业务模块：端到端 CRUD 实战

> 持久化实现已于 2026-11-04 统一到 Kratos + GORM。本文保留原设计与历史实现记录；当前代码路径、模型生成和租户约束以 [后端 CRUD 指南](../crud_module_guide/backend.md) 与 [租户隔离](../tenant_isolation.md) 为准。

前置：[代码生成链路](./03-codegen-chain.md)。本章是新增资源的路线图，具体代码约定见 [后端手册](../crud_module_guide/backend.md) 和 [React 手册](../crud_module_guide/react.md)。已有资源加字段看 [字段修改指南](../field_change_guide.md)。

## 1. 确认资源形态

确定实体名、业务字段、所属业务模块，以及普通 CRUD、树、主从或租户资源的形态。复用字典类型、菜单、角色等现有模块的实现，并遵循对应 AGENTS.md。

## 2. 分别定义 RPC 与 HTTP 契约

- operator：`backend/platform/proto/operator/v1/<entity>.proto`，定义 `operator.v1` 数据结构及 gRPC 方法。
- admin：`backend/admin/idl/proto/admin/v1/`，定义 `admin.v1` HTTP 请求、响应和 Hertz 路由。API 和 RPC 各自维护业务消息，handler 使用显式逐字段转换。
- 普通资源路径使用 `/admin/v1/<entities>`；CRUD 请求体包 `{ data: {...} }`，Update 携带 `update_mask`。

在 backend 执行：

```bash
make api openapi ts
```

这会更新 operator RPC、admin API 模型、Hertz 路由、显式转换、OpenAPI 以及 React `src/api/generated/admin/v1/`。只修改 HTTP 契约时使用 `make hz openapi ts`。

## 3. 实现 operator 业务

在 `platform/proto/operator/v1/model_<resource>.proto` 定义 GORM 标签与关系，再执行 `make models`。租户模型保留 tenant_id，并验证 store 的查询和写入隔离。

按 `dict_type_repo.go`、`dict_type_service.go` 实现 Repository 和 Service。审计身份从可信网关上下文读取；数据归属和租户隔离在 operator 保留。带关联 ID 列表的更新遵循 [后端手册](../crud_module_guide/backend.md) 的同事务替换规则。

注册新模块：

```bash
make register ENTITY=<entity>
```

检查 `operator/cmd/server/wiring.go` 和 `platform/tools/gateway-services.json`，补齐额外构造依赖。该命令更新生成的 gRPC 注册与 admin 代理装配。HTTP 路由由 IDL 经 `hz update` 生成。

## 4. 实现 React 页面

在 `frontend/src/` 添加 Service getter、Hooks、ProTable 列表、DrawerForm 表单、业务路由和中英文词条。

- 使用 `new PaginationQuery({ paging, formValues })`。
- Update 使用带 `updateMask` 的 Hooks；保存后使对应列表 Query 失效。
- 菜单标题使用 `meta.title: 'routes:xxx'`。
- 表单关闭行为遵循 [抽屉关闭守卫](../form_drawer_close_guard.md)。

具体模板见 [React 手册](../crud_module_guide/react.md)。

## 5. 验证与部署

运行 [CRUD 验证清单](../crud_module_guide/README.md#验证与部署) 中的 Go 测试、两个入口构建，以及 React typecheck/build。启动后检查 OpenAPI，并完成列表、搜索、分页、新建、编辑、删除和权限隔离测试。

已有部署需要重新生成内嵌 OpenAPI、重启 operator，然后在管理页执行「接口同步」；新增菜单用「菜单同步」。部署配置见 [服务拆分指南](../service_split.md)。
