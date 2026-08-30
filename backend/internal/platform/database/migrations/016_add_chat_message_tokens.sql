-- 会话消息记录 LLM token 消耗（用户可查自己的今日/总消耗）
ALTER TABLE chat_messages
    ADD COLUMN prompt_tokens BIGINT NOT NULL DEFAULT 0 COMMENT 'LLM 输入 token 数',
    ADD COLUMN completion_tokens BIGINT NOT NULL DEFAULT 0 COMMENT 'LLM 输出 token 数';
