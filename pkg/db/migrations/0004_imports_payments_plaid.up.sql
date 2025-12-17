-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_file_type') THEN
        CREATE TYPE user_file_type AS ENUM (
            'csv',
            'xlsx',
            'pdf',
            'image'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'import_kind') THEN
        CREATE TYPE import_kind AS ENUM (
            'transactions',
            'invoice'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'import_status') THEN
        CREATE TYPE import_status AS ENUM (
            'pending',
            'running',
            'succeeded',
            'failed',
            'canceled'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'billing_provider') THEN
        CREATE TYPE billing_provider AS ENUM (
            'stripe'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'checkout_mode') THEN
        CREATE TYPE checkout_mode AS ENUM (
            'subscription',
            'payment'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'entitlement_type') THEN
        CREATE TYPE entitlement_type AS ENUM (
            'premium',
            'lifetime'
        );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'plaid_item_status') THEN
        CREATE TYPE plaid_item_status AS ENUM (
            'active',
            'error',
            'revoked'
        );
    END IF;
END
$$;

CREATE TABLE user_files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type user_file_type NOT NULL,
    mime_type TEXT NOT NULL,
    file_name TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum_sha256 TEXT,
    storage_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_files_size_bytes_chk CHECK (size_bytes >= 1 AND size_bytes <= 20000000),
    CONSTRAINT user_files_checksum_sha256_chk CHECK (checksum_sha256 IS NULL OR checksum_sha256 ~ '^[a-fA-F0-9]{64}$')
);

CREATE INDEX idx_user_files_user_id_created_at ON user_files (user_id, created_at DESC);
CREATE INDEX idx_user_files_user_id_type_created_at ON user_files (user_id, type, created_at DESC);

CREATE TABLE import_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES user_files (id) ON DELETE CASCADE,
    kind import_kind NOT NULL,
    status import_status NOT NULL DEFAULT 'pending',
    account_id UUID REFERENCES accounts (id) ON DELETE SET NULL,
    timezone TEXT,
    date_format TEXT,
    error_message TEXT,
    rows_total INT NOT NULL DEFAULT 0,
    rows_imported INT NOT NULL DEFAULT 0,
    rows_failed INT NOT NULL DEFAULT 0,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT import_jobs_rows_total_chk CHECK (rows_total >= 0),
    CONSTRAINT import_jobs_rows_imported_chk CHECK (rows_imported >= 0),
    CONSTRAINT import_jobs_rows_failed_chk CHECK (rows_failed >= 0)
);

CREATE INDEX idx_import_jobs_user_id_requested_at ON import_jobs (user_id, requested_at DESC);
CREATE INDEX idx_import_jobs_user_id_status_requested_at ON import_jobs (user_id, status, requested_at DESC);
CREATE INDEX idx_import_jobs_user_id_kind_requested_at ON import_jobs (user_id, kind, requested_at DESC);

-- Invoice/document storage (PDFs/images today; extracted transactions later).
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES user_files (id) ON DELETE CASCADE,
    merchant_name TEXT,
    issued_at TIMESTAMPTZ,
    total_amount_minor BIGINT,
    currency_code CHAR(3),
    extracted_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT documents_currency_code_chk CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_documents_user_id_created_at ON documents (user_id, created_at DESC);

CREATE TRIGGER trigger_set_documents_updated_at
BEFORE UPDATE ON documents
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Payments / Stripe
CREATE TABLE payment_customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider billing_provider NOT NULL DEFAULT 'stripe',
    provider_customer_id TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trigger_set_payment_customers_updated_at
BEFORE UPDATE ON payment_customers
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE payment_checkout_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider billing_provider NOT NULL DEFAULT 'stripe',
    mode checkout_mode NOT NULL,
    provider_session_id TEXT UNIQUE NOT NULL,
    stripe_price_id TEXT,
    checkout_url TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_checkout_sessions_user_id_created_at ON payment_checkout_sessions (user_id, created_at DESC);

CREATE TABLE payment_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider billing_provider NOT NULL DEFAULT 'stripe',
    provider_subscription_id TEXT UNIQUE NOT NULL,
    stripe_price_id TEXT,
    status TEXT NOT NULL,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    canceled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_subscriptions_status ON payment_subscriptions (status);
CREATE INDEX idx_payment_subscriptions_current_period_end ON payment_subscriptions (current_period_end);

CREATE TRIGGER trigger_set_payment_subscriptions_updated_at
BEFORE UPDATE ON payment_subscriptions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Durable entitlements. Subscriptions and one-time purchases grant entitlements.
CREATE TABLE user_entitlements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    entitlement entitlement_type NOT NULL,
    source_provider billing_provider,
    source_ref TEXT,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_entitlements_date_range_chk CHECK (ends_at IS NULL OR ends_at >= starts_at)
);

CREATE INDEX idx_user_entitlements_user_id ON user_entitlements (user_id);
CREATE INDEX idx_user_entitlements_user_id_entitlement ON user_entitlements (user_id, entitlement);
CREATE INDEX idx_user_entitlements_ends_at ON user_entitlements (ends_at);

CREATE TRIGGER trigger_set_user_entitlements_updated_at
BEFORE UPDATE ON user_entitlements
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Webhook event dedupe/idempotency.
CREATE TABLE payment_webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider billing_provider NOT NULL DEFAULT 'stripe',
    provider_event_id TEXT UNIQUE NOT NULL,
    event_type TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload JSONB NOT NULL
);

-- Plaid (post‑MVP) storage to support incremental sync + normalization into canonical transactions.
CREATE TABLE plaid_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plaid_item_id TEXT UNIQUE NOT NULL,
    institution_id TEXT,
    institution_name TEXT,
    status plaid_item_status NOT NULL DEFAULT 'active',
    -- Store tokens encrypted at rest in application code; DB stores opaque ciphertext/metadata.
    access_token_ciphertext BYTEA,
    access_token_kid TEXT,
    access_token_nonce BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_plaid_items_user_id ON plaid_items (user_id);

CREATE TRIGGER trigger_set_plaid_items_updated_at
BEFORE UPDATE ON plaid_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE plaid_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES plaid_items (id) ON DELETE CASCADE,
    plaid_account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    mask TEXT,
    type TEXT,
    subtype TEXT,
    currency_code CHAR(3),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT plaid_accounts_currency_code_chk CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX uniq_plaid_accounts_item_id_plaid_account_id
ON plaid_accounts (item_id, plaid_account_id);

CREATE INDEX idx_plaid_accounts_user_id ON plaid_accounts (user_id);

CREATE TRIGGER trigger_set_plaid_accounts_updated_at
BEFORE UPDATE ON plaid_accounts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE plaid_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES plaid_items (id) ON DELETE CASCADE,
    plaid_transaction_id TEXT NOT NULL,
    plaid_account_id TEXT NOT NULL,
    posted_at TIMESTAMPTZ NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    name TEXT NOT NULL,
    merchant_name TEXT,
    category_primary TEXT,
    category_detailed TEXT,
    raw JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT plaid_transactions_currency_code_chk CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX uniq_plaid_transactions_item_id_plaid_transaction_id
ON plaid_transactions (item_id, plaid_transaction_id);

CREATE INDEX idx_plaid_transactions_user_id_posted_at ON plaid_transactions (user_id, posted_at DESC);
CREATE INDEX idx_plaid_transactions_item_id_posted_at ON plaid_transactions (item_id, posted_at DESC);

CREATE TRIGGER trigger_set_plaid_transactions_updated_at
BEFORE UPDATE ON plaid_transactions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Link spreadsheet templates to files for unified storage (optional).
ALTER TABLE spreadsheet_templates
    ADD COLUMN IF NOT EXISTS user_file_id UUID REFERENCES user_files (id) ON DELETE SET NULL;
-- +goose StatementEnd
