# Go Admin

本分支采用 **React + Hertz API + Kratos operator gRPC 服务**。

```
React → admin HTTP :7788 → operator gRPC :7790 → 数据库 / Redis / 对象存储
           SSE :7789 ← gRPC 事件流 ← Redis 发布订阅
```

- `frontend/platform-admin` 为平台后台，`frontend/merchant-admin` 为商户后台，两套 React 应用独立构建和部署，共享组件与 API 客户端。
- `backend/admin`：HTTP、认证、RBAC、租户入口门禁、字段权限和审计。
- `backend/operator`：业务、GORM、任务和数据访问。
- `backend/platform`：公共依赖、共享协议、生成代码及构建部署工具；backend 仅有这三个顶级目录。
- Nacos 提供配置管理与服务发现；env 仅保存 Nacos 连接，应用配置读取 server.yaml、data.yaml。

## 本地启动

安装 Go（版本见 backend/go.mod）、Node.js 和 React 锁文件对应的 pnpm。先启动数据库、Redis 和对象存储，并在 Nacos 发布 server.yaml、data.yaml。

```sh
cd backend
export NACOS_ADDR='http://127.0.0.1:8848'
export NACOS_GROUP='GO_ADMIN'
export NACOS_NAMESPACE_ID=''
export NACOS_USERNAME='your-nacos-user'
export NACOS_PASSWORD='your-nacos-password'
# 终端一
go run ./operator/cmd/server
# 终端二，使用相同 Nacos 环境变量
go run ./admin
```

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev:platform
# 商户端：先将 merchant-admin/.env.development.local.example 复制为
# merchant-admin/.env.development.local，填入已有商户编号后启动
pnpm dev:merchant
```

平台地址为 `http://localhost:15888`，商户地址为 `http://localhost:15889`。全新环境平台账号为 `admin / Abcd@1234`，商户使用对应租户账号。两端均代理到 admin 的 HTTP/SSE 服务，入口门禁仍由后端校验。详见 [前端启动与部署](frontend/README.md)。

部署、Nacos、环境变量和迁移细节见 [服务拆分指南](docs/service_split.md)。功能专题见 [文档索引](docs/README.md)，开发规则见 [AGENTS.md](AGENTS.md)。

## 验证

```sh
cd backend
go test ./...
go build -o operator/bin/operator ./operator/cmd/server
go build -o admin/bin/admin ./admin
```

React 目录执行 `npm run typecheck` 和 `npm run build`。
