# 做市管理部署

平台与商户端复用既有 Hertz/admin → Kratos/operator，operator 使用 mTLS 调用独立 Muskdeer strategy Console gRPC。管理数据库不复制业务表、不配置业务数据库 DSN；只新增商户与做市账户绑定。

## 配置和部署顺序

1. 备份管理库、业务库和 Nacos 配置。四核心升级使用对应工作分支，停止并排空受管理的挂单、等待租约过期后运行 `instanceupgrade` 升级独立实例 schema 到版本 3。维护命令仍只从 env 连接 Nacos；不重建账户、订单或开发批次。
2. 为 operator 配置同一 CA 下精确 `operator.grpc` SAN 的客户端证书。strategy 的 `grpc_security.allowed_peer_services` 显式加入 `operator.grpc` 后才注册 Console。保留核心间原来的身份认证和最终 fencing。
3. 在独立管理库运行 `go run ./operator/cmd/muskdeer-migrate`，从 env → Nacos 获取管理数据库配置，仅显式安装本模块第 1 版绑定表。已存在表时命令拒绝重复安装，不执行全框架 AutoMigrate。
4. 在管理 Nacos `data.yaml` 的 `runtime.muskdeer` 配置 enabled、endpoint、server_name=strategy.grpc、root_cert_file、certificate_file、private_key_file、timeout_ms。样例见已有 data.yaml.example。env 只保留 Nacos 引导连接，证书只读挂载。
5. 部署 operator、admin，启动会校验模块表并幂等合并本模块菜单/权限/API。不会清空其他模块的权限、角色和资源。模板管理员获得可向下分配的本模块权限；商户管理员不获得账户绑定管理能力。
6. 构建并部署平台与商户前端。沿用 Nginx 模板，ADMIN_API_UPSTREAM 指向内部 admin；商户部署设置 MERCHANT_CODE，代理覆盖浏览器传入的租户头。前端 VITE_AES_KEY 与 Nacos runtime.crypto_key 配对。

## 业务行为

- 包含总览、策略模板、实例新建/完整参数编辑/启停、品种、账户额度、订单/撤单、撮合原生挂单和持仓、开发行情发布记录；账户绑定仅对平台开放。
- 模板只读展示三种已注册实现及数据合同。实例创建保存为停止；参数编辑要求先停，保存后显式重启四核心加载。界面区分请求运行状态、实际装载修订和待重启状态。
- 启停与参数修改使用版本检查，不透明重试；超时或版本冲突先刷新。停止后执行服务受控撤销本实例管理的订单。单独撤单要求停止实例，结果需撮合快照确认。
- 商户身份只从可信 HTTP 网关注入，operator 每个请求查管理库绑定范围；请求 DTO 不接受 Actor 或 allowed_account_ids。没有绑定默认拒绝业务数据。绑定的账户/商户身份不可修改，禁用撤销后续访问，不能转交历史归属。
- WashTrade 是开发行情发布，0 次数/累计名义量为不限，不产生真实账户对刷成交。金额、数量、比例使用十进制字符串。
- 不提供实例删除和资金预算编辑，不以历史订单回执替代当前撮合快照。

## 校验命令

backend 执行 `make api openapi ts wire-check`、`go test ./...`、`go test -race ./operator/internal/data ./operator/internal/service -run Muskdeer`，构建 admin、operator 和维护命令。共享 MinIO 测试使用 MINIO_TEST_* 提供现有基础设施凭据；仅使用各测试自建逻辑资源。

frontend 执行 `npm run typecheck`、`npm run build`、`npm run test:tenant-routes`。业务服务的真实接口接通与 UI 夹具/模拟结果分别验收，部署账号及凭据只保存在私有部署记录，不写入仓库。
