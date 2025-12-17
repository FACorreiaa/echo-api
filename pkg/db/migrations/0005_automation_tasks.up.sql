-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'task_status') THEN
        CREATE TYPE task_status AS ENUM (
            'open',
            'done',
            'dismissed'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'task_priority') THEN
        CREATE TYPE task_priority AS ENUM (
            'low',
            'medium',
            'high'
        );
    END IF;
END
$$;

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT,
    status task_status NOT NULL DEFAULT 'open',
    priority task_priority NOT NULL DEFAULT 'medium',
    due_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_user_id_created_at ON tasks (user_id, created_at DESC);
CREATE INDEX idx_tasks_user_id_status_created_at ON tasks (user_id, status, created_at DESC);
CREATE INDEX idx_tasks_user_id_due_at ON tasks (user_id, due_at);

CREATE TRIGGER trigger_set_tasks_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE automation_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_automation_rules_user_id_created_at ON automation_rules (user_id, created_at DESC);
CREATE INDEX idx_automation_rules_user_id_enabled ON automation_rules (user_id, enabled);

CREATE TRIGGER trigger_set_automation_rules_updated_at
BEFORE UPDATE ON automation_rules
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

