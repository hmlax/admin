# Go Admin

平台管理后台（`platform-admin/`）与商户管理后台（`merchant-admin/`）是两套 React 19 + Ant Design 6 应用。两端共享 `src/` 的组件、布局、业务页面与 API 客户端，各自维护入口、页面清单、环境配置和构建产物，可分别部署到独立域名。

## 技术栈

| 类别 | 技术 | 说明 |
|------|------|------|
| 构建工具 | Vite 8 + @vitejs/plugin-react-swc | 快速 HMR，ESM 原生构建 |
| 核心框架 | React 19 + TypeScript 6 | 严格模式，路径别名 `@/` |
| UI 组件 | Ant Design 6 + ProComponents | 主题 Token，暗色模式 |
| 路由 | react-router-dom v6 | 前端/后端双模式动态路由 + 权限守卫 |
| 状态管理 | Zustand 5 | 轻量，支持 persist 中间件 |
| 数据请求 | Axios + TanStack Query 5 | 统一拦截封装，接口缓存/自动状态管理 |
| 国际化 | i18next + react-i18next | 命名空间分离，静态全量预加载 |
| 样式 | Tailwind CSS 4 + Less + AntD Token | 原子化 CSS + 主题变量 |
| 图表 | ECharts 6 + echarts-for-react | 数据可视化 |
| 富文本 | Tiptap 3 | 可扩展的富文本编辑器 |
| 工程化 | ESLint + Prettier + Husky + Commitlint | 代码质量 + Git 提交规范 |

---

## 快速开始

Node.js 使用 `^20.19.0` 或 `>=22.12.0`。在 `frontend/` 安装整个 workspace 的依赖：

```bash
pnpm install --frozen-lockfile
npm run dev:platform                  # http://localhost:15888
```

商户端需要先绑定已有且可用的租户编号：

```bash
cp merchant-admin/.env.development.local.example merchant-admin/.env.development.local
# 编辑 merchant-admin/.env.development.local 的 VITE_MERCHANT_CODE
npm run dev:merchant                  # http://localhost:15889
```

两个服务可以同时运行，分别使用独立的 Vite 缓存；HTTP `/admin/` 代理到 7788，SSE `/events` 代理到 7789。平台代理清除 `X-Tenant-Code`，商户代理将其固定为配置的编号。登录表单只收账号、密码和验证码，不能切换商户。

```bash
npm run typecheck                     # 两个应用及构建配置
npm run test:tenant-routes             # 路由、入口隔离与刷新会话回归
npm run build                         # 分别构建两个应用
npm run build:platform
npm run build:merchant
npm run preview:platform
VITE_MERCHANT_CODE=your-code npm run preview:merchant
```

默认 `dev`、`preview` 指向平台端。构建输出分别是 `platform-admin/dist`、`merchant-admin/dist`，压缩包在各自目录的 `dist.zip`。

生产包本地 preview 默认使用平台 16888、商户 16889 端口。

### 应用配置与边界

两套应用分别读取自身目录的 `.env`、`.env.development`、`.env.production`，本地覆盖写入对应 `.env.*.local`。根目录历史 `.env*` 不参与应用构建。公共 AES 密钥需与 Nacos 的 `runtime.crypto_key` 一致。

| 变量 | 作用 |
|------|------|
| `VITE_APP_TITLE` | 各自的浏览器标题与默认品牌 |
| `VITE_SERVER_PORT` | 平台 15888、商户 15889 |
| `VITE_API_URL` / `VITE_SSE_URL` | 默认同源 `/`、`/events` |
| `VITE_PROXY` | 开发 HTTP/SSE 上游 |
| `VITE_MERCHANT_CODE` | 商户开发与 preview 的部署编号，生产构建不需要它 |
| `VITE_AES_KEY` | 与后端一致的密码加密密钥 |

生产商户入口先从同源 `/app-config.json` 读取 `{ "merchantCode": "your-code" }`，校验成功后才加载 React 和状态存储；编号缺失、非法或请求失败会停止启动。平台入口不读取商户配置。商户镜像可用不同 `MERCHANT_CODE` 重复部署。

`@app/` 在构建期指向当前应用。平台 `routes.ts` 收集本地业务路由；商户固定使用后端按角色与套餐返回的菜单。两端的 `pages.ts` 决定可加载的页面，商户产物不包含租户/套餐管理、菜单/API/权限点管理、全局通知配置、系统配置及服务器监控等平台页面。租户隔离与 API 授权继续由 admin/operator 执行。

认证、偏好、标签页、锁屏、语言、搜索历史和 BI 草稿均按应用与商户编号隔离；刷新调度读取当前入口对应的 `refresh_exp` 或 `refresh_exp_<merchantCode>` Cookie。升级后旧的共用 UI 偏好不会自动迁入两个新应用。

### Docker 部署

在 `frontend/` 构建两套镜像，使用现有本机后端：

```bash
MERCHANT_CODE=your-code docker compose up --build -d
```

也可单独构建和部署某一端：

```bash
docker build --build-arg ADMIN_APP=platform -t go-admin/platform-admin:local .
docker build --build-arg ADMIN_APP=merchant -t go-admin/merchant-admin:local .
docker run -d -p 15888:8080 go-admin/platform-admin:local
docker run -d -p 15889:8080 -e MERCHANT_CODE=your-code go-admin/merchant-admin:local
```

Linux 使用示例 `docker run` 命令时需添加 `--add-host=host.docker.internal:host-gateway`，Compose 已配置。`ADMIN_API_UPSTREAM`、`ADMIN_SSE_UPSTREAM` 可改为容器网络内的 admin 地址（含 `http://`）。镜像应用类型由构建参数固定，运行时不能切换；商户编号未配置时容器拒绝启动。

容器启动脚本生成运行配置，nginx 在 HTTP 和 SSE 转发时强制写入当前入口的商户头。配置和 HTML 禁止缓存，SSE 关闭缓冲。对外域名和 HTTPS 由部署入口配置。后端 operator gRPC 继续仅在内部网络开放。

`scripts/deploy/build-local-docker-image.sh` 调用同一个 Dockerfile，环境变量 `ADMIN_APP=merchant` 可选择商户镜像；脚本不会停止已有容器或删除已有镜像。

---

## 项目结构

```
src/
├── api/                        # API 层（两层架构，不存在 service/ 目录）
│   ├── generated/              # 后端 Proto 生成的类型 + Service Client（禁止手改）
│   ├── client.ts               # apiClient 单例（懒加载各 Service）
│   ├── hooks/                  # React Query Hooks（组件内使用）
│   ├── README.md               # 本层详细文档
│   └── index.ts
│
├── core/                       # 核心模块（通用、可复用）
│   ├── access/                 # 权限控制（useAccess / AccessControl）
│   ├── i18n/                   # 国际化（useI18n / 语言检测 / 格式化工具）
│   ├── preferences/            # 偏好设置（主题/布局/语言/存储到 localStorage）
│   ├── router/                 # 路由引擎（路由生成 / 菜单生成 / 类型定义）
│   ├── storage/                # 增强存储（TTL / 驱逐策略 / 批量操作 / 跨 Tab 同步）
│   └── transport/              # 网络传输
│       ├── rest/               # REST 客户端（Axios 封装 + 拦截器）
│       └── sse/                # SSE 客户端（服务端推送）
│
├── router/                     # 业务路由配置
│   ├── config/                 # 静态路由 / 认证路由 / 错误路由
│   ├── guards/                 # AuthGuard / GuestGuard
│   ├── modules/                # 业务路由模块（自动导入）
│   └── index.tsx               # AppRouter 入口
│
├── stores/                     # 全局状态（Zustand）
│   ├── auth.ts                 # 认证状态（token / refreshToken / forceLogout）
│   ├── user.ts                 # 用户信息（userInfo / roles / permissions）
│   ├── tabs.ts                 # 标签页状态
│   └── pageRefresh.ts          # 页面刷新控制
│
├── hooks/                      # 业务 Hooks
│   ├── useAuth.ts              # 认证统一入口（登录/登出/权限码获取）
│   ├── useDictCache.ts         # 字典缓存
│   ├── useProTableScrollY.ts   # ProTable 动态高度
│   ├── useTokenRefresh.ts      # Token 定时刷新
│   └── ...
│
├── locales/                    # 翻译资源
│   ├── zh-CN/                  # 中文（_core/ 核心 + _modules/ 业务模块）
│   └── en-US/                  # 英文（结构同上）
│
├── layouts/                    # 布局组件
│   ├── MainLayout/             # 主布局（侧边栏 + 顶栏 + 标签页 + 内容区）
│   ├── UserLayout/             # 用户布局（登录/注册）
│   ├── BlankLayout/            # 空白布局（错误页）
│   └── IFrameLayout/           # iframe 内嵌布局
│
├── pages/                      # 页面组件
│   ├── app/                    # 业务页面
│   │   ├── dashboard/          # 仪表盘
│   │   ├── permission/         # 权限管理（角色/权限点/权限组）
│   │   ├── opm/                # 组织人员（组织架构/职位/用户）
│   │   ├── tenant/             # 租户管理
│   │   ├── internal-message/   # 内部消息
│   │   ├── log/                # 审计日志（登录/API/操作/数据访问/权限）
│   │   └── system/             # 系统管理（菜单/API/字典/文件/任务/策略/语言）
│   └── core/                   # 核心页面
│       ├── auth/               # 登录 / 注册
│       └── error/              # 401 / 403 / 500 / 404 等
│
├── components/                 # 公共组件
│   ├── bussiness/AuthLayout/   # 认证页布局组件
│   └── common/                 # 通用组件（Editor / Loading / PageContainer）
│
├── utils/                      # 工具函数（树操作 / 颜色 / 日期 / 加密等）
├── styles/                     # 全局样式（CSS 覆盖 / 暗色模式 / 骨架屏）
├── config/constants.ts         # 全局常量
├── App.tsx                     # 根组件
├── main.tsx                    # 入口（bootstrap → render）
└── bootstrap.ts                # 初始化（i18n + RequestClient 配置）
```

---

## 核心模块文档

每个核心模块都有详细的 README，涵盖 API、使用方法和注意事项：

| 模块 | 文档 | 说明 |
|------|------|------|
| 权限控制 | [core/access/README.md](src/core/access/README.md) | useAccess Hook、AccessControl 组件、静态鉴权 |
| 路由系统 | [core/router/README.md](src/core/router/README.md) | 前端/后端路由模式、RouteMeta、菜单生成、懒加载 |
| 国际化 | [core/i18n/README.md](src/core/i18n/README.md) | 命名空间规范、useI18n、格式化工具、翻译键约定 |
| 偏好设置 | [core/preferences/README.md](src/core/preferences/README.md) | 主题/布局配置、ThemeProvider、持久化机制 |
| 存储管理 | [core/storage/README.md](src/core/storage/README.md) | TTL 过期、缓存驱逐、批量操作、多标签页同步 |

---

## 开发规范

### 路径别名

| 别名 | 指向 | 用途 |
|------|------|------|
| `@/` | `src/` | 业务代码导入 |
| `#/` | `types/` | 全局类型定义 |

```typescript
import { useAuth } from '@/hooks/useAuth';
import type { AppRouteObject } from '@/core/router';
```

### 新增业务页面流程

1. **创建页面组件**：`src/pages/app/<module>/<page>/index.tsx`
2. **添加路由配置**：在 `src/router/modules/` 下对应文件中添加路由（使用 `createLazyRoute` 懒加载）
3. **添加翻译文件**：在 `src/locales/zh-CN/_modules/` 和 `en-US/_modules/` 下添加对应 JSON
4. **添加 API Hook**（如需）：在 `src/api/hooks/` 下封装（内部直调 `apiClient`），并在 `src/api/hooks/index.ts` 导出

### 代码提交规范

项目使用 Husky + Commitlint 强制 Git 提交规范：

```bash
# 提交信息格式
<type>(<scope>): <subject>

# 示例
feat(user): 新增用户管理页面
fix(auth): 修复 token 过期后重定向丢失问题
docs(router): 补充路由模块文档
refactor(storage): 重构缓存驱逐策略
```

**Pre-commit** 自动执行 `lint-staged`（ESLint + Prettier）。

### 命名约定

| 类型 | 规范 | 示例 |
|------|------|------|
| 组件文件 | PascalCase | `UserDrawer.tsx` |
| 工具/Hook 文件 | camelCase | `useDictCache.ts` |
| 路由模块文件 | kebab-case | `internal-message.tsx` |
| 翻译 JSON | kebab-case | `login-audit-log.json` |
| Store 文件 | camelCase | `auth.ts` |
| 样式文件 | kebab-case / camelCase | `AuthLayout.style.less` |
| i18n 命名空间 | JSON 文件名 | `user`、`permission`、`routes` |

---

## 关键架构

### 应用启动流程

```
main.tsx
  → bootstrap()
    → initI18n(locale)              // 初始化 i18next
    → RequestClient.init(...)       // 配置 Axios（getToken / refreshToken / 错误处理）
  → render()
    → QueryClientProvider           // TanStack Query
    → ThemeProvider                 // AntD 主题 + 暗色模式 + 水印 + 国际化
    → AntdApp                       // AntD 上下文（message / notification / modal）
    → App
      → useLocaleSync()            // 偏好设置 ↔ i18n 双向同步
      → AppRouter
        → useEffect(isAuthenticated)
          → getUserPermissionCodes() // 获取权限码
          → fetchAllDictEntries()    // 预加载字典
          → createAccessibleRouter() // 根据模式生成路由 + 菜单
        → <RouterProvider />        // 渲染路由
```

### 认证流程

- **登录**：`useAuth().login()` → 存储 token → 路由守卫放行 → 跳转原页面
- **Token 失效**：401 响应 → `forceLogout()` → 清除 token → 跳转登录页（携带 redirect）
- **Token 刷新**：`useTokenRefresh` Hook 定时检查，过期前自动调用 refreshToken
- **登出**：`useAuth().logout()` → 调用后端 /logout → 清除状态 → 跳转登录页

### 权限体系

- **权限码**：`GET /admin/v1/perm-codes` → `useUserStore.accessCodes`
- **角色码**：`GET /admin/v1/me` → `useUserStore.userRoles`
- **路由权限**：`meta.authority` 数组（角色码 + 权限码混合）
- **鉴权方式**：`useAccess()` Hook（条件渲染）、`<AccessControl>` 组件、`getAccessStatic()` 静态方法
- **超级管理员**：拥有 `*:*:*` 角色自动通过所有权限检查

### API 调用模式

```typescript
// 1. Generated 层（src/api/generated/）产出类型与 Service Client；src/api/client.ts 的 apiClient
//    单例以懒加载 getter 聚合它们。**没有独立的 service/ 层**，请求由 apiClient 直发。

// 2. Hook 层：封装 React Query（src/api/hooks/user.ts:39）
export function useListUsers(
  query: PaginationQuery,
  options?: UseQueryOptions<identityservicev1_ListUserResponse, Error>,
) {
  return useQuery({
    queryKey: ['listUsers', query],
    queryFn: () => listUsers(query),
    ...options,
  });
}

// 同文件内的私有函数才是真正发请求的地方：PaginationQuery.toRawParams() 摊平成
// 后端认识的扁平参数，再显式剥掉本接口用不到的键（user.ts:23-34）。
function listUsers(query: PaginationQuery) {
  const params = query.toRawParams();
  return apiClient.userService.List({
    ...params,
    sorting: undefined, offset: undefined, limit: undefined,
    token: undefined, filter: undefined, filterExpr: undefined,
  });
}

// 3. 页面中使用：必须 new PaginationQuery，分页参数在 paging 里
const { data, isLoading } = useListUsers(
  new PaginationQuery({ paging: { page: 1, pageSize: 20 }, formValues: { username: 'a' } }),
);

// 4. 写操作走 useMutation 的 mutateAsync（请求体须包 { data: {...} }）
const { mutateAsync: createUser } = useCreateUser();
await createUser({ data: values });
```

### 路由模式

| 模式 | 偏好设置 | 路由来源 | 说明 |
|------|----------|----------|------|
| 前端模式 | `accessMode: 'frontend'` | `src/router/modules/` | 静态定义 + 权限过滤（默认） |
| 后端模式 | `accessMode: 'backend'` | `GET /admin/v1/routes` | API 返回路由树 + 组件映射 |

通过偏好设置面板切换（通用 → 权限模式），刷新后生效。

### 状态管理

| Store | 文件 | 持久化 | 说明 |
|-------|------|--------|------|
| `useAuthStore` | `stores/auth.ts` | accessToken 持久化 | 认证状态、token、登录登出 |
| `useUserStore` | `stores/user.ts` | 不持久化 | 用户信息、角色码、权限码 |
| `useTabsStore` | `stores/tabs.ts` | 标签页数据持久化 | 标签页管理 |
| `usePreferencesStore` | `core/preferences/store/` | preferences 持久化 | 偏好设置 |
| `usePageRefreshStore` | `stores/pageRefresh.ts` | 不持久化 | 页面刷新控制 |

---

## 功能模块

### 已实现功能

- **仪表盘**：数据概览、图表展示
- **权限管理**：角色管理、权限点管理、权限组管理
- **组织人员**：组织架构管理、职位管理、用户管理（含用户详情）
- **租户管理**：多租户隔离、租户成员管理
- **内部消息**：消息列表、消息分类、收件箱
- **日志审计**：登录审计、API 审计、操作审计、数据访问审计、权限审计
- **系统管理**：菜单管理、API 管理、字典管理、文件管理、任务管理、登录策略、语言管理
- **个人中心**：用户资料编辑
- **偏好设置**：主题色 / 暗色模式 / 布局方式 / 侧边栏 / 标签页 / 快捷键
- **认证**：登录、注册、Token 刷新、登录过期处理

---

## 常见问题

### Q: 开发环境启动后接口 404？

检查 `.env.development` 中的 `VITE_PROXY` 配置是否指向正确的后端地址，确保后端服务已在 `7788` 端口启动。

### Q: 修改代码后页面没有更新？

Vite 的 HMR 通常能自动刷新。如果遇到状态残留，尝试手动刷新页面。路由级别改动（如新增路由模块）可能需要重启开发服务器。

### Q: 如何切换暗色/亮色模式？

点击顶栏右侧的主题切换图标，或打开偏好设置面板（顶栏齿轮图标）→ 外观 → 主题模式。支持 `light` / `dark` / `auto`（跟随系统）。

### Q: 如何新增一个后端接口的 API 调用？

1. 后端 proto 就位后执行 `cd backend && make ts`，刷新 `src/api/generated/`（本端 `package.json` 无 `generate:api` 脚本）
2. 在 `src/api/hooks/` 下创建 React Query Hook，内部直调 `apiClient.xxxService.Method()`（无 service 层）
3. 在 `src/api/hooks/index.ts` 追加 `export * from './xxx';`，页面组件 import Hook 使用

### Q: 生产构建如何分析包体积？

设置 `.env` 中 `VITE_ANALYZE=true`，然后执行 `pnpm build`，构建完成后会自动生成可视化分析报告。

---

## 相关链接

- [Vite 文档](https://vitejs.dev/)
- [React 文档](https://react.dev/)
- [Ant Design 6](https://ant.design/)
- [TanStack Query](https://tanstack.com/query/)
- [Zustand](https://zustand.docs.pmnd.rs/)
- [i18next](https://www.i18next.com/)
