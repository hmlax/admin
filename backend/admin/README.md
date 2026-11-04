# admin：Hertz HTTP API

目录采用 [hz 的常规布局](https://www.cloudwego.io/docs/hertz/tutorials/toolkit/layout/)，与 operator 共用 backend/go.mod：

```text
admin/
  main.go                  # 进程入口
  router.go                # 文档等非业务路由扩展
  router_gen.go            # 调用 biz/router 的生成注册入口
  biz/
    handler/               # HTTP 参数绑定、类型化 gRPC 代理、文件与 SSE 处理
    model/                 # 独立 API protobuf 消息和 HTTP service 接口
    router/                # hz 生成的路由及 register.go
    mw/                    # JWT/RBAC、租户门禁和审计
  internal/
    bootstrap/             # 配置、客户端、监听生命周期与服务发现
    transport/             # 通用 Hertz 绑定、编码及路由元数据
  idl/proto/admin/v1/       # HTTP IDL
  .hz
  hz.yaml
  Makefile
  build.sh
  script/bootstrap.sh
```

在 backend 执行 `go run ./admin`，或在 admin 执行 `go run .` / `make run`。不要只运行 `go run main.go`，入口需要同目录的 router.go、router_gen.go。Nacos 启动环境变量及双文件配置规则仍见 [服务拆分说明](../../docs/service_split.md)。

路由调用链为 `main.go → router_gen.go → biz/router/register.go → 各服务路由 → biz/handler → operator gRPC`。SSE 使用独立监听，仍由同一 IDL 生成的 EventService 路由注册。

HTTP 接口和全部对外业务消息独立维护在 `idl/proto/admin/v1/*.proto`（`admin.v1`）。`*_types.proto` 定义 API 自己的请求、响应、实体、枚举和分页，服务 proto 定义 HTTP 路由、绑定和 SSE。Go 模型生成到 `biz/model/admin/v1/`，前端客户端生成到 `frontend/admin/react/src/api/generated/admin/v1/`。

operator 的内部消息和 gRPC 方法仍在 `../platform/proto/operator/v1/*.proto`（`operator.v1`），Go 类型在 `../platform/pb/operator/`。两边不互相导入业务 proto、不使用类型别名；只有 Google Timestamp、Empty、FieldMask 等标准类型通用。API 字段不是 operator 字段的自动副本，后续分别维护。

handler 先绑定 API 请求，再通过 `converters_generated.go` 的普通 Go 字段赋值转换为 RPC 请求；收到 RPC 回复后转换为 API 响应。转换保留字段缺省、零值 oneof、空列表、map、精确字符串、枚举和文件数据，不使用 JSON/二进制序列化或反射拷贝。API 字段或枚举没有对应映射时生成失败，必须显式调整适配逻辑；RPC 内部字段不会自动加入 API。转换后的 RPC 请求仍执行 RPC 自身的校验。

例如 `idl/proto/admin/v1/user.proto`：

```proto
service UserService {
  rpc Get (admin.v1.GetUserRequest) returns (admin.v1.User) {
    option (api.get) = "/admin/v1/users/:id";
  }

  rpc Create (admin.v1.CreateUserRequest) returns (google.protobuf.Empty) {
    option (api.post) = "/admin/v1/users";
    option (admin.http.body) = "*";
  }
}
```

`GetUserRequest` 定义路径/查询参数的字段类型；`:id` 必须对应请求字段。`CreateUserRequest` 定义 JSON 请求结构，`body = "*"` 绑定完整请求，因此仍接受 `{ "data": { ... }, "password": "..." }`。`body = "data"` 则将 JSON 绑定到该命名字段。查询参数支持 API proto 字段名及 JSON 名称，路径参数优先于同名请求体字段，FieldMask 语义保持。

附加入口也独立声明路由，但复用原 RPC：

```proto
rpc GetByUsername (admin.v1.GetUserRequest) returns (admin.v1.User) {
  option (api.get) = "/admin/v1/users/username/:username";
  option (admin.http.rpc) = "Get";
}
```

admin 不注册 `admin.v1` gRPC 服务，API 类型不会作为 operator gRPC 参数直接传输。`admin.http.rpc` 选择要调用的 RPC 方法，流方向必须匹配，但两套请求/响应是独立类型、字段编号可以不同。`events.proto` 的 `returns (stream admin.v1.EventFrame)` 定义 SSE；文件接口使用 API 自己的文件消息。

## 更新命令

安装与当前 `.hz` 配套的工具：

```sh
go install github.com/cloudwego/hertz/cmd/hz@v0.9.1
```

在 `backend` 执行：

```sh
# API 消息、HTTP 路由或请求体映射更新：生成 API protobuf，再调用官方 hz update。
make hz

# 同时修改了 operator 消息或 gRPC：分别生成 RPC 与 API，再更新适配。
make api

# 更新内嵌 OpenAPI 和 React 客户端。
make openapi ts
```

也可在本目录执行 `make hz`。`HZ=/path/to/hz` 可指定工具路径。

`make hz` 先校验全部 HTTP IDL 并生成绑定辅助代码，导出 Buf 依赖，再逐个执行以下官方命令。全部路由成功生成后才替换现有的 hz 路由文件，并更新 gRPC 客户端代理。HTTP 方法、路径和路由注册代码均来自 hz；自定义模板只接入现有 JWT/RBAC、typed protobuf 参数绑定与 SSE 编码链路。

```sh
# 在 backend 中仅调试一个 IDL 的 hz 输出；完整更新仍使用 make hz。
buf export . -o /tmp/go-admin-proto
hz update --idl admin/idl/proto/admin/v1/user.proto \
  --module go-admin --out_dir admin \
  --customize_package admin/hz.yaml \
  --use go-admin/admin/biz/model --no_recurse \
  -I admin/idl/proto -I /tmp/go-admin-proto
```

`--use` 复用的是 admin 自己生成的模型，不是 operator 模型。`buf.admin.gen.yaml` 独立生成 API protobuf；`hz.yaml` 输出 `biz/router`，`httpgen` 输出 API 绑定、显式转换和类型化代理，`gatewaygen.py` 只装配依赖及注册入口。HTTP 专属接口不再写入 `platform/pb/operator/*_http.pb.go`，该目录只保留 RPC 消息、客户端/服务端及 RPC 操作名。

OpenAPI 和 TypeScript 从 admin IDL 生成，描述符中不包含 operator 业务协议。Hertz 注解放在 `idl/proto/hertz/api.proto`，使用有命名空间的路径避免与第三方 SDK 的 api.proto 注册冲突。

协议更新后执行 `go test ./...`，分别构建 admin/operator，并运行前端类型和构建检查。新增端点的数据库权限资产同步流程见 [服务拆分说明](../../docs/service_split.md)。
