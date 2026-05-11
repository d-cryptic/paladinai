-- Prompt versions table — stores versioned system prompts per agent+model combination.
-- Active prompt is the one with is_active = true for a given agent+model.
CREATE TABLE prompt_versions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name  TEXT        NOT NULL,  -- "supervisor" | "triage" | "rca" | "runbook" | "comms"
    model_id    TEXT        NOT NULL,  -- "claude-sonnet-4-6" | "qwen3-72b" | "deepseek-v3" | "*" (wildcard)
    version     INT         NOT NULL,
    content     TEXT        NOT NULL,  -- full system prompt content
    is_active   BOOLEAN     NOT NULL DEFAULT false,
    created_by  TEXT        NOT NULL DEFAULT 'system',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes       TEXT,
    CONSTRAINT uq_prompt_version UNIQUE (agent_name, model_id, version)
);

-- Only one active version per agent+model combo (partial unique index)
CREATE UNIQUE INDEX idx_prompt_active ON prompt_versions (agent_name, model_id)
    WHERE is_active = true;

CREATE INDEX idx_prompt_agent_model ON prompt_versions (agent_name, model_id, version DESC);
