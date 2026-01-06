-- +goose Up
-- Balance snapshots for user-settable starting balances
CREATE TABLE balance_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL DEFAULT 'EUR',
    snapshot_type VARCHAR(32) NOT NULL DEFAULT 'opening_balance',
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT balance_snapshots_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT balance_snapshots_type_chk CHECK (
        snapshot_type IN (
            'opening_balance',
            'adjustment',
            'reconciliation'
        )
    )
);

-- Only one opening balance per user
CREATE UNIQUE INDEX idx_balance_snapshots_user_opening ON balance_snapshots (user_id)
WHERE
    snapshot_type = 'opening_balance';

CREATE INDEX idx_balance_snapshots_user_id ON balance_snapshots (user_id);

CREATE INDEX idx_balance_snapshots_effective_at ON balance_snapshots (user_id, effective_at DESC);