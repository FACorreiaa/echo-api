-- +goose Up
-- Category rules for learned transaction categorization
-- Part of the "Clean Room" feature for Echo OS

CREATE TABLE IF NOT EXISTS category_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    match_pattern TEXT NOT NULL, -- Pattern to match, e.g., '%APPLE%', '%REVOLUT%'
    clean_name TEXT, -- Cleaned merchant name, e.g., 'Apple Services'
    assigned_category_id UUID REFERENCES categories (id) ON DELETE SET NULL,
    is_recurring BOOLEAN DEFAULT false, -- Whether this merchant is typically recurring
    priority INTEGER DEFAULT 0, -- Higher priority rules match first
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW(),
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW()
);

-- Index for efficient rule lookup during import
CREATE INDEX IF NOT EXISTS idx_category_rules_user_pattern ON category_rules (user_id, match_pattern);

-- Index for finding rules by user
CREATE INDEX IF NOT EXISTS idx_category_rules_user_id ON category_rules (user_id);

-- Index for priority ordering
CREATE INDEX IF NOT EXISTS idx_category_rules_priority ON category_rules (user_id, priority DESC);

-- Unique constraint: one pattern per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_rules_unique_pattern ON category_rules (user_id, match_pattern);

-- +goose StatementBegin
-- Trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_category_rules_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trigger_category_rules_updated_at ON category_rules;

CREATE TRIGGER trigger_category_rules_updated_at
    BEFORE UPDATE ON category_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_category_rules_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trigger_category_rules_updated_at ON category_rules;

DROP FUNCTION IF EXISTS update_category_rules_updated_at ();

DROP INDEX IF EXISTS idx_category_rules_unique_pattern;

DROP INDEX IF EXISTS idx_category_rules_priority;

DROP INDEX IF EXISTS idx_category_rules_user_id;

DROP INDEX IF EXISTS idx_category_rules_user_pattern;

DROP TABLE IF EXISTS category_rules;