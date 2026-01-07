-- +goose Up
-- Goal contributions table for tracking deposits/progress toward goals
CREATE TABLE goal_contributions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    goal_id UUID NOT NULL REFERENCES goals (id) ON DELETE CASCADE,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    note TEXT,
    transaction_id UUID REFERENCES transactions (id) ON DELETE SET NULL,
    contributed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT goal_contributions_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_goal_contributions_goal_id ON goal_contributions (goal_id);

CREATE INDEX idx_goal_contributions_goal_id_contributed_at ON goal_contributions (goal_id, contributed_at DESC);

CREATE INDEX idx_goal_contributions_transaction_id ON goal_contributions (transaction_id)
WHERE
    transaction_id IS NOT NULL;

-- Add occurrence tracking to recurring_subscriptions
ALTER TABLE recurring_subscriptions
ADD COLUMN IF NOT EXISTS occurrence_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE recurring_subscriptions
DROP COLUMN IF EXISTS occurrence_count;

DROP TABLE IF EXISTS goal_contributions;