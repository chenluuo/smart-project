# 02. 前端架构冻结

## 架构目标

在现有 `smart-project` 仓库中补充前端应用，形成一个移动端优先的智慧农业操作台。前端不替代后端职责，不重新设计业务模型，只作为现有 Go API、智能体接口和实时事件的用户界面。

## 前端技术栈

- 构建工具：Vite。
- UI 框架：React。
- 语言：TypeScript。
- 图标：lucide-react。
- 样式：原生 CSS，先不引入组件库。
- 路由：第一阶段暂不引入 React Router，使用轻量视图状态切换。
- 状态管理：第一阶段使用 React 内置 state/hooks；后续复杂化后再评估是否引入独立状态库。
- HTTP：浏览器 `fetch` 封装统一 API 客户端。
- 实时事件：使用 `fetch` 读取 `/api/v1/events/stream` SSE 流。

选择依据：

- 当前仓库原本没有前端，Vite + React + TypeScript 足够轻量，适合快速跑通移动端看板。
- 后端已有 REST/SSE 契约，前端不需要 SSR。
- UI 参考图偏移动端应用形态，原生 CSS 更容易精确控制卡片、手机面板和底部导航。

## 后端技术栈

沿用现有仓库后端，不在前端阶段调整：

- Go。
- Gin。
- GORM。
- MySQL。
- MinIO：知识文档对象存储，按后端配置启用。
- SSE：实时事件。
- Agent 服务：Python/FastAPI 方向，前端阶段只预留会话与消息历史入口。

## 数据库

数据库仍由后端管理，前端不直接访问数据库。

现有主要业务域：

- 用户与角色：`users`、`roles`、`user_roles`。
- 地块：`plots`。
- 设备：`devices`、`device_bindings`。
- 告警：`alert_rules`、`alerts`。
- 控制命令：`device_commands`。
- 通知、审计、Outbox：`notifications`、`audit_logs`、`outbox_events`。
- AI 会话：`chat_sessions`、`chat_messages`、`ai_suggestions`。
- 知识库：`knowledge_documents`。

数据库表结构和字段细节在阶段三通过 `03_database.md` 冻结。

## 项目目录

冻结后的仓库目录方向：

```text
smart-project/
├── agent/                      # 现有智能体相关服务
├── backend/                    # 现有 Go/Gin 后端
├── docs/
│   ├── frontend/
│   │   ├── 01_requirements.md
│   │   ├── 02_architecture.md
│   │   ├── 03_database.md      # 下一阶段生成
│   │   ├── 04_api.md           # 下一阶段生成
│   │   ├── 05_ui.md            # UI 阶段生成
│   │   └── 06_tasks.md
│   └── ...                     # 现有项目文档
└── frontend/                   # 新增前端应用
    ├── index.html
    ├── package.json
    ├── vite.config.ts
    └── src/
        ├── api.ts              # API 客户端
        ├── types.ts            # 前端 DTO 类型
        ├── main.tsx
        ├── App.tsx             # 当前阶段临时承载，可在模块化开发时拆分
        └── styles.css
```

后续模块化开发时，`frontend/src` 可逐步演进为：

```text
frontend/src/
├── api/
│   ├── client.ts
│   ├── auth.ts
│   ├── dashboard.ts
│   ├── plots.ts
│   ├── devices.ts
│   ├── controls.ts
│   ├── alerts.ts
│   └── knowledge.ts
├── components/
│   ├── AppShell.tsx
│   ├── BottomNav.tsx
│   ├── MetricCard.tsx
│   └── EmptyState.tsx
├── pages/
│   ├── LoginPage.tsx
│   ├── DashboardPage.tsx
│   ├── DeviceListPage.tsx
│   ├── DeviceDetailPage.tsx
│   ├── AlarmCenterPage.tsx
│   ├── ControlPanelPage.tsx
│   └── UserManagementPage.tsx
├── types/
│   └── api.ts
├── styles/
│   └── global.css
└── main.tsx
```

第一阶段允许先集中在少量文件里跑通；进入模块开发后，再按页面拆分，不做一次性大重构。

## 模块划分

### 登录模块

- 负责登录、注册、Token 保存、退出登录。
- 使用后端 `/api/v1/auth/*` 与 `/api/v1/users/me`。

### Dashboard 模块

- 负责移动端 A3 实时看板。
- 展示地块湿度、温度、在线设备、告警、灌溉状态。
- 这是最小闭环的核心页面。

### 地块模块

- 负责地块列表、地块详情、阈值规则查看与启停。
- 不做 GIS 地图。

### 设备模块

- 负责设备列表、设备详情、设备绑定、设备状态展示。
- 设备控制不放在设备列表中直接复杂化，控制能力归入控制模块。

### 控制模块

- 负责灌溉状态、开启灌溉、关闭灌溉、命令记录。
- 所有控制操作必须有明确按钮状态和错误提示。

### 告警模块

- 负责告警列表、告警确认、告警状态展示。
- 第一阶段不做复杂告警统计分析。

### 知识库模块

- 负责已发布文档列表。
- 管理员可上传文档。
- 审核、发布、归档可以后续单独补。

### AI 问答模块

- 负责创建会话、查看消息历史。
- 完整流式聊天、建议采纳、工具调用链路后置。

## 核心数据流

### 登录数据流

```text
用户输入账号密码
→ 前端调用 POST /api/v1/auth/login
→ 后端返回 accessToken 和 user
→ 前端保存 accessToken
→ 后续请求带 Authorization: Bearer <token>
→ 前端调用 GET /api/v1/users/me 校验登录状态
```

### Dashboard 数据流

```text
进入 Dashboard
→ 读取当前用户信息
→ 读取地块列表
→ 读取 Dashboard overview
→ 读取选中地块最新遥测
→ 读取选中地块灌溉状态
→ 读取设备和告警摘要
→ 渲染移动端 A3 看板
```

### 灌溉控制数据流

```text
用户点击开启/关闭
→ 前端生成 Idempotency-Key
→ 调用 POST /api/v1/plots/{plotId}/irrigation/commands
→ 后端返回 commandId/status
→ 前端刷新灌溉状态和命令记录
→ SSE 收到 command.result 时可局部更新
```

### 告警处理数据流

```text
前端读取 GET /api/v1/alerts
→ 用户确认活动告警
→ 调用 POST /api/v1/alerts/{alertId}/confirm
→ 前端刷新告警列表和 Dashboard 告警数量
```

### 实时事件数据流

```text
登录后连接 GET /api/v1/events/stream
→ 后端按用户隔离推送事件
→ 前端只保留少量最近事件
→ 关键事件触发局部刷新
```

## 开发约束

- 不修改后端架构。
- 不在前端绕过后端直接访问数据库。
- 不在当前阶段引入 UI 组件库。
- 不在未冻结 API 前继续猜接口。
- 页面开发按 `06_tasks.md` 当前任务推进。
- 每个模块完成后本地运行验证，再进入下一模块。

