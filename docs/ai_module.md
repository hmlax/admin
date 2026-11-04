# AI 模块（提供商 / 流式对话 / 用量配额 / 知识库 RAG / 脚本与任务集成 / 安全洞察 / 智能问数）

> 持久化实现已于 2026-11-04 统一到 Kratos + GORM。本文保留原设计与历史实现记录；当前代码路径、模型生成和租户约束以 [后端 CRUD 指南](crud_module_guide/backend.md) 与 [租户隔离](tenant_isolation.md) 为准。

> **定位**：AI 模块的参考层文档：子域划分、流式对话语义、密钥与配额机制、知识库 RAG 链路与部署要求。
> 改 AI 相关代码（`platform/proto/operator/ai/`、`platform/pkg/ai/`、`internal/service/ai_*.go`、三端 `ai` 页面）前先读本文。

## 子域与数据模型

| 子域 | 表 | 说明 |
|---|---|---|
| 模型提供商 | `sys_ai_providers` | 一行 = 一个可调用端点（云端 OpenAI 兼容 / 本地 Ollama）；api_key AES-GCM 加密落库，读视图只有 `apiKeyHint` 脱敏 |
| 对话会话 | `sys_ai_conversations` | 归属用户（user_id），删除级联消息 |
| 对话消息 | `sys_ai_messages` | USER/ASSISTANT/SYSTEM 三角色；ASSISTANT 行带 tokens/耗时快照 |
| 用量流水 | `sys_ai_usage_logs` | 每次成功调用一行（只增不改），配额聚合的事实源 |
| 知识库 | `sys_ai_knowledge_bases` / `sys_ai_docs` / `sys_ai_chunks` | RAG：文档切片 → 向量化 → 余弦检索（`embedding` 向量列不进 ent schema，走启动期 SQL 补建 + 原生 SQL 读写） |
| 菜单语义搜索 | `sys_ai_search_index` | 全局搜索的菜单向量化索引：标题 embedding 余弦检索返回 title/route；经 `RebuildSearchIndex` RPC 原子重建（见下「菜单语义搜索」节） |
| 脚本集成 | — | 脚本内 `ai.chat / ai.chatWith / ai.chatWithSystem`（`platform/pkg/scripting/api/module_ai.go`），复用提供商解析与用量记账 |
| 定时任务 | — | `ai_doc_reindex`（向量重索引，按需）与 `ai_audit_digest`（审计日报，每日 08:00 常驻） |

## 流式对话语义

- `POST /admin/v1/ai/chat/completions` 是**普通 POST**（body 为扁平请求体，**不包 `data`**——与站内信 send 同型，多包一层会被 protojson 丢弃），同步返回完整回复（含会话与 tokens）。
- 生成过程中的增量 token 通过 SSE 网关（:7789）以 `ai_chat_chunk` 事件推送给发起用户（streamID=userId），事件 data 为 `ChatChunkEvent` 的 protojson（`conversationId`/`seq`/`delta`，camelCase）。**seq=0 时 protojson 省略零值字段**，前端不能依赖 seq 存在。
- chunk 是尽力而为（`TryPublish` 缓冲满即丢帧），以 POST 同步响应为最终事实。
- 流式时长由 context deadline（5 分钟）控制，不是 `http.Client.Timeout`——`platform/pkg/ai/client.go` 显式注入无超时 client 覆盖上游默认 30s，否则长回复被腰斩。
- 脚本 `ai` 模块与定时任务/洞察共用同一条 provider 解析 → 密钥解密 → 客户端工厂链路（`newOpenAIClientForProvider` / `ChatForScript`），不会出现第二套接入逻辑。

## 密钥与配置

- api_key 经全局加密器（`go-utils/crypto`，密钥来自 Nacos `data.yaml` 的 `runtime.crypto_key`）加密落库；应用启动时必须提供该密钥。
- Update 语义：apiKey 留空 = 不修改已存 Key（该字段从 updateMask 摘除，防 FieldMask 空值清写）。

## 配额与租户门禁

- `QuotaType.AI_TOKENS=4`（月度）：`chat` 前检查租户套餐配额，超限返回 400 "ai token quota exceeded for this month"；未配置该维度 = 不限量；平台用户（tenant_id=0）跳过检查。
- **embedding 计量口径（2026-09-28 起）**：全部 embedding 调用——RAG 入库/检索/chat 注入（`embedTextsForBase` 唯一咽喉）、菜单语义搜索的查询向量化与索引重建——与 chat 同口径写入 `sys_ai_usage_logs`。重索引等无操作者场景按知识库归属租户计量（`user_id=0`）。此前这些消耗对配额体系完全不可见。
  同批遗留的耗时缺口已补（2026-09-29）：这四处 DTO 原本没有 `duration_ms` 字段，流水页因此显示空值；现按 `ai_chat_service.go:213/237` 的既有写法（调用前取时间、写行前 `time.Since`）各自补上。**2026-09-29 之前写入的历史行不会回填**，`duration_ms` 保持 NULL，react 端渲染为 `-`（不是 `- ms`）。
  实测归属（48 行流水全表逐页读，`model|completionTokens|duration` 三元组）：库内 4 行 NULL 全是 `deepseek-chat` 且带 completion tokens（23/23/23/37）⇒ 来自 `GenerateContent`（表单旁的 ✨ 场景化内容生成），**不是** embedding。
  另三处 embedding 已在 2026-09-29 逐个活验证（把 `127.0.0.1:18080` 的 mock LLM 临时挂成 LOCAL 默认 provider 后逐条打通）：`RebuildSearchIndex` → 流水 `text-embedding-3-small | 7 ms`、`SemanticSearch` 查询向量化 → `text-embedding-3-small | 2 ms`、知识库入库 `embedTextsForBase`（建一个 probe 知识库 + 上传一条文档）→ `mock-embedding | 1 ms`，三行在 `/ai/usage` 均渲染成真值而非 `-`。模型名取自 `base.EmbeddingModel`（第三行是 `mock-embedding` 而非硬编码，证实这条路径读的是知识库自己的配置）。验证用的 provider/知识库/文档已删除，`sys_ai_search_index` 按快照逐字节还原（14/14 embedding 全等，id 回到 60029–60042）；三条流水留着当证据（id 50/51/52）。
- **用量页摘要与流水同口径（2026-09-29）**：`GetUsageSummary` 的 `MonthStats` 只在 `tenantId>0` 时加租户谓词——租户管理员按本租户统计，平台管理员统计全量，与他下方看到的全量流水列表一致（此前无条件 `TenantIDEQ(tid)`，平台管理员的卡是 47 条/3,259 tokens 而列表是 48 条，同屏两个数字对不上）。配额不受影响：租户分支谓词不变，平台侧本就跳过检查（上一条）且卡上显示 ∞。
- **套餐模块白名单**：AI 六服务已登记进 `platform/pkg/constants/module_mapping.go`（Module 枚举 `AI=11`），租户访问 AI 端点要求租户套餐的白名单里有 `AI` 模块行（`sys_plan_modules`），否则 403 "module not allowed"。漏登记的后果是 fail-closed 拒绝，不是放行。
- 菜单归类：`ComponentToModule` 已加 `app/ai/` 前缀 → AI 模块。
### 新部署注意：给租户放行 AI

`sys_plan_modules` 不会自动出现 `AI` 行，Api 表里 AI 端点的模块归类也不会自动出现——两步都要手动做，缺任何一步租户访问 AI 端点一律 403：

1. **接口同步（Api 表）**：在管理页触发「接口同步」。全量重建读的是打进二进制的 `assets/openapi.yaml`——若本版本比已部署版本新增了 AI 相关 proto，必须先 `make openapi`、重启进程、再触发同步，否则同步"成功"而新端点依旧不在表里。同步后 AI 六服务的端点以 `business_module=AI` 落进 Api 表（模块归类由 openapi 的服务标记经 `platform/pkg/constants/module_mapping.go` 的映射而来，AI 服务已登记）。
2. **套餐白名单加 `AI` 行**：管理页 → 租户管理 → 套餐管理（路由 `/tenant/plans`，仅平台管理员）→ 目标套餐行「编辑」→ 模块多选框勾选 `AI` → 保存。该多选框直接增删 `sys_plan_modules` 行；保存即生效——租户闸门每个请求实时查这张表，没有缓存、无需重启。无管理页的环境（直接操作数据库）SQL 等价直插：`INSERT INTO sys_plan_modules (plan_id, module) VALUES (<套餐ID>, 'AI');`（该表其余列均可空）。
3. **验证**：以该套餐下的租户用户调用任一 AI 端点（如 `GET /admin/v1/ai/providers`）。加行前 403；加行后应为 200。

仍 403 时按响应体 `message` 字段定位卡点。注意：前端界面对 403 一律按 reason 统一翻译、看不出区别，区分必须看 HTTP 响应体本身（curl 或浏览器网络面板）：

| message | 卡点 | 处置 |
|---|---|---|
| `access denied` | Api 表没有该 (path, method) 行（同步未触发、或 openapi 落后于代码），或白名单查询本身出错 | 重做第 1 步（含 `make openapi` + 重启）；仍如此则看服务日志的白名单查询报错 |
| `module not allowed` | 端点行存在但 `business_module` 归类缺失，或套餐白名单缺 `AI` 行 | 先确认第 1 步真把 `business_module=AI` 写进去了，再补第 2 步 |
| `no subscription plan` | 租户未关联任何套餐 | 租户管理里把套餐关联到该租户 |
| `tenant is not active` / `tenant is read-only due to expiry` | 租户停用 / 套餐到期只读策略挡住写操作 | 与 AI 白名单无关，按租户与套餐状态处置 |

## 知识库 RAG

链路：上传纯文本或文件 → 切片（500 字符/片，50 重叠）→ 调 provider 端点的 OpenAI 兼容 `/v1/embeddings` 批量向量化 → `sys_ai_chunks` 落库 → 检索（pgvector `<=>` 余弦距离 topK，SQL 层 tenant 过滤兜底）→ chat 携带 `knowledgeBaseId` 时把命中片段注入 system 消息（检索失败降级为普通对话，不阻断）。

**文件上传**：`POST /admin/v1/ai/knowledge-bases/{baseId}/docs/file`，body 为 JSON（`fileName` + `contentBase64`，protojson 的 bytes 即 base64——kratos 无 form-data codec 的既定形态）。抽取器在 `platform/pkg/doctext`：纯文本族直读、docx 走标准库 zip+XML 剥标签、pdf 走 `ledongthuc/pdf` 文本层（扫描件无文本层会明确报错），其余扩展名拒绝；上限 10MB。

**重索引**：更换知识库 embedding 模型后，经「任务管理」创建 `ai_doc_reindex` 类型任务（payload `{"baseId":N}`，0=全部库）触发全量重算——切片文本不变只换向量，分批 32 条/次。启动迁移因维度不匹配整列重建 `embedding` 后（见「部署要求」），该任务是恢复检索的唯一途径。

**审计日报**：系统常驻任务 `ai_audit_digest`（每日 08:00）聚合昨日操作审计（总数/失败/用户/动作分布），经默认模型生成 150 字摘要——按收件用户的偏好语言（个人中心 locale，中文/英文，未设置回落中文）分组渲染，每组一条消息行；LLM 失败自动降级为该语言的纯统计文本。

**部署要求（pgvector）**：

1. Postgres 必须带 pgvector 扩展（0.8.x 验证可用）。官方 `postgres` 镜像不含，需用 `pgvector/pgvector` 镜像或发行版包（Debian：`apt-get install postgresql-16-pgvector`，需 PGDG 源）。
2. 服务启动时自动执行（`data.EnsureVectorColumnDim`，知识库 `MigrateVectorColumn` 与菜单索引重建共用，幂等）：
   ```sql
   CREATE EXTENSION IF NOT EXISTS vector;
   ALTER TABLE sys_ai_chunks ADD COLUMN IF NOT EXISTS embedding vector(1536);
   ```
   **embedding 列定维 1536**（`data.AiEmbeddingDimensions`，即内置请求的 `text-embedding-3-small` 输出维度）。启动迁移会读取列的已声明维度：不匹配（含历史非定维列）即**整列重建、旧向量清空**——随后知识库必须经 `ai_doc_reindex` 任务重索引、菜单搜索索引必须重调 `rebuild-index` RPC 重建，恢复前相应检索返回空。
   `CREATE EXTENSION` 需要超级用户权限；失败仅 RAG 降级（检索/入库报错），其余功能不受影响。
3. **换 embedding 模型的边界**：`sys_ai_chunks` 与 `sys_ai_search_index` 的维度与 `text-embedding-3-small` 绑定。知识库配置了输出维度 ≠1536 的 embedding 模型时，向量化会在**写入期**收到维度不符报错（fail-fast，防止混维向量让 `<=>` 检索整体报错）；换模型必须同步改 `data.AiEmbeddingDimensions` 并触发全量重索引。
4. **embedding 向量列不进 ent schema**（ent 对 pgvector 自定义类型支持受限），切片读写走原生 SQL（`entClient.DB()`）。手改表结构时注意保持列名 `embedding` 与定维。
5. Docker 部署示例：把 `backend/platform/scripts/deploy/` 下的 compose 里 postgres 镜像换成 `pgvector/pgvector:pg16` 即可，其余不变。
6. **HNSW 自动门槛（2026-09-28 起，`sys_ai_chunks` 专属）**：启动迁移在定维检查后按**精确行数**（`data.CountTableRows`；不用 `pg_class.reltuples`——未 ANALYZE 前恒为 -1，会漏判）对照门槛 `data.HnswIndexRowThreshold = 10000`：越线即自动建 `CREATE INDEX ... USING hnsw (embedding vector_cosine_ops)` 并把本库 `hnsw.iterative_scan` 默认设为 `relaxed_order`（允许 planner 对带租户/知识库/文档过滤的 ANN 查询走迭代索引扫描），同时打 WARN 告警。菜单索引表不接此门槛（行数随菜单数有界，永不越线）。**越线后的语义变化**：ANN 检索变为近似召回（实测对抗性近重复数据上 top-8 尾部错位；`hnsw.ef_search` 可调）、距离序可能不严格（relaxed_order 的取舍）、写入承担索引维护代价（实测 ~3×/行，含 `ai_doc_reindex` 重索引任务）。阈值取 1 万的原因：建索引代价随表规模超线性增长（实测 1.1 万行 2.7s），越线即建是最便宜的建设时机，拖到更大表会把秒级构建拖成分钟级阻塞启动。门槛只负责建索引与放开 GUC，何时真正走索引由 planner 按选择性自主决定（实测 1/3 选择性下仍选顺序扫描）。
7. **向量负载隔离（部署层，规模触发）**：单表到十万级行或 RAG 成为常驻负载时，建议把向量负载迁出主 OLTP 库（独立实例/读副本），属 DSN 级部署改动、无需改代码——本仓不做自动迁移，出现上述 WARN 后人工评估即可。

## 菜单语义搜索（全局搜索）

三端全局搜索框（react 端已接入，HeaderContent 内 500ms 防抖后调 `/admin/v1/ai/content/search`）背后的菜单向量化索引：`POST /admin/v1/ai/content/rebuild-index`（`RebuildSearchIndex` RPC）把全部菜单标题向量化写入 `sys_ai_search_index`，搜索时向量化查询文本做余弦检索返回 `title`/`route`。

- **原子重建**：先离线算完全部 embedding，再单事务内 `DELETE`+分批多行 `INSERT`——任一环节失败整体回滚，旧索引原样保留，不会留下半空索引。`item_id` 为菜单真实 id（2026-09-28 前误存过数组下标）。
- **定维约束**：与 `sys_ai_chunks` 相同（见「部署要求」）。定维迁移导致列重建后，菜单索引为空，须重调本 RPC 重建。
- **V1 口径**：索引无租户列（菜单平台共享），检索已按调用者菜单权限过滤（2026-10-03）：`SemanticSearch` 经 角色ID→权限→菜单 链路解析调用者可见菜单集，SQL 加 `item_id IN (...)` 过滤；平台管理员不过滤（与 GetNavigation 口径一致）；权限解析前置在 embedding 之前（无可见菜单省一次向量调用）；解析失败 fail-closed（宁可不返回也不泄露越权菜单）。

## 智能问数（NL → 只读 SQL）

三端「智能问数」页：自然语言提问 → LLM 生成只读 SQL → 只读事务执行 → 结果表格 +（可选）自然语言结论。`POST /admin/v1/ai/query/ask`，响应含 `platform/sql/columns/rows/rowCount/answer/totalTokens`。

**五重安全护栏**（每轮生成的 SQL 全量过检，多轮追问不豁免）：

1. **只读语句**：仅 `SELECT`/`WITH` 开头；
2. **危险模式拒绝**：INSERT/UPDATE/DELETE/DROP/DDL/DCL、`pg_sleep`/`dblink` 等危险函数、多语句（分号）、SQL 注释（防注释截断绕过）；
3. **表白名单**：`FROM/JOIN` 涉及的每张表逐一对照白名单（平台 10 表 / 租户 6 表，见下）；
4. **LIMIT 钳制**：缺失补 100，超限改写；
5. **只读事务执行**：`READ ONLY` 事务，数据库层兜底。

**双白名单与租户谓词**：

- 平台用户（tenant_id=0）：10 张全平台表；
- 租户用户：6 张带 `tenant_id` 列的本租户数据表（操作审计/登录审计/AI 用量/用户/角色/收件记录），平台级表（sys_tenants/sys_plans 等）不在白名单；
- 租户 SQL 必须包含 `tenant_id = {自身ID}` 谓词——提示词写强制规则（含具体 ID），后置正则校验**缺失即拒绝**（fail-closed）。`sanitizeSQLFor(raw, whitelist, tenantPattern)` 带参化。

**拒答对抗（真实 DeepSeek 实测教训）**："登录失败记录"类措辞会触发模型安全拒答（理解成用户在查个人账号）。三层对策：①提示词声明"提问永远是对白名单表的查询，绝不拒绝"；②few-shot 示例放**真实消息序列**（user/assistant 对，约束力远强于 system 指令），租户路径的示例本身带 tenant_id 谓词；③`extractSQL` 非锚定提取（模型输出常带说明前缀/围栏，锚定 `^` 匹配不到）。

**多轮追问**：请求带 `history[]`（question+sql+resultSummary，建议 ≤5 轮），作为 user/assistant 交替消息传入消解指代（"那只看 admin 的"）；追问生成的 SQL 同样过全部护栏。

**结论生成**：`withAnswer=true` 时第二次调用把"问题+SQL+前 20 行结果"喂给模型生成结论（结果为空/结论失败不影响数据返回）。两次调用均记 `AI_TOKENS` 用量。

**权限**：平台用户查全平台表；租户用户查本租户表（仍需套餐 `AI` 模块白名单 + AI_TOKENS 配额，与对话一致）。

## 安全与异常洞察（dashboard 卡片）

三端分析页的「AI 安全与异常洞察」卡片：**规则预筛审计明细的行为模式，LLM 只生成总体评估措辞**——告警是确定性事实，不依赖模型编造；无告警时不调模型。

| 信号（24h 窗口） | 严重度 | 规则 |
|---|---|---|
| 疑似口令尝试 | HIGH | 单账号登录失败 ≥3 次（`sys_login_audit_logs.status='FAILED'`），detail 带最近来源 IP |
| 非常规时段操作 | MEDIUM | 本地 0-6 点（Asia/Shanghai）的操作 ≥1 次 |
| 操作失败集中 | MEDIUM | 单用户失败操作 ≥3 次 |
| 敏感操作 | LOW | DELETE/EXPORT/ASSIGN 明细（最多列 5 条） |

**i18n 架构（重要约定）**：后端只回结构化事实（`AiInsightAlert{severity, type, facts, items}`，type=BRUTE_FORCE/NIGHT_OPS/FAILED_OPS/SENSITIVE_OPS），**不生成任何人类文案**——告警标题/明细由前端 i18n 模板按界面语言渲染（三端各自 dashboard 文案文件里的 `aiInsights.alerts.<type>` 模板）；LLM 总评按请求 `lang` 生成（提示词尾部注入语言指示）。新告警类型 = 后端加规则 + 三端加模板键，缺一方该告警在前端显示原始键。

**权限**：平台用户专属（`tenant_id != 0` 一律 403）——洞察会读取全平台审计明细且把事实外发到模型端点，出域边界比普通统计接口严格。

## 三端页面

| 端 | 路由 | 内容 |
|---|---|---|
| react | `/ai/chat` `/ai/providers` `/ai/knowledge` `/ai/query` | 基准实现（流式 markdown 渲染、知识库选择器、文档上传、对话式问数） |
| vue-element | 同构 | ProPage + useDrawerForm |
| vue-vben | 同构（hash 路由） | VxeGrid + useVbenDrawer |

聊天页的知识库选择器：选中后每次发送携带 `knowledgeBaseId`，该会话的后续轮次都带检索注入。

## 已知边界

- e2e 与本地演示用 `mock_llm.py`（OpenAI 兼容；对话 echo、洞察请求按事实清单生成分析要点、RAG 请求引用片段、embeddings 为 bigram 词袋向量可断言检索）验证；真实云端模型（DeepSeek/通义）与本地 Ollama 走同一 OpenAI 兼容协议，未逐家实测。
- 会话标题默认取首条消息前 30 字符；重命名三端均已落地（react/vben 内联编辑 + ele prompt 弹窗，走 Update conversation + title 掩码；2026-10-03 核实）。
- 图像/多模态、agent 编排未做（eino/langchaingo 插件能力在上游已备，按需接入）。**Function Call 协议层已落地（2026-10-03）**：ai_tools.go 内置工具注册表（get_current_time 起步）+ aiToolLoop 多轮循环接入流式对话（工具轮不推 SSE、只流最终答案；轮数耗尽强制文本；用量跨轮累计）。新增工具 = defs 加定义 + execAiTool 加分支。前端无需改动。
