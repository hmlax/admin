# Contributing

开发前读 [AGENTS.md](AGENTS.md)、[后端规范](backend/AGENTS.md) 和 [React 规范](frontend/admin/react/AGENTS.md)。

本分支只维护 React，后端是 admin HTTP API 和 operator gRPC。访问权限在 admin 检查，operator 保留业务与数据约束。启动参数只从环境变量读取。

协议变更需提交 proto 及生成产物。backend 下 `make api` 生成 Go 与服务适配器，`make openapi` 更新内嵌文档，`make ts` 只更新 React 客户端。GORM 生成优先 `make models`。禁止 Wire。

PR 需说明行为变化和验证证据。门禁是后端 `go test ./...`、两个服务构建，以及 React `npm run typecheck` 和 `npm run build`。新增 CRUD 的详细流程见 `docs/crud_module_guide/`，旧文档中的多前端/单体路径以当前指南为准。
