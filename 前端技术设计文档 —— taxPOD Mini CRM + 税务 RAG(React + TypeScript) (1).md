# 前端技术设计文档 —— taxPOD Mini CRM + 税务 RAG(React + TypeScript)

| 项目 | 内容 |
| --- | --- |
| 文档版本 | V1.0 |
| 对应需求文档 | PRD:taxPOD Mini CRM + 税务 RAG 问答 |
| 配套文档 | 后端技术设计文档(Go;REST 接口、错误码、RAG 与会议纪要提取的服务端流程) |
| 语言 / 框架 | TypeScript(strict)+ React 18 + Vite |
| 通信方式 | 纯 REST(JSON),不使用 WebSocket / SSE |
| 部署 | 静态站点(Vercel / Cloudflare Pages);本地 Vite dev server |

> **文档范围**:页面与组件、数据获取与状态管理、表单与校验、RAG 聊天界面、会议纪要审核流程、测试、构建与部署。服务端逻辑见后端文档。界面文案使用**英文**(面向马来西亚公司的评审人,且 RAG 同时支持中英文提问),不做国际化。
>
> **命名约定**:源码目录下所有文件名一律**小写 + 下划线**(如 `contact_form_dialog.tsx`)。React 组件在代码里仍用 PascalCase 命名(`ContactFormDialog`),文件名与组件名的对应关系是「驼峰转下划线」。工具强制要求的配置文件(`vite.config.ts`、`tsconfig.json`、`package.json`、`.env.example`)保持其约定名称。

---

## 1. 界面设计基准

### 1.1 整体页面结构

一个固定的应用壳:左侧导航 + 右侧内容区;没有登录状态,所以导航始终完整显示。

```
┌────────────┬────────────────────────────────────────────────┐
│ taxPOD CRM │  页面标题                          [页面级操作按钮]  │
│            ├────────────────────────────────────────────────┤
│ Dashboard  │                                                │
│ Leads      │                                                │
│ Pipeline   │              当前页面内容                       │
│ Tax Q&A    │                                                │
│ Meeting    │                                                │
│  Notes     │                                                │
└────────────┴────────────────────────────────────────────────┘
```

- 侧边栏(`sidebar.tsx`)五个入口,当前路由高亮;窄屏时折叠为顶部菜单。
- Meeting Notes 是 Option B,如未实现则不显示该导航项(由 `constants.ts` 里的功能开关控制)。
- 面向桌面浏览器优先;窄屏只保证可用、不横向滚动整页(看板在自己的容器内横向滚动)。

### 1.2 Dashboard

```
┌────────────────┬───────────────┬────────────────┬────────────────┐
│ Total Contacts │ Open Opps      │ Open Value     │ Won Value      │
│ 135            │ 12             │ RM 480,000.00  │ RM 120,000.00  │
└────────────────┴───────────────┴────────────────┴────────────────┘
┌─────────────────────────────────────────────────────────────────────┐
│ Recent Activity                                                     │
│ ● Call · Ahmad (Acme Sdn Bhd) · 2h ago                              │
│   Discussed e-Invoice readiness                                     │
│ ● Note · System · Stage: Contacted → Qualified · 5h ago             │
└─────────────────────────────────────────────────────────────────────┘
```

「Open Value」下方标注口径「Open opportunities only」,与 PRD D-3 一致。每个卡片是独立的 `StatCard`;动态流每条可点击跳到对应线索详情。

### 1.3 Leads 列表

```
┌─────────────────────────────────────────────────────────────────────┐
│ [🔍 Search name, email, company]  [Status: All ▾]   [+ New Lead]    │
├───────────┬──────────────┬──────────┬───────────┬─────────┬────────┤
│ Name      │ Email        │ Phone    │ Company   │ Status  │ Actions│
├───────────┼──────────────┼──────────┼───────────┼─────────┼────────┤
│ ...       │ ...          │ ...      │ ...       │ [Lead]  │ ✎  🗑  │
├───────────┴──────────────┴──────────┴───────────┴─────────┴────────┤
│ Showing 1-20 of 135                      [‹ Prev]  1 2 3 ...  [Next ›] │
└─────────────────────────────────────────────────────────────────────┘
```

点击一行进入线索详情(资料 + 商机 + 活动时间线)。新建 / 编辑用弹窗表单,删除用确认弹窗。

### 1.4 Pipeline 看板

```
┌─ New (3) ────┐ ┌─ Contacted (2) ┐ ┌─ Qualified ──┐ ┌─ Proposal ───┐ ┌─ Won ───┐ ┌─ Lost ──┐
│ RM 30,000    │ │ RM 18,000      │ │ RM 52,000    │ │ RM 40,000    │ │ RM 9,000│ │ RM 0    │
│ ┌──────────┐ │ │ ┌────────────┐ │ │ ┌──────────┐ │ │ ┌──────────┐ │ │         │ │         │
│ │ Title    │ │ │ │ Title      │ │ │ │ Title    │ │ │ │ Title    │ │ │         │ │         │
│ │ RM 10,000│ │ │ │ Contact    │ │ │ │ ...      │ │ │ │ ...      │ │ │         │ │         │
│ └──────────┘ │ │ └────────────┘ │ │ └──────────┘ │ │ └──────────┘ │ │         │ │         │
└──────────────┘ └────────────────┘ └──────────────┘ └──────────────┘ └─────────┘ └─────────┘
   [Board | List]  顶部视图切换;[+ New Opportunity]
```

每列列头显示「名称 + 数量 + 金额小计」;卡片显示标题、金额、关联联系人。Won / Lost 列用不同底色区分已关闭。「List」视图是按 Stage 分组的表格,作为拖拽之外的备选操作方式(见 6.2)。

### 1.5 Tax Q&A(RAG 聊天)

```
┌─────────────────────────────────────────────────────────────────┐
│ Tax Q&A                         Documents: [All ▾]              │
├─────────────────────────────────────────────────────────────────┤
│                                   ┌────────────────────────────┐│
│                                   │ 用户的问题(右对齐)       ││
│                                   └────────────────────────────┘│
│ ┌─────────────────────────────────────────────┐                 │
│ │ 答案正文 ... [1] ... [2]                    │                 │
│ │ Sources                                     │                 │
│ │  [1] 文档名 · pp.12-13 · §3.2   ▸ 展开片段  │                 │
│ │  [2] 文档名 · p.4 · §1.1        ▸ 展开片段  │                 │
│ └─────────────────────────────────────────────┘                 │
├─────────────────────────────────────────────────────────────────┤
│ [Ask a tax question...                              ] [Send]    │
│ General information only, not legal advice.            0/1000   │
└─────────────────────────────────────────────────────────────────┘
```

空线程时显示示例问题按钮;答案、引用、「找不到」、错误的渲染规则见 6.4。

### 1.6 Meeting Notes(Option B)

三步流程在同一个页面内切换:**粘贴笔记** → **审核草稿**(联系人、行动项、商机三个可编辑区块,每行可勾选或丢弃)→ **完成**(展示写入结果与跳转链接)。状态机见 6.5。

---

## 2. 技术栈选型

| 能力 | 选型 | 理由 |
| --- | --- | --- |
| 语言 | TypeScript(`strict: true`) | 用类型锁死前后端契约,编译期发现字段不一致 |
| 框架 | React 18 + Vite | 纯 SPA,无需 SSR;后端是独立的 Go 服务 |
| 路由 | React Router v6 | 标准方案;筛选状态放 URL 参数 |
| 服务端状态 | TanStack Query v5 | 缓存、失效、乐观更新、重试一步到位 |
| UI | Tailwind CSS + shadcn/ui(Radix) | shadcn 是把组件源码拷进项目,便于改样式、无运行时依赖锁定;Radix 提供无障碍基础 |
| 表单 | react-hook-form + zod | 校验规则与 TS 类型同源;`useFieldArray` 适合会议纪要审核表 |
| 看板拖拽 | @dnd-kit/core + @dnd-kit/sortable | 支持键盘传感器,维护活跃;比 react-beautiful-dnd(已停止维护)更稳妥 |
| HTTP | 原生 `fetch` 封装 | 需求简单,不额外引入 axios;原生支持 `AbortSignal` |
| 提示 | sonner | 轻量 toast |
| 图标 / 日期 | lucide-react / date-fns | — |
| 测试 | Vitest + React Testing Library + MSW | 与 Vite 同源;MSW 在网络层拦截,不污染业务代码 |
| 实时通信 | **不使用** WebSocket / SSE | 没有服务端主动推送的场景:RAG 是一次请求一次响应,看板靠乐观更新即可 |
| 全局状态 | 不引入 Redux / Zustand | 服务端状态交给 Query,筛选状态放 URL,其余是组件局部状态 |

**为什么 RAG 不做流式输出**:后端为了拿到结构化 JSON(`found` / `answer` / `used_excerpts`)并校验引用,使用的是 `responseSchema` 一次性返回,与流式输出冲突(后端文档第 12 章已列为二期)。前端用「加载中的骨架消息 + 阶段性文案」顶替,见 6.4。

---

## 3. 项目目录结构

```
web/
├── src/
│   ├── main.tsx
│   ├── app.tsx                          # QueryClientProvider + RouterProvider + Toaster
│   ├── router.tsx                       # 路由表 + 路由级 errorElement
│   ├── api/                             # 纯 TS,不含 React
│   │   ├── client.ts                    # fetch 封装:base URL、错误映射、AbortSignal
│   │   ├── contacts.ts
│   │   ├── opportunities.ts
│   │   ├── activities.ts
│   │   ├── dashboard.ts
│   │   ├── rag.ts
│   │   └── meeting_notes.ts
│   ├── types/                           # 与后端契约一一对应(见第 7 章)
│   │   ├── common.ts                    # Paginated<T>、ApiError、Money
│   │   ├── contact.ts
│   │   ├── opportunity.ts
│   │   ├── activity.ts
│   │   ├── dashboard.ts
│   │   ├── rag.ts
│   │   └── meeting.ts
│   ├── hooks/                           # 封装 query key 与失效规则
│   │   ├── use_contacts.ts
│   │   ├── use_opportunities.ts
│   │   ├── use_activities.ts
│   │   ├── use_dashboard.ts
│   │   ├── use_rag.ts                   # 提问 mutation + 文档清单 query
│   │   ├── use_meeting_notes.ts         # 提取 / 确认两个 mutation
│   │   ├── use_debounced_value.ts
│   │   └── use_url_filters.ts
│   ├── pages/                           # 只做编排,不写细节
│   │   ├── dashboard_page.tsx
│   │   ├── contacts_page.tsx
│   │   ├── contact_detail_page.tsx
│   │   ├── pipeline_page.tsx
│   │   ├── ask_page.tsx
│   │   ├── meeting_notes_page.tsx
│   │   └── not_found_page.tsx
│   ├── components/
│   │   ├── layout/                      # app_shell.tsx、sidebar.tsx、page_header.tsx
│   │   ├── common/                      # error_state.tsx、empty_state.tsx、loading_skeleton.tsx、confirm_dialog.tsx
│   │   ├── contacts/                    # contact_table.tsx、contact_form_dialog.tsx、delete_contact_dialog.tsx、status_badge.tsx
│   │   ├── pipeline/                    # kanban_board.tsx、kanban_column.tsx、opportunity_card.tsx、opportunity_form_dialog.tsx、stage_list_view.tsx
│   │   ├── activities/                  # activity_timeline.tsx、activity_form_dialog.tsx
│   │   ├── dashboard/                   # stat_card.tsx、recent_activity_feed.tsx
│   │   ├── ask/                         # chat_thread.tsx、message_bubble.tsx、citation_marker.tsx、citation_list.tsx、
│   │   │                                # question_input.tsx、example_questions.tsx、doc_type_select.tsx
│   │   ├── meeting/                     # notes_input.tsx、extraction_review.tsx、contact_review_row.tsx、
│   │   │                                # action_item_review_row.tsx、opportunity_review_section.tsx、result_summary.tsx
│   │   └── ui/                          # shadcn 生成的基础组件(文件名本就是小写)
│   ├── lib/                             # 全部是纯函数
│   │   ├── constants.ts                 # STAGES、STATUSES、ACTIVITY_TYPES、DOC_TYPES、功能开关
│   │   ├── query_keys.ts                # 所有 query key 工厂
│   │   ├── money.ts                     # 金额字符串的求和、比较、格式化(不经过浮点)
│   │   ├── format.ts                    # 日期、相对时间格式化
│   │   ├── citations.ts                 # 拆分答案文本中的 [n] 标记
│   │   ├── error_messages.ts            # 错误码 → 用户可读文案
│   │   └── cn.ts
│   └── test/
│       ├── setup.ts
│       ├── msw_handlers.ts
│       └── fixtures.ts                  # 测试用的联系人 / 商机 / RAG 响应样例
├── index.html
├── vite.config.ts
├── tsconfig.json
├── package.json
├── vercel.json                          # SPA 路由回退(见第 10 章)
└── .env.example
```

**分层原则**

- `api/` 只负责发请求和解析响应,不依赖 React,方便单独测试。
- `hooks/` 把 query key、失效规则收拢到一处,组件不自己拼 key。
- `pages/` 只拼装组件与 hooks;组件不直接调用 `api/`,必须经过 hooks。
- `lib/` 里全部是纯函数,最容易写单测。
- 依赖方向单向:`pages → components → hooks → api → types`,`lib` 与 `types` 可被任意层引用。

---

## 4. 路由与页面

| 路由 | 页面 | 数据来源 | 说明 |
| --- | --- | --- | --- |
| `/` | — | — | 重定向到 `/dashboard` |
| `/dashboard` | DashboardPage | `GET /dashboard` | 四张统计卡 + 最近活动 |
| `/leads` | ContactsPage | `GET /contacts` | 筛选参数在 URL:`?q=&status=&page=` |
| `/leads/:id` | ContactDetailPage | `GET /contacts/{id}`、`GET /activities` | 资料、商机、活动时间线;含「Ask a tax question」按钮 |
| `/pipeline` | PipelinePage | `GET /opportunities` | 看板 / 列表切换,视图选择放 URL:`?view=board\|list` |
| `/ask` | AskPage | `POST /rag/ask`、`GET /rag/documents` | 文档下拉选项来自 documents 清单;可带 `?company=` 作为 CRM 联动上下文(R-8) |
| `/meeting-notes` | MeetingNotesPage | `POST /meeting-notes/extract` 与 `/confirm` | Option B |
| `*` | NotFoundPage | — | 404 视图 |

路由级 `errorElement` 兜底未捕获的渲染异常,避免白屏。

---

## 5. 数据获取与状态管理

### 5.1 API 客户端(`api/client.ts`)

一个很薄的 `fetch` 封装,所有接口函数都走它:

- **Base URL**:来自 `import.meta.env.VITE_API_BASE_URL`,默认 `http://localhost:8080/api/v1`。
- **错误映射**:非 2xx 时解析后端统一错误体 `{ error: { code, message, request_id } }`,抛出 `ApiError`(带 `code`、`status`、`message`、`requestId`)。响应体不是合法 JSON(如网关 502 返回 HTML)时,退化为 `code = 'NETWORK_OR_GATEWAY'`,不让 `JSON.parse` 的异常直接冒到界面。
- **204 处理**:`DELETE` 返回 204 无响应体,封装里按状态码直接返回 `undefined`。
- **超时与取消**:每个请求都带 `AbortSignal`。CRUD 默认 15 秒超时;`/rag/ask` 与 `/meeting-notes/extract` 因为后端 Gemini 调用最长 30 秒(含重试更久),前端超时设为 45 秒。超时抛出 `code = 'CLIENT_TIMEOUT'`。
- **request\_id 透出**:错误 toast 与错误态组件里在小字展示 `request_id`,评审人或自己排查问题时可以直接去后端日志里搜。

### 5.2 Query key 与缓存失效

所有 key 由 `lib/query_keys.ts` 里的工厂函数生成,组件和 hooks 不手写数组。

| Query key | 数据 | 失效触发 |
| --- | --- | --- |
| `['contacts', 'list', { q, status, page }]` | 线索分页列表 | 线索增 / 改 / 删 |
| `['contacts', 'detail', id]` | 线索详情(含 `opportunity_count`、`activity_count`) | 该线索的改动;其商机或活动的增删 |
| `['opportunities', 'list', { stage?, contactId? }]` | 商机列表(看板用全量) | 商机增 / 改 / 删 / 换阶段;线索被删 |
| `['activities', 'list', { contactId?, opportunityId? }]` | 活动时间线 | 活动新增;商机换阶段(后端会写系统活动);会议纪要确认 |
| `['dashboard']` | 统计 + 最近 10 条活动 | **以上任何一类写操作之后** |
| `['rag', 'documents']` | 已导入文档清单 | 基本不变,`staleTime` 设为 1 小时 |

**失效规则的原则**:凡是会改变 Dashboard 数字或动态流的写操作(线索 / 商机 / 活动 / 会议纪要确认),`onSuccess` 里一律额外 `invalidateQueries({ queryKey: ['dashboard'] })`。这条规则写在各自的 `use_*.ts` 里,组件不用记。

### 5.3 重试与加载策略

| 场景 | 策略 |
| --- | --- |
| GET 查询 | 失败重试 1 次(只针对网络错误与 5xx);4xx 不重试 |
| 普通写操作(mutation) | 不自动重试,失败后由用户决定 |
| **AI 接口**(`/rag/ask`、`/meeting-notes/extract`) | **绝不自动重试**:后端已经做过指数退避,前端再重试会放大对免费额度的消耗;429 时只提示等待 |
| 窗口重新聚焦 | 仅 Dashboard 开启 `refetchOnWindowFocus`,其余关闭,避免无谓请求 |
| 翻页 / 改筛选 | `placeholderData: keepPreviousData`,旧数据保留到新数据到达,表格不闪烁 |

### 5.4 筛选状态放 URL(`use_url_filters.ts`)

线索列表的 `q`、`status`、`page` 全部读写 URL 查询参数,而不是组件 state:刷新页面、复制链接、浏览器前进后退都能还原现场。

- 搜索框是**受控的本地输入值**,经 `use_debounced_value`(300ms,对应 PRD L-3)后才写入 URL 并触发请求。
- 修改 `q` 或 `status` 时把 `page` 重置为 1。
- `status` 取值不在枚举内(用户手改 URL)时当作「All」处理,不报错。

### 5.5 金额处理(`lib/money.ts`)

后端金额全程是**字符串**(`NUMERIC(14,2)` → JSON 字符串),前端同样不让它进入 `number` 浮点域:

- 类型上用 `type Money = string`(形如 `'10000.00'`),在 `types/common.ts` 里集中声明。
- **求和、比较**(看板列头金额小计)用整数分:把字符串按小数点拆开,转成 `BigInt` 的分,相加后再格式化回两位小数字符串。不使用 `parseFloat`。
- **格式化展示**:千分位 + `RM` 前缀,例如 `RM 480,000.00`;格式化函数只做字符串层面的分组,不做数值运算。
- **表单输入**:Value 字段用 zod 校验正则(非负、最多两位小数),提交时原样以字符串发给后端。

---

## 6. 核心组件实现要点

### 6.1 Leads 线索管理(对应 L-1 \~ L-7)

**列表页(`contacts_page.tsx`)**

- 顶部三件套:搜索框、Status 下拉(All / Lead / Prospect / Customer)、「+ New Lead」按钮。三者叠加生效(L-3、L-4)。
- 表格 `page_size = 20`,默认后端按创建时间倒序(L-2)。分页器读 `total`、`page`、`page_size` 计算总页数。
- 三种非数据状态各自有组件:**加载**(`loading_skeleton`,保持表头不动,只替换行)、**空**(无任何线索时引导新建;有筛选但无结果时提示「清除筛选」)、**错误**(`error_state`,带重试按钮与 `request_id`)。

**新建 / 编辑弹窗(`contact_form_dialog.tsx`)**

- 同一个组件通过 `mode: 'create' | 'edit'` 复用;表单用 react-hook-form + zod。
- 校验:Name 必填;Email 必填且格式合法;Phone、Company 可空;长度上限与后端保持一致(以后端 DTO 为准,前端只做同样的上限提示,后端仍是最终裁决)。Status 默认 `lead`(L-1)。
- **后端错误回填到字段**:`409 CONFLICT`(email 已存在)直接设置为 Email 字段的错误「A lead with this email already exists」,而不是只弹一个 toast;`400 VALIDATION_ERROR` 其余情况以表单顶部提示展示。
- 提交成功后关闭弹窗、toast 成功提示、失效 `contacts` 与 `dashboard`;编辑成功后列表即时更新(L-5)。

**删除确认(`delete_contact_dialog.tsx`,对应 L-6)**

- 打开弹窗时才请求 `GET /contacts/{id}`,用返回的 `opportunity_count`、`activity_count` 渲染提示:「This will also delete N opportunities and M activities.」计数为 0 时省略这句。
- 计数加载期间确认按钮禁用,避免用户在没看到影响范围时就点了删除。
- 删除成功后:失效 `contacts`、`opportunities`、`activities`、`dashboard` 四类缓存(级联删除影响面)。如果当前在 `/leads/:id`,则跳回 `/leads`。

**详情页(`contact_detail_page.tsx`,对应 L-7)**

三个区块:基本资料(带编辑 / 删除按钮)、该线索的商机列表(可新建商机)、活动时间线(可新增活动)。顶部有「Ask a tax question」按钮,跳转 `/ask?company=<公司名>`(R-8,P2,见 6.4)。

### 6.2 Pipeline 看板(对应 O-1 \~ O-5)

**数据与分组**

- 一次 `GET /opportunities` 取全量(PRD 规模下商机数量有限),在前端按 `stage` 分组、列内按 `position` 升序。分组逻辑是 `lib` 或 hook 内的纯函数,便于单测。
- 列头小计 = 该列商机 `value_rm` 经 `money.ts` 求和;数量 = 该列长度(O-2)。Won / Lost 列底色不同,卡片上显示 `closed_at` 的日期。
- 卡片标题点击打开编辑弹窗;卡片上显示关联联系人姓名(后端列表需带 `contact_name`,若没有则前端用 `contact_id` 查缓存中的线索,实现时与后端对齐)。

**拖拽实现(`@dnd-kit`)**

- 传感器:`PointerSensor`(设置 `activationConstraint: { distance: 5 }`,避免点击被误判为拖拽)+ `KeyboardSensor`(无障碍)。
- 每一列是 droppable,卡片是 sortable。`onDragEnd` 算出**目标 stage** 与**目标列内下标**(去掉被拖卡片自身后的 0-based 位置),这正是后端 `PATCH /opportunities/{id}/stage` 要的 `{ stage, position }`。
- 允许任意两列之间拖拽(含回退与跳级),与后端「只校验取值合法、不强制线性流转」一致。

**乐观更新与回滚(对应 O-3)**

```
onMutate  : cancelQueries(opportunities) → 保存快照 → setQueryData 把卡片移到新列新位置
onError   : 用快照回滚 → toast 'Could not move opportunity. Changes reverted.'
onSettled : invalidate opportunities、activities、dashboard
```

- 失效 `activities` 与 `dashboard` 是因为后端在阶段确实变化时会写一条 `system` 活动(同一列内仅排序不写)。
- **并发保护**:某张卡片的 mutation 在途时,该卡片临时禁止再次拖动,避免同一张卡的两次请求乱序;后端用 `FOR UPDATE` 兜底,前端这一层只是减少无谓冲突。
- 拖到的新阶段是 Won / Lost 时,卡片即时显示关闭样式,但 `closed_at` 以服务端返回为准(`onSettled` 的重新拉取会覆盖)。

**List 视图(O-5,P1)**

- 按 Stage 分组的表格,每行末尾有「Move to…」下拉,调用同一个 `PATCH /stage`,`position` 取目标列末尾。
- 这是拖拽的**无障碍与窄屏降级方案**:触屏或键盘用户不必依赖拖拽也能推进商机。视图选择存在 URL 的 `?view=` 里。

**新建 / 编辑商机**:必须选择关联线索(下拉带搜索,数据来自线索列表)、Title 必填、Value 非负两位小数、Stage 默认 `new`(O-1)。从线索详情页进入时,线索字段预填且锁定。

### 6.3 Dashboard(对应 D-1 \~ D-4)

- 单个 `useDashboard()` 查询取 `GET /dashboard`,页面拆成 4 个 `StatCard` + 1 个 `RecentActivityFeed`。
- 四张卡:**Total Contacts**、**Open Opps**、**Open Value**(副标题固定写「Open opportunities only」,即 PRD 待确认项 1 的口径声明)、**Won Value**。金额字段直接用字符串格式化,不做运算。
- 动态流:每条显示类型图标、联系人姓名(链接到 `/leads/:contact_id`)、描述、相对时间(`date-fns`);`source = system` 的条目用弱化样式,与人工记录区分。
- 四张卡与动态流**共用一次请求**,所以加载、错误状态是整页统一的:加载时 4 张卡骨架 + 动态流骨架;失败时整块替换为 `error_state`。
- 口径若日后改为「全部商机」,只需改 `StatCard` 的副标题与后端字段映射,不改页面结构。

### 6.4 Tax Q&A 聊天界面(对应 R-4 \~ R-8,评估重点)

这是评审最仔细看的部分,前端要把后端做的三层防线(检索阈值、提示约束、引用校验)**原样呈现给用户**,而不是在界面上再掩盖一层。

**线程状态**

- 后端每次提问彼此独立(不带上文),所以前端的消息线程只是**展示用的本地数组**,刷新即清空,不持久化。
- 每条助手消息有一个类型标签,决定渲染方式:`pending` / `answer` / `not_found` / `degraded` / `error`。
- 同一时间只允许一个在途请求:发送期间输入框与示例问题按钮禁用,并提供「Cancel」按钮(`AbortController`)。

**五种消息的渲染规则**

| 类型 | 触发条件 | 渲染 |
| --- | --- | --- |
| `pending` | 请求在途 | 骨架气泡 + 阶段文案(0 秒「Searching documents…」,3 秒后「Writing answer…」)。阶段文案按计时器切换,只是等待体验,不代表真实进度,代码注释里写明 |
| `answer` | `found = true` | 答案正文 + 行内 `[n]` 角标 + 下方 Sources 列表 |
| `not_found` | `found = false` 且非降级 | 固定文案「I could not find this in the ingested documents.」(与后端固定文案一致)+ 灰色提示「Try rephrasing, or change the document filter.」不展示任何引用 |
| `degraded` | 响应带 `degraded = true` | 黄色提示「Answer generation failed. Showing the most relevant excerpts instead.」+ 返回的相关片段列表(与后端第 9 章降级策略对应) |
| `error` | 请求失败 | 红色气泡,按错误码给文案(见第 8 章),带「Retry」按钮重发同一问题 |

**引用渲染(`lib/citations.ts` + `citation_marker.tsx` + `citation_list.tsx`)**

1. 纯函数 `splitAnswer(text, validIds)` 把答案文本按形如 `[数字]` 的标记切成「文本段 / 引用段」数组。
2. **只有编号存在于响应 `citations` 里的标记才渲染成可点击角标**;其余看起来像 `[7]` 但没有对应出处的文本,保持为普通文字。后端已经校验过引用编号,前端再按 `citations` 兜一次,保证界面上不会出现点了没反应的角标。
3. 角标点击后,Sources 列表对应行高亮并滚动入视野;Sources 每行展示「`[n]` 文档名 · `pp.12-13` · `§3.2`」,右侧「展开片段」显示 `snippet`(后端已截取前 200 字符)。HTML 文档没有页码时,`pages` 为空则只展示 `section`。
4. **XSS 防护**:答案与片段一律作为**文本节点**渲染,不使用 `dangerouslySetInnerHTML`,也不引入 Markdown 渲染器。答案里的换行用 CSS `white-space: pre-wrap` 保留。这与后端安全章节「输出在前端按纯文本渲染」的约定对应。

**输入区(`question_input.tsx`)**

- 多行输入,`Enter` 发送,`Shift+Enter` 换行;字数计数器 `n/1000`,超过 1000 禁用发送(后端上限相同,前端提前拦截,省一次请求)。空白字符串不发送。
- 下方固定一行免责声明:「General information only, not legal advice.」
- 中英文提问都支持,前端不做语言检测;回答语言由后端 prompt 决定(跟随提问语言)。

**文档类型筛选(`doc_type_select.tsx`,对应 R-3)**

- 选项来自 `GET /rag/documents` 返回清单里的 `doc_type` 去重结果,外加 All;`doc_type` 值到展示名的映射放 `constants.ts`(`einvoice` → e-Invoice、`sst` → SST、`income_tax` → Income Tax、`other` → Other)。
- 选中非 All 时,请求体带 `doc_type`;All 时**省略该字段**,不传空字符串。
- 文档清单为空(尚未执行摄取)时,聊天页顶部显示提示「No documents ingested yet. Run the ingestion script first.」并禁用输入,而不是让用户提问后得到一个莫名其妙的「找不到」。

**示例问题(`example_questions.tsx`)**

线程为空时显示 4 个按钮,点击即填入并发送:取自 PRD 的三道可答题(e-Invoice 实施范围、SST 服务税注册门槛、e-Invoice 错误修改期限)+ 一道不可答题(2030 年企业税率),方便评审人一键验证「带出处」和「拒答」两种行为。

**限流与错误处理**

- `429 RATE_LIMITED`:气泡提示「Too many questions. Please wait a moment.」,发送按钮冷却 10 秒(后端未约定 `Retry-After`,冷却时长为前端常量)。
- `502 / 504`:提示 AI 服务暂时不可用,提供 Retry。
- 绝不在前端自动重试(见 5.3)。

**调试信息(可观测性,对应 PRD 非功能需求)**

每条 `answer` 气泡底部有一个默认折叠的「Details」,展示 `latency_ms`、检索块数量与最高相似度分数(来自 `retrieved`)。评审人想核对「检索命中」时可以直接看到;折叠状态下不干扰普通用户。

**与 CRM 联动(R-8,P2)**

线索详情页的「Ask a tax question」跳转 `/ask?company=Acme%20Sdn%20Bhd`;`AskPage` 读到 `company` 参数后,把输入框预填为 ` Regarding Acme Sdn Bhd:  `(只是预填文本,用户可改;后端接口不变)。这是 P2,时间不够就只保留按钮跳转 `/ask`。

### 6.5 Meeting Notes 提取与审核(Option B,对应 PRD 4.6)

核心是 **human-in-the-loop**:提取阶段不写任何业务数据,用户在审核表里逐项确认后才落库。页面是一个状态机:

```
idle ──[Extract]──▶ extracting ──成功──▶ reviewing ──[Confirm]──▶ confirming ──成功──▶ done
  ▲                     │                    │                          │
  │                     └─失败(保留原文)──▶ idle                       └─失败──▶ reviewing(草稿与改动保留)
  └──────────────[Start over / Discard draft]─────────┘
```

| 状态 | 界面 | 说明 |
| --- | --- | --- |
| `idle` | 大文本框 + 字数 `n/20000` + Extract 按钮 | 原文超过 20000 字符禁用按钮(与后端上限一致) |
| `extracting` | 按钮 loading,文本框只读 | 沿用 AI 接口 45 秒超时,不自动重试 |
| `reviewing` | 三个可编辑区块 + 摘要 | 见下 |
| `confirming` | Confirm 按钮 loading,整个表格只读 | 防重复点击;请求带 `meeting_note_id` |
| `done` | 写入结果摘要 + 跳转链接 | 新建 / 更新了几个线索、记录了几条活动,链接到对应线索 |

**审核区块(react-hook-form + `useFieldArray`)**

- **Contacts**:每行有「Include」勾选框,可改姓名、公司、职位、邮箱。后端预匹配到已有联系人时,该行显示徽标「Matches existing: <姓名>」,并提供单选「Update existing / Create new」,**最终新建还是合并由用户选**(与后端设计一致)。
- **Action items**:描述、负责人、截止日期(日期输入,值为 ISO 日期)。后端已按 Asia/Kuala\_Lumpur 的「今天」换算过相对日期,界面直接展示换算结果让用户核对。每行可勾选或丢弃。
- **Opportunity**:整块有总开关(默认关闭,避免没提到商机时被迫处理),开启后可改 Title、Value、建议阶段;`evidence`(模型引用的原文依据)以只读引用块展示在字段旁,方便对照。
- 缺失字段后端返回 `null`,表单里显示为空输入,**不要把 `null` 当作字符串展示**。

**确认与幂等**

- 提交内容只包含被勾选的行;全部被丢弃时 Confirm 按钮禁用。
- 确认请求携带 `meeting_note_id`。服务端已确认过的再次提交会返回 `409 CONFLICT`:前端把它当作「已经提交过」处理,提示并进入 `done` 视图,而不是报红色错误。
- 确认失败(非 409)回到 `reviewing`,**表单里用户已做的改动保留**。
- 确认成功后失效 `contacts`、`opportunities`、`activities`、`dashboard` 缓存。
- 离开页面保护:`reviewing` 状态且表单有改动时,关闭标签页触发浏览器的离开确认(`beforeunload`)。

### 6.6 Activity Log(对应 A-1 \~ A-3,P1)

- `activity_form_dialog.tsx`:Type(Call / Email / Meeting / Note)、Date、Description。入口有两处:线索详情页(绑定该线索)与商机卡片菜单(同时绑定商机,后端要求每条活动必属某个线索,所以商机入口会同时带上其 `contact_id`)。
- `activity_timeline.tsx`:按 `occurred_at` 倒序,每条显示类型图标、时间、描述;`source` 为 `system` 的条目(阶段变更)用弱化样式,`meeting_extractor` 来源的条目带一个小徽标,便于区分三类来源。
- 新增成功后失效 `activities` 与 `dashboard`(A-3:Dashboard 动态流读同一张表)。

---

## 7. 类型契约(与后端 5.9 一一对应)

`types/` 下的类型逐字段对应后端响应。JSON 字段名保持后端的 snake\_case,**不在 API 层做驼峰转换**(少一层映射就少一类 bug,评审人对照后端文档也更直观)。

```ts
// types/common.ts
export type Money = string;                       // '10000.00',永不转 number
export interface Paginated<T> { data: T[]; page: number; page_size: number; total: number }
export interface ApiErrorBody { error: { code: ErrorCode; message: string; request_id: string } }
export type ErrorCode =
  | 'VALIDATION_ERROR' | 'NOT_FOUND' | 'CONFLICT' | 'RATE_LIMITED'
  | 'UPSTREAM_ERROR' | 'UPSTREAM_TIMEOUT' | 'INTERNAL'
  | 'CLIENT_TIMEOUT' | 'NETWORK_OR_GATEWAY';      // 后两个是前端自己产生的

// types/contact.ts
export type ContactStatus = 'lead' | 'prospect' | 'customer';
export interface Contact {
  id: string; name: string; email: string;
  phone: string | null; company: string | null;
  status: ContactStatus; created_at: string; updated_at: string;
}
export interface ContactDetail extends Contact {
  opportunities: Opportunity[]; activities: Activity[];
  opportunity_count: number; activity_count: number;
}

// types/opportunity.ts
export type Stage = 'new' | 'contacted' | 'qualified' | 'proposal' | 'won' | 'lost';
export interface Opportunity {
  id: string; contact_id: string; title: string; value_rm: Money;
  stage: Stage; position: number; closed_at: string | null;
  created_at: string; updated_at: string;
}
export interface MoveStageRequest { stage: Stage; position: number }

// types/activity.ts
export type ActivityType = 'call' | 'email' | 'meeting' | 'note';
export type ActivitySource = 'manual' | 'system' | 'meeting_extractor';
export interface Activity {
  id: string; type: ActivityType; contact_id: string; opportunity_id: string | null;
  occurred_at: string; description: string; source: ActivitySource;
}

// types/rag.ts
export interface Citation { id: number; document: string; pages: string; section: string; snippet: string }
export interface AskRequest { question: string; doc_type?: DocType; top_k?: number }
export interface AskResponse {
  found: boolean; answer: string; citations: Citation[];
  retrieved: { chunk_id: number; score: number }[];
  latency_ms: number; degraded?: boolean;
}
```

**需要与后端文档对齐的细节**(后端 5.9 没有写死的字段,实现前确认一遍,以免联调时才发现):

| 项 | 当前前端假设 | 备注 |
| --- | --- | --- |
| `/dashboard` 字段名 | `total_contacts`、`open_count`、`open_value_rm`、`won_value_rm`、`recent_activities[]`(每条含 `contact_id`、`contact_name`) | 后端 5.3 的 SQL 别名为 `open_count` / `open_value_rm` / `won_value_rm`;联系人总数字段名待定 |
| 商机列表是否带联系人姓名 | 带 `contact_name` | 看板卡片要显示;不带则前端用缓存里的线索自行关联 |
| `/contacts/{id}` 里商机与活动的嵌套方式 | 内嵌 `opportunities`、`activities` 数组 | 若后端改为分开接口,详情页对应拆成两个 query |
| 降级响应(`degraded = true`)里的片段 | 放在 `citations` 数组,带 `snippet` | 后端第 9 章只说「附最相关的检索片段」 |
| 会议纪要 extract / confirm 的请求与响应体 | 见下方草案 | 后端只给了提取 Schema,没有给完整信封 |

```ts
// types/meeting.ts(草案,联调时以后端为准)
export interface ExtractedContact { name: string; company: string | null; role: string | null; email: string | null }
export interface ExtractedActionItem { description: string; owner: string | null; due_date: string | null }
export interface ExtractedOpportunity { title: string | null; value_rm: Money | null; suggested_stage: Stage | null; evidence: string | null }
export interface ContactMatch { contact_id: string; reason: 'email' | 'name_company' }
export interface ExtractResponse {
  meeting_note_id: string; summary: string;
  contacts: (ExtractedContact & { match: ContactMatch | null })[];
  action_items: ExtractedActionItem[]; opportunity: ExtractedOpportunity | null;
}
export interface ConfirmRequest {
  meeting_note_id: string;
  contacts: (ExtractedContact & { resolution: 'create' | 'update'; contact_id?: string })[];
  action_items: ExtractedActionItem[]; opportunity: ExtractedOpportunity | null;
}
```

---

## 8. 错误处理与用户文案

所有错误经 `ApiError` 进入 `lib/error_messages.ts`,**由错误码决定文案与行为**,组件不直接读 `message`(后端 `message` 面向开发者,仅作为小字补充)。

| 错误码 | 用户文案(英文) | 行为 |
| --- | --- | --- |
| `VALIDATION_ERROR` | 具体字段提示 | 表单内回填到字段或顶部;非表单场景用 toast |
| `NOT_FOUND` | This item no longer exists. | 详情页显示「Not found」视图并提供返回列表;列表操作则刷新缓存 |
| `CONFLICT` | 按场景:email 重复 / 纪要已确认 | email 重复回填到字段;纪要已确认进入 `done`(见 6.5) |
| `RATE_LIMITED` | Too many requests. Please wait a moment. | 仅出现在 AI 接口;发送按钮冷却 10 秒 |
| `UPSTREAM_ERROR` / `UPSTREAM_TIMEOUT` | The AI service is temporarily unavailable. | 提供 Retry;不自动重试 |
| `INTERNAL` | Something went wrong on our side. | 展示 `request_id` 供排查 |
| `CLIENT_TIMEOUT` | The request took too long. | 提供 Retry |
| `NETWORK_OR_GATEWAY` | Cannot reach the server. | 提示检查后端是否已启动(本地评审时最常见的原因) |

- **两层兜底**:组件级用 `error_state` 展示查询错误;路由级 `errorElement` 兜住未捕获的渲染异常,避免白屏。
- 写操作失败一律 toast + 保持用户已输入内容,不清空表单。

---

## 9. 测试策略

Vitest + React Testing Library,网络层用 MSW 拦截(fixtures 放 `test/fixtures.ts`)。优先级与后端一致:**先测最容易出错、最被关注的部分**。

| 层 | 对象 | 用例 |
| --- | --- | --- |
| 纯函数 | `lib/citations.ts` | 正常编号、编号不在 `citations` 内保持纯文本、连续角标 `[1][2]`、文本里无角标、角标在句首句尾 |
| 纯函数 | `lib/money.ts` | 两位小数求和、`'0.10' + '0.20' = '0.30'`、大金额、空列表为 `0.00`、格式化千分位 |
| 纯函数 | 商机分组函数 | 按 stage 分组、列内按 position 排序、空列也存在 |
| 组件 | RAG 聊天 | `answer` 渲染角标与 Sources、`not_found` 显示固定文案且无引用、`degraded` 提示、429 冷却、超 1000 字符禁用发送 |
| 组件 | 线索表单 | Email 格式校验、409 回填到 Email 字段 |
| 组件 | 删除确认 | 计数加载前按钮禁用、计数为 0 时省略级联提示 |
| 交互 | 看板乐观更新 | 模拟 PATCH 失败,断言卡片回到原列并弹出提示 |
| 交互 | 会议纪要审核 | 丢弃行不进入 confirm 请求;409 进入 `done` |
| 手工验收 | PRD 每个需求 ID | 录屏前按第 11 章对照表逐条过一遍 |

拖拽本身(`@dnd-kit` 的指针事件)在 jsdom 里难以可靠模拟,**不测拖拽手势,只测 `onDragEnd` 之后的计算与 mutation**:把「由拖拽结果推出 `{stage, position}`」抽成纯函数单测。

---

## 10. 构建、配置与部署

**环境变量(`.env.example`)**

```
# 后端 API 地址(含 /api/v1 前缀)
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

- 前端只有这一个变量。**不放任何密钥**:所有 LLM / Embedding 调用都在后端,浏览器里没有 API key。
- `VITE_` 前缀的变量会被打进静态产物,所以天然不适合放密钥,这也是前后端分离的好处。

**本地运行(对应 README 的「三条命令」)**

```
docker compose up -d db      # 起数据库
go run ./cmd/server          # 起后端 API(在 api/ 目录)
npm install && npm run dev   # 起前端(在 web/ 目录),默认 http://localhost:5173
```

前端默认端口 5173 与后端 `.env.example` 里的 `CORS_ORIGINS=http://localhost:5173` 一致。

**构建与部署**

- `npm run build` 产出静态文件到 `dist/`,可部署到 Vercel / Cloudflare Pages。
- SPA 路由回退:`vercel.json` 里把所有非静态资源路径重写到 `/index.html`,否则直接访问 `/leads/xxx` 会 404。
- 线上联调的唯一容易踩坑处是 **CORS**:部署后需要把前端的线上域名加进后端的 `CORS_ORIGINS` 白名单(后端不使用通配符)。
- 后端免费层可能休眠,首次请求会有冷启动。前端对此的处理:`NETWORK_OR_GATEWAY` / `CLIENT_TIMEOUT` 文案里提示「The server may be waking up, please retry」;演示前先预热。

**质量门槛**:`tsc --noEmit`(strict)、ESLint、`vitest run` 三项通过后才打录屏;`package.json` 提供 `npm run check` 一次跑完。

---

## 11. 与 PRD 的需求对应关系

| PRD 编号 | 前端实现 | 章节 |
| --- | --- | --- |
| L-1 新建线索 | `contact_form_dialog`(create) | 6.1 |
| L-2 列表与分页 | `contacts_page` + `contact_table`,`page_size = 20` | 6.1 |
| L-3 搜索 | 搜索框 + `use_debounced_value`(300ms)→ URL 参数 | 5.4 |
| L-4 状态筛选 | Status 下拉,与搜索叠加 | 5.4 / 6.1 |
| L-5 编辑 | `contact_form_dialog`(edit) | 6.1 |
| L-6 删除 | `delete_contact_dialog`(显示级联数量) | 6.1 |
| L-7 详情页 | `contact_detail_page` | 6.1 |
| O-1 新建商机 | `opportunity_form_dialog` | 6.2 |
| O-2 看板视图 | `kanban_board` + `kanban_column`(数量与金额小计) | 6.2 |
| O-3 拖拽换阶段 | `@dnd-kit` + 乐观更新与回滚 | 6.2 |
| O-4 编辑与删除 | `opportunity_form_dialog` + 确认弹窗 | 6.2 |
| O-5 列表视图(P1) | `stage_list_view` | 6.2 |
| D-1 \~ D-4 | `dashboard_page` + `stat_card` + `recent_activity_feed` | 6.3 |
| A-1 \~ A-3 活动记录(P1) | `activity_form_dialog` + `activity_timeline` | 6.6 |
| R-4 带引用回答 | `citation_marker` + `citation_list` | 6.4 |
| R-5 防幻觉 | `not_found` / `degraded` 消息类型,不展示引用 | 6.4 |
| R-6 聊天界面 | `ask_page` 全套组件(输入、消息流、加载、错误、示例、筛选) | 6.4 |
| R-3 文档类型过滤 | `doc_type_select` | 6.4 |
| R-7 评估 | 后端 `cmd/eval` 命令行完成,**前端不做评估界面** | — |
| R-8 CRM 联动(P2) | 详情页按钮 + `?company=` 预填 | 6.4 |
| Option B(P2) | `meeting_notes_page` 状态机 + 审核表 | 6.5 |

---

## 12. 已知限制与后续规划

- **看板规模**:一次取全量商机、前端分组。数据量上千后需要按列分页或虚拟滚动。
- **线程不持久化、不带上文**:与后端「每次提问独立」一致;多轮对话需要后端会话表配合(后端第 12 章已列二期)。
- **没有流式输出**:与后端结构化 JSON 输出冲突;二期可改为先流式输出正文、再单独返回引用。
- **无国际化**:界面固定英文,RAG 回答语言跟随提问语言。
- **无认证**:单一固定用户,是评估范围内的取舍,不是疏漏。
- **窄屏体验**:只保证可用;看板在窄屏建议用 List 视图。
- **离线 / 弱网**:没有离线缓存与请求队列,失败即提示。
- **字段名对齐**:第 7 章表格中的假设需在联调前与后端确认。

---

## 13. 小结

- **架构骨架**:Vite + React + TypeScript 的纯 SPA,`api → hooks → components → pages` 单向分层;服务端状态全部交给 TanStack Query,筛选状态放 URL,不引入全局状态库。
- **RAG 界面的核心取舍**:后端做了三层防线,前端把「找不到」「降级」「错误」做成独立的消息类型原样呈现;引用角标只渲染 `citations` 里真实存在的编号,答案按纯文本渲染防 XSS;AI 接口绝不自动重试,保护免费额度。
- **金额**:全程字符串,求和用整数分,从不经过浮点。
- **看板**:乐观更新 + 失败回滚,拖拽之外提供 List 视图作为无障碍与窄屏降级;写操作成功后统一失效 Dashboard 缓存。
- **会议纪要**:提取不写库、确认才落库,审核表逐项可丢弃,`meeting_note_id` 保证重复确认的幂等。
- **交付时的使用方式**:第 2、3、6.4、6.5 章可精炼后写进 ARCHITECTURE.md 的前端部分;「如果有更多时间我会怎么改」需在实际开发后据真实遇到的问题补写。
