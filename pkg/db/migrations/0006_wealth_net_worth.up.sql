-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'net_worth_entry_kind') THEN
        CREATE TYPE net_worth_entry_kind AS ENUM (
            'asset',
            'liability'
        );
    END IF;
END
$$;

-- Manual entries (assets/liabilities) to support net worth tracking before bank sync exists.
CREATE TABLE net_worth_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind net_worth_entry_kind NOT NULL,
    name TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    as_of TIMESTAMPTZ NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT net_worth_entries_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_net_worth_entries_user_id_as_of ON net_worth_entries (user_id, as_of DESC);
CREATE INDEX idx_net_worth_entries_user_id_kind_as_of ON net_worth_entries (user_id, kind, as_of DESC);

CREATE TRIGGER trigger_set_net_worth_entries_updated_at
BEFORE UPDATE ON net_worth_entries
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Precomputed snapshots for fast charts and “runway” style metrics.
CREATE TABLE net_worth_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    as_of TIMESTAMPTZ NOT NULL,
    total_assets_minor BIGINT NOT NULL,
    total_liabilities_minor BIGINT NOT NULL,
    net_worth_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT net_worth_snapshots_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX uniq_net_worth_snapshots_user_id_as_of
ON net_worth_snapshots (user_id, as_of);

CREATE INDEX idx_net_worth_snapshots_user_id_as_of ON net_worth_snapshots (user_id, as_of DESC);
-- +goose StatementEnd

