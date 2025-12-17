-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_type') THEN
        CREATE TYPE account_type AS ENUM (
            'cash',
            'checking',
            'savings',
            'credit_card',
            'investment',
            'loan',
            'other'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_source') THEN
        CREATE TYPE transaction_source AS ENUM (
            'manual',
            'csv',
            'aggregator'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'goal_type') THEN
        CREATE TYPE goal_type AS ENUM (
            'save',
            'pay_down_debt',
            'spend_cap'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'goal_status') THEN
        CREATE TYPE goal_status AS ENUM (
            'active',
            'paused',
            'completed',
            'archived'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'wrapped_period') THEN
        CREATE TYPE wrapped_period AS ENUM (
            'month',
            'year'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'spreadsheet_canonical_field') THEN
        CREATE TYPE spreadsheet_canonical_field AS ENUM (
            'transactions',
            'categories',
            'accounts',
            'goals',
            'monthly_insights'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'recurring_status') THEN
        CREATE TYPE recurring_status AS ENUM (
            'active',
            'paused',
            'canceled'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'recurring_cadence') THEN
        CREATE TYPE recurring_cadence AS ENUM (
            'weekly',
            'monthly',
            'quarterly',
            'annual',
            'unknown'
        );
    END IF;
END
$$;

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name CITEXT NOT NULL,
    type account_type NOT NULL DEFAULT 'other',
    currency_code CHAR(3) NOT NULL,
    institution TEXT,
    last4 TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT accounts_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT accounts_last4_chk CHECK (last4 IS NULL OR last4 ~ '^[0-9]{4}$')
);

CREATE INDEX idx_accounts_user_id ON accounts (user_id);
CREATE INDEX idx_accounts_user_id_is_active ON accounts (user_id, is_active);

CREATE TRIGGER trigger_set_accounts_updated_at
BEFORE UPDATE ON accounts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id UUID REFERENCES categories (id) ON DELETE SET NULL,
    name CITEXT NOT NULL,
    color TEXT,
    icon TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_user_id ON categories (user_id);
CREATE INDEX idx_categories_parent_id ON categories (parent_id);

CREATE TRIGGER trigger_set_categories_updated_at
BEFORE UPDATE ON categories
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    account_id UUID REFERENCES accounts (id) ON DELETE SET NULL,
    category_id UUID REFERENCES categories (id) ON DELETE SET NULL,
    posted_at TIMESTAMPTZ NOT NULL,
    description TEXT NOT NULL,
    merchant_name TEXT,
    original_description TEXT,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    source transaction_source NOT NULL DEFAULT 'csv',
    external_id TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT transactions_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_transactions_user_id_posted_at ON transactions (user_id, posted_at DESC);
CREATE INDEX idx_transactions_user_id_account_id_posted_at ON transactions (user_id, account_id, posted_at DESC);
CREATE INDEX idx_transactions_user_id_category_id_posted_at ON transactions (user_id, category_id, posted_at DESC);
CREATE INDEX idx_transactions_merchant_name ON transactions (merchant_name);

CREATE UNIQUE INDEX uniq_transactions_user_id_source_external_id
ON transactions (user_id, source, external_id)
WHERE external_id IS NOT NULL;

CREATE TRIGGER trigger_set_transactions_updated_at
BEFORE UPDATE ON transactions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE goals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name CITEXT NOT NULL,
    type goal_type NOT NULL DEFAULT 'save',
    status goal_status NOT NULL DEFAULT 'active',
    target_amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    current_amount_minor BIGINT NOT NULL DEFAULT 0,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT goals_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT goals_target_amount_chk CHECK (target_amount_minor > 0),
    CONSTRAINT goals_date_range_chk CHECK (end_at >= start_at)
);

CREATE INDEX idx_goals_user_id ON goals (user_id);
CREATE INDEX idx_goals_user_id_status ON goals (user_id, status);
CREATE INDEX idx_goals_user_id_end_at ON goals (user_id, end_at);

CREATE TRIGGER trigger_set_goals_updated_at
BEFORE UPDATE ON goals
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Detected recurring charges/subscriptions in a user's spending.
-- (Separate from the SaaS billing table: `subscriptions`.)
CREATE TABLE recurring_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    merchant_name TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    cadence recurring_cadence NOT NULL DEFAULT 'unknown',
    status recurring_status NOT NULL DEFAULT 'active',
    first_seen_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    next_expected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT recurring_subscriptions_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_recurring_subscriptions_user_id ON recurring_subscriptions (user_id);
CREATE INDEX idx_recurring_subscriptions_user_id_status ON recurring_subscriptions (user_id, status);

CREATE TRIGGER trigger_set_recurring_subscriptions_updated_at
BEFORE UPDATE ON recurring_subscriptions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Precomputed monthly stats + highlights for a user.
CREATE TABLE monthly_insights (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    month_start DATE NOT NULL,
    total_spend_minor BIGINT NOT NULL DEFAULT 0,
    total_income_minor BIGINT NOT NULL DEFAULT 0,
    net_minor BIGINT NOT NULL DEFAULT 0,
    currency_code CHAR(3) NOT NULL,
    top_categories_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    top_merchants_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    highlights_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monthly_insights_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX uniq_monthly_insights_user_id_month_start
ON monthly_insights (user_id, month_start);

CREATE TABLE wrapped_summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    period wrapped_period NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    cards_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wrapped_summaries_date_range_chk CHECK (period_end >= period_start)
);

CREATE INDEX idx_wrapped_summaries_user_id_period ON wrapped_summaries (user_id, period);
CREATE INDEX idx_wrapped_summaries_user_id_period_start ON wrapped_summaries (user_id, period_start);

CREATE TABLE spreadsheet_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    file_name TEXT NOT NULL,
    storage_url TEXT,
    checksum_sha256 TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_spreadsheet_templates_user_id ON spreadsheet_templates (user_id);

CREATE TRIGGER trigger_set_spreadsheet_templates_updated_at
BEFORE UPDATE ON spreadsheet_templates
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE spreadsheet_mappings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL REFERENCES spreadsheet_templates (id) ON DELETE CASCADE,
    field spreadsheet_canonical_field NOT NULL,
    sheet_name TEXT NOT NULL,
    range_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_spreadsheet_mappings_template_id ON spreadsheet_mappings (template_id);
-- Prevent duplicates for the same template field mapping.
CREATE UNIQUE INDEX uniq_spreadsheet_mappings_template_id_field
ON spreadsheet_mappings (template_id, field);
-- +goose StatementEnd

