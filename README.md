# Go Admin

通用管理后台框架，保留平台后台、独立部署的商户后台和多租户隔离。

```text
platform-admin / merchant-admin
  → admin（Hertz HTTP :7788）
  → operator（Kratos gRPC :7790）
  → 数据库 / Redis / 对象存储
```

保留的基础能力：

- 账号登录、验证码、刷新会话、MFA、SSO、密码策略与在线会话。
- 管理员、角色、权限点、菜单、API 资源、按钮和字段权限。
- 组织、岗位、数据范围及租户数据隔离。
- 平台租户管理、商户入口配置、租户状态和有效期校验。
- 审计日志、字典、语言、文件上传、访问密钥和系统配置。

前后端已移除 AI、BI、消息通知、任务调度、脚本、打印模板、业务监控、套餐计费与配额模块。首页是通用欢迎页。数据库不会自动删除旧表或旧记录；已有菜单中无法在当前应用加载的页面会被过滤。

## 目录与接口

- `backend/admin`：标准 Hertz 布局；独立 HTTP proto IDL，使用 `hz update` 生成路由。
- `backend/operator`：Kratos gRPC 服务、业务实现、GORM 仓储。
- `backend/platform`：公共依赖；RPC 定义在 `proto/operator/v1/*.proto`，生成到 `pb/operator/`。
- `frontend/platform-admin` / `frontend/merchant-admin`：独立入口、配置、构建和部署，共享 React 组件与客户端。

HTTP 与 gRPC 使用各自的 proto 消息，按字段转换。平台请求不携带商户编号；商户部署固定一个编号，通过 `X-Tenant-Code` 请求后端，登录表单无需填写。

## 本地启动

安装 Go（版本见 `backend/go.mod`）、Node.js 和 pnpm。准备数据库结构、Redis、对象存储，并在 Nacos 发布 `server.yaml`、`data.yaml`。配置示例在 `backend/platform/deploy/infra/nacos/`。服务启动不执行数据库 DDL。

```sh
cd backend
export NACOS_ADDR='http://127.0.0.1:8848'
export NACOS_GROUP='GO_ADMIN'
export NACOS_NAMESPACE_ID=''
export NACOS_USERNAME='your-nacos-user'
export NACOS_PASSWORD='your-nacos-password'
go run ./operator/cmd/server
# 另一终端使用相同 Nacos 环境变量
go run ./admin
```

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev:platform
# 商户端先复制并填写 merchant-admin/.env.development.local.example
pnpm dev:merchant
```

平台地址 `http://localhost:15888`，商户地址 `http://localhost:15889`。全新环境默认平台账号 `admin / Abcd@1234`，商户使用对应租户管理员账号。

## 生成与验证

```sh
cd backend
make api openapi ts
make wire-check
go test ./...
go build ./admin ./operator/cmd/server
```

```sh
cd frontend
npm run typecheck
npm run test:tenant-routes
npm run build
```

MinIO 和 Nacos 集成测试需要对应服务。MinIO 测试连接通过 `MINIO_TEST_ENDPOINT`、`MINIO_TEST_ACCESS_KEY`、`MINIO_TEST_SECRET_KEY` 指定。部署和配置见 [服务指南](docs/service_split.md)，双前端见 [前端指南](frontend/README.md)，其他说明见 [文档索引](docs/README.md)。
