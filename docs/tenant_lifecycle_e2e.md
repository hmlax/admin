# 租户与套餐 HTTP 自动化验收

在现有 infra-docker 部署上，从平台账号登录开始，真实调用 Hertz HTTP API 创建套餐、模块、配额和租户，随后使用新租户账号验证登录与权限。默认经过 React 开发服务器 `http://localhost:15888`，SSE 使用 `http://localhost:7789`。

## 执行

在 `backend` 目录运行：

```sh
python3 -m pip install -r platform/scripts/test/requirements.txt
make infra-test-tenant
```

已有 Python 环境可用 `make PYTHON=/path/to/python3 infra-test-tenant` 指定解释器。依赖 Docker CLI、OpenSSL、PyYAML，以及已部署且可用的 admin/operator、Nacos 和 Redis。

连接 Nacos 使用现有 `backend/.env.infra`。加密参数和 Redis 配置读取该 Nacos 分组的 `data.yaml`，脚本不修改 Nacos 配置。平台登录默认使用项目初始管理员；如果已经改密，通过测试进程的 `ADMIN_SMOKE_USERNAME`、`ADMIN_SMOKE_PASSWORD` 提供账号。测试参数与应用启动配置分开，服务启动仍只接受 Nacos 连接环境变量。

| 测试参数 | 默认值 | 用途 |
|---|---|---|
| `E2E_BASE_URL` | `http://localhost:15888` | HTTP 验收入口，可设为 `http://localhost:7788` 直测 admin |
| `E2E_TENANT_BASE_URL` | `http://localhost:7788` | 商户请求入口；生命周期隔离测试需直连网关以测试两个编号，单商户功能/权限测试可设为 `http://localhost:15889` |
| `E2E_TENANT_ENTRY_MODE` | `proxy` | 商户功能脚本的入口断言；直连网关时设 `gateway`，验证伪造商户头被拒绝；固定商户代理则验证该头被覆盖 |
| `E2E_MERCHANT_URL` | `http://localhost:15889` | 最终保留商户的浏览器登录地址 |
| `E2E_SSE_URL` | `http://localhost:7789` | SSE 入口 |
| `E2E_REDIS_CONTAINER` | `infra-redis` | 读取本次新验证码的 Redis 容器 |
| `NACOS_PUBLISH_URL` | `http://127.0.0.1:8848` | 宿主机访问 Nacos 的地址 |
| `ADMIN_SMOKE_USERNAME` | 项目初始管理员 | 平台测试账号 |
| `ADMIN_SMOKE_PASSWORD` | 项目初始密码 | 平台测试账号密码 |

验证码由真实接口生成，测试只读取自己刚创建的验证码答案，以小写形式提交，继续经过真实验证码验证。业务创建、修改和删除全部经过 HTTP 权限链路；脚本不写数据库，不绕过 JWT、RBAC 或租户门禁。

商户登录和后续请求通过 `X-Tenant-Code` 选择入口，使用按编号区分的刷新 Cookie；不再向平台登录表单或请求体传递租户编号。平台开发代理会清除商户头，商户开发代理会覆盖为部署固定编号，因此多商户隔离测试不能全部经过同一个平台或商户代理。

## 覆盖范围

- 套餐创建、列表检索、详情、更新和删除；模块白名单创建、详情、更新、删除与恢复。
- USER_LIMIT、STORAGE、API_CALL、AI_TOKENS 四类配额创建、查询、更新和删除；USER_LIMIT、API_CALL 的实际拦截与解除。
- 创建租户及管理员、套餐绑定、角色初始化、重复编号拒绝；弱密码失败时检查租户、用户、角色均回滚。
- 必填验证码、验证码一次性消费、大小写兼容、错误密码、错误租户及正确登录，HttpOnly 刷新 Cookie。
- `/me`、`/perm-codes`、`/routes`、`/initial-context` 内容一致性，刷新会话，SSE 建连，平台专属配置拒绝访问。
- 两个租户使用同名管理员和同名角色，分别验证密码和身份；用户列表隔离，跨租户读取与修改拒绝，伪造 tenantId 的新用户仍归属当前租户。
- 租户成员增删改查，用量与配额展示，无套餐拒绝、绑定后恢复、移除模块阻断与恢复。
- 移除 OPM 后递归检查菜单和初始化上下文不再含 OPM 页面、空目录被裁掉，恢复后菜单重新出现；移除 TENANT 后租户管理菜单隐藏且租户用量接口拒绝访问。
- OFF、FREEZE、EXPIRED 状态阻断登录及既有会话；重新启用后恢复；READONLY 到期允许读取、阻断写入，续期恢复。
- 用户改密、错误旧密码、旧密码失效、新密码登录；平台管理员重置密码；改密及重置后的旧访问令牌失效；退出后访问令牌和刷新 Cookie 失效。
- 权限管理继续由 `tenant_permissions.py` 串行验收：创建权限组和只读权限点、绑定真实 API 与菜单、建立租户只读角色和普通成员；验证字段隐藏、越权写入拒绝、角色撤权、权限点与角色禁用/恢复，以及成员换角色、禁用后的旧会话失效。
- 临时租户数据清理、租户删除及存在性检查；临时套餐、模块、配额删除。

本流程不调用真实 AI 或支付服务、不上传存储文件，也不等待每小时的到期扫描任务。STORAGE / AI_TOKENS 覆盖配置 CRUD；到期扫描及底层存储等行为由后端已有测试负责。HTTP 失败必须匹配预期状态及必要的错误原因，跨租户请求返回 500 会使测试失败。

React 的菜单来源另有路由回归：在 `frontend` 执行 `npm run test:tenant-routes`，验证
租户即使保存了前端模式偏好，也只挂载服务端允许的页面，直接访问被隐藏页面不会命中业务路由；
身份未加载或菜单加载器缺失时不会回退本地业务菜单。正常类型和构建检查仍执行 `npm run typecheck`、`npm run build`。

`/perm-codes` 和 `/initial-context` 同时返回有效角色、权限点关联的 `apiPermissions`（HTTP 方法和路由模板）。用户列表据此显示新建、编辑、删除、打印等操作和组织树；自定义只读权限点无需使用内置角色码即可正常打开授权菜单。授权事实不存入浏览器持久缓存，退出或切换身份时清空；接口仍执行完整权限校验。

## 报告和生成的登录账号

每次运行使用时间戳与随机后缀隔离数据，目录为 `backend/platform/artifacts/tenant-e2e/<run_id>/`：

- `report.md`：步骤结果和耗时，便于阅读。
- `report.json`：每一步的请求方法、路径、实际/预期状态、错误原因、耗时及当次资源 ID，便于 CI 归档。
- `credentials.json`：最终可用的新租户编号、用户名、最后一次重置后的密码、套餐 ID 和登录 URL；`ready: true` 表示最终登录验收成功。
- 上级目录的 `latest.json` 指向最近一次报告与凭据。

成功后保留本次主租户和套餐，临时隔离租户及临时删除测试套餐会在流程中清理。最终保留的账号已经验证登录并退出，可直接用于浏览器登录。失败立即返回非零退出码，保存原始异常的脱敏堆栈和当次资源清单；保留失败现场供排查，凭据里的 `ready` 不会标记为成功。

权限验收报告单独保存在 `backend/platform/artifacts/tenant-permissions/<run_id>/`。该目录的 `credentials.json` 同时包含租户管理员和 `reader` 只读成员账号，并记录保留套餐、租户和角色 ID。只读角色保留用户查询权限，隐藏邮箱和手机号字段。邮箱原本还受系统基础脱敏保护，移除字段隐藏只恢复基础脱敏值。

主流程、权限流程和浏览器验收应串行运行：它们会使用同一个平台账号，退出或吊销会话会影响该账号的其他测试。成员角色、状态以及角色字段权限/数据范围变更后，相关旧会话会被吊销，需要重新登录取得最新声明。

产物目录已加入 Git 和 Docker 排除项；凭据 JSON 创建时即设置为 0600，单次运行目录为 0700。报告不记录请求体、访问令牌、刷新 Cookie 或明文密码。不要把凭据作为普通 CI 日志发布。

## 已创建商户的功能验收

完成权限流程后，把商户前端绑定到保留的 `tenant_code`，在 backend 运行：

```sh
E2E_BASE_URL=http://localhost:15888 \
E2E_TENANT_BASE_URL=http://localhost:15889 \
python3 platform/scripts/test/merchant_functional.py
```

脚本默认读取 `tenant-permissions/latest.json` 指向的凭据，也可用 `E2E_CREDENTIALS` 指定凭据文件。只接受 `e2e_` 编号的测试商户。它验证组织、岗位、角色和成员增删改查，个人资料与头像，TOTP 注册和登录挑战，个人会话撤销，通知偏好零点值，内部通知收发与撤回，真实文件上传、下载及半开区间范围读取，访问凭证轮换与禁用，以及无需外部模型的 Agent 执行和运行记录。临时业务记录会清理，主商户和原有管理员、只读成员保留。

授权回归同时覆盖商户不能授予自己未持有的平台权限、不能执行平台任务调度和全平台会话管理。当前注册任务处理器及周期调度键属于平台共用资源，默认商户只读任务列表。角色表单读取的权限目录只包含当前操作者持有的权限，编辑普通字段不改写未修改的权限关联。

报告写入 `platform/artifacts/merchant-functional/<run_id>/`。外部模型、知识库向量检索、外部邮件和短信若未配置，会单独记为未覆盖；空列表读取或正确的 403 不代表这些外部业务已完成实测。浏览器验收须另外核对实际页面、操作反馈、刷新恢复和关闭保护，不能用 HTTP 测试代替。
