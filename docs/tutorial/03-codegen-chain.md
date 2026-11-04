# 代码生成链路

本项目的 HTTP API、operator RPC 和持久化模型分别定义，生成文件不手工编辑。

```text
platform/proto/operator/v1/*.proto
  └─ operator RPC 消息、客户端和服务端接口 → platform/pb/operator/
platform/proto/operator/v1/model_*.proto
  └─ proto GORM 标签与关联元数据 → 原生持久化模型 + store 类型化操作
admin/idl/proto/admin/v1/*.proto
  ├─ 独立 API 消息 → admin/biz/model/admin/v1/
  ├─ 官方 hz update → Hertz 路由
  ├─ 显式 API/RPC 转换及传输适配
  ├─ HTTP OpenAPI → 内嵌文档与策略资产
  └─ TypeScript → frontend/src/api/generated/admin/v1/
```

在 `backend/` 执行：

```bash
make api       # RPC + models + hz + gatewaygen
make openapi
make ts
```

仅修改持久化模型或模型生成器时使用 `make models`；仅修改 HTTP 契约使用 `make hz`。生成器根据 proto 产出原生 GORM Scope 和字段 setter，不读取旧 ORM schema。生成模型直接作为 GORM 目标；运行时构造器不会执行 DDL。

手写业务放在 operator 的 data/service；装配在 `operator/cmd/server/wiring.go`。新增 HTTP 路由不能绕开 proto；API/RPC 业务 protobuf 不互相 import、不使用类型别名。

生成后检查重复生成稳定性、运行 Go 测试并构建两个入口。新增接口部署后还需执行平台「接口同步」，新增菜单执行「菜单同步」。
