# Operator v1 契约

所有协议平铺在 `operator/v1/*.proto`，统一使用 `package operator.v1` 和 Go 包 `go-admin/platform/pb/operator`。

```text
platform/proto/operator/v1/
├── user.proto          # 用户结构与 UserService RPC
├── role.proto          # 角色结构与 RoleService RPC
├── gateway.proto       # 内部权限事实、审计和辅助 RPC
├── events.proto        # SSE 对应的流式 RPC
├── model_user.proto    # protobuf 定义的 GORM 模型
├── persistence.proto   # 持久化生成选项
└── ...
```

operator 实现并注册生成的 gRPC 接口，负责业务、仓储和事务。admin 创建同一包的 gRPC 客户端，以 Hertz 提供 HTTP/SSE 给 React；入口 JWT、会话吊销、RBAC、字段权限、HTTP 编码和审计归 admin。HTTP 注解放在共享 RPC 上，operator 不提供 HTTP 监听。

在 backend 运行 `make api openapi ts`。生成器自动发现新协议文件；`model_*.proto` 由专用生成器生成原生 GORM 结构及仓储构建器。Go 输出在 `platform/pb/operator/`，React 客户端输出在 `src/api/generated/operator/v1/`。

顶层枚举以消息作用域隔离同名枚举值，保留原数值和 JSON 字符串。错误定义使用 `AdminErrors.Reason`、`IdentityErrors.Reason` 等作用域；生成的 `ErrorAdminBadRequest` 等函数保留原 HTTP code/reason。

内部 gRPC 全名已统一到 `operator.v1.*`，admin/operator 需要配套构建和重启；HTTP URL、CRUD 包装和前端 JSON 字段保持原契约。已有数据库重启 operator 后执行管理页「接口同步」。
