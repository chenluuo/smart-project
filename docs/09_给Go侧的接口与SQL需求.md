# 给 Go 侧的需求清单（新增部分）

> 用途：汇总智能体侧需要 **Go 侧新增**的内容（SQL 变更 + 新增接口）；**Go 已有的接口不在本文件**（见 `08_智能体接口契约.md` §3.1 复用清单）
> 依据：`智慧农业系统架构设计.md`（框架基线，Go + PostgreSQL + TimescaleDB）、`智慧农业接口设计_修改后.md`（Go 已有接口，复用）、`08_智能体接口契约.md`（接口契约）
> 已定：消息**实时落 PostgreSQL**（复用 Go 已有 `chat_sessions / chat_messages`，智能体经写接口落库、不新增 SQL 表）；Redis 窗口为可重建缓存，LRU 逐出直接销毁；告警/阈值归 Go 侧；**向量库归智能体侧**（独立部署，智能体直连，Go 不碰）；SQL 扁平化（智能体不感知用户↔田块存储结构，`GET /plots` 按权限返回即可）
> 状态：**需求清单，Go 侧按此实现**

---

## 一、所需 SQL（仅 2 处，PostgreSQL 方言）

### 1.1 users 表：加个性化标签（2 列）

```sql
ALTER TABLE users
    ADD COLUMN interaction_style   VARCHAR(16) NULL,
    ADD COLUMN knowledge_reliance  VARCHAR(16) NULL;

COMMENT ON COLUMN users.interaction_style  IS '语言风格：plain/casual/professional';
COMMENT ON COLUMN users.knowledge_reliance IS '决策依据：experience/document/data';
```

> 仅供 agent 读取（System 段个性化），不进权限模型；`GET /api/v1/users/me` 返回时带上这两列。

### 1.2 knowledge_documents 知识文档表（新增，业务状态归 Go 管）

```sql
CREATE TABLE knowledge_documents (
    id            BIGSERIAL PRIMARY KEY,
    title         VARCHAR(255) NOT NULL,
    category      VARCHAR(64)  NOT NULL,
    file_url      VARCHAR(512) NOT NULL,
    file_hash     CHAR(64)     NOT NULL,
    source        VARCHAR(255),
    status        VARCHAR(16)  NOT NULL DEFAULT 'DRAFT',
    version       INT          NOT NULL DEFAULT 1,
    uploaded_by   BIGINT       NOT NULL REFERENCES users(id),
    approved_by   BIGINT       REFERENCES users(id),
    published_at  TIMESTAMPTZ,
    archived_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uk_knowledge_doc_version UNIQUE (title, version)
);

CREATE INDEX idx_knowledge_doc_status   ON knowledge_documents (status, version);
CREATE INDEX idx_knowledge_doc_category ON knowledge_documents (category);

COMMENT ON TABLE  knowledge_documents IS '知识文档元数据（全平台共享，状态机驱动版本切换）';
COMMENT ON COLUMN knowledge_documents.status IS 'DRAFT/APPROVED/ACTIVE/ARCHIVED（可用性由 Go 判断并保证）';
COMMENT ON COLUMN knowledge_documents.file_hash IS '文件 SHA-256，防重复上传';
```

> **Go 侧职责**：文档上传/审核/发布/下线、文件存 MinIO、可用性保证（`GET /knowledge/docs` 只返回可用文档）。
> **智能体侧职责**：收到通知（见 2.2）→ 拉文档向量化 → 写入自有向量库（**Milvus，独立部署，已定案归智能体侧**，智能体直连，Go 不碰；不使用 pgvector——其在 Go 的 PostgreSQL 内，会让智能体被迫直连业务库，违反"不直连数据库"定案）。

---

## 二、接口（Go 侧新增 2 个 + agent 侧暴露 3 个）

### 2.1 Go 侧新增（2 个，带 JWT）

| 方法/路径 | 入参 | 出参 | 说明 |
|---|---|---|---|
| `POST /agent/sessions/{id}/messages` | body: `{role, content, citations?, plot_id?, model_version}` | `{message_id, created_at}` | **每轮消息实时落库**（事实源写 `chat_messages`）；同时更新 `last_message_at` |
| `GET /api/v1/knowledge/docs` | query: category? | `[{id, title, category, version, file_url, updated_at}]` | 只返回**可用**文档（Go 保证可用性，对应 `knowledge_documents` 表）；ingest 加工源、检索元数据对照 |

### 2.2 文档通知（HTTP，Go 调 agent）

| 方法/路径 | 入参 | 出参 | 说明 |
|---|---|---|---|
| `POST /internal/knowledge/notify`（agent-service:8000） | body: `{doc_id, event: uploaded/updated/archived, version}` | `{ok: true}` | **Go 在文档上传/变更时调用**，携带**内部服务密钥**；agent 入队 `queue:doc.process` 异步向量化，立即返回不阻塞 |

### 2.3 智能体侧暴露的端口（agent-service:8000 提供）

供前端直接调用（**不经过 Go**，问答入口已从 Go 侧移出）：

| 方法/路径 | 入参 | 出参 | 说明 |
|---|---|---|---|
| `POST /agent/chat` | body: `{session_id?, plot_id?, question, context?}` | **SSE 流式**：answer 增量 + 引用片段 + 收尾意图（can_close） | 问答入口（JWT）；会话不存在则自动创建（agent 侧 Redis 会话状态） |
| `POST /agent/chat/sessions/{id}/close` | — | `{session_id, status: closed}` | 前端"结束会话"按钮 → agent 置 closed → 入队摘要 |
| `POST /internal/knowledge/notify` | body: `{doc_id, event, version}` | `{ok: true}` | 见 2.2（Go 调用） |

> 说明：智能会话端口由 agent-service 暴露（前端问答走 agent，SSE 流式）；`/agent/chat` 已从 Go 侧移出（接口文档 §8.1 已改为 agent-service 提供），Go 侧无需实现问答入口。

---

## 三、端口与调用约定

| 服务 | 端口 | 说明 |
|---|---|---|
| Go 主后端 | `8080` | REST，前缀 `/api/v1`（智能体侧同域调用） |
| agent-service | `8000` | REST + SSE（前端入口）+ `/internal/*` 通知路由（文档通知） |
| context-service | `8001` | REST（agent 内部调用） |
| tool-service | `8002` | REST（agent 内部调用） |
| ingest-service | `8003` | 无对外 HTTP，消费 Redis Stream（`queue:doc.process` / `queue:session.summary` / `queue:session.activity`）+ 写向量库（Milvus） |

- 服务间用 compose 内网 DNS（服务名+端口），不走公网；
- **认证分层**：JWT 仅 agent → tool/context → Go 链路透传；Go → agent（2.2 通知）用**内部服务密钥**（共享密钥头）；
- `trace_id` 贯穿：前端 → agent → tool/context → Go → LLM → 向量库；
- 统一响应格式沿用 `智慧农业接口设计_修改后.md` §1.3（`{code, message, data}`），分页沿用 `page/pageSize/total`。

---

## 四、Go 侧实现优先级

| 优先级 | 内容 | 说明 |
|---|---|---|
| **P0（小幅改造）** | 1.1 users 加标签列 + `GET /users/me` 带上 | 少量 ALTER |
| **P1（智能体阶段前）** | 2.1 `POST /agent/sessions/{id}/messages`（每轮消息落库，写 `chat_messages`） | 消息事实源链路 |
| **P2（智能体阶段）** | 1.2 knowledge_documents 建表 + 2.1 `GET /knowledge/docs` + 2.2 文档通知（调 agent `POST /internal/knowledge/notify`，内部密钥） | 知识库管理面 |
| **P2** | 2.3 agent 暴露端口（`/agent/chat` SSE、`/close`）：前端对接；Go 无需实现（已移出 Go 侧） | 前端问答链路 |
