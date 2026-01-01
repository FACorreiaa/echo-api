-- +goose Up
-- Merchants table for normalized merchant names
-- Part of the "Clean Room" feature for Echo OS

CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    user_id UUID REFERENCES users (id) ON DELETE CASCADE, -- NULL = global/default merchants
    raw_pattern TEXT NOT NULL, -- Original pattern, e.g., 'APPLE.C', 'COMPRAS C.DEB'
    clean_name TEXT NOT NULL, -- Cleaned name, e.g., 'Apple Services'
    logo_url TEXT, -- Optional merchant logo
    default_category_id UUID REFERENCES categories (id) ON DELETE SET NULL,
    is_system BOOLEAN DEFAULT false, -- System-provided vs user-created
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW(),
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT NOW()
);

-- Index for pattern matching during categorization
CREATE INDEX IF NOT EXISTS idx_merchants_pattern ON merchants (raw_pattern);

-- Index for user-specific merchants
CREATE INDEX IF NOT EXISTS idx_merchants_user_id ON merchants (user_id)
WHERE
    user_id IS NOT NULL;

-- Index for system merchants (global defaults)
CREATE INDEX IF NOT EXISTS idx_merchants_system ON merchants (is_system)
WHERE
    is_system = true;

-- +goose StatementBegin
-- Trigger for updated_at
CREATE OR REPLACE FUNCTION update_merchants_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trigger_merchants_updated_at ON merchants;

CREATE TRIGGER trigger_merchants_updated_at
    BEFORE UPDATE ON merchants
    FOR EACH ROW
    EXECUTE FUNCTION update_merchants_updated_at();

-- Seed some common merchants as system defaults
INSERT INTO
    merchants (
        raw_pattern,
        clean_name,
        is_system
    )
VALUES ('NETFLIX', 'Netflix', true),
    ('SPOTIFY', 'Spotify', true),
    ('AMAZON', 'Amazon', true),
    ('UBER', 'Uber', true),
    ('LYFT', 'Lyft', true),
    (
        'STARBUCKS',
        'Starbucks',
        true
    ),
    (
        'MCDONALDS',
        'McDonald''s',
        true
    ),
    ('APPLE.COM', 'Apple', true),
    ('GOOGLE', 'Google', true),
    ('PAYPAL', 'PayPal', true) ON CONFLICT DO NOTHING;

-- +goose Down
DROP TRIGGER IF EXISTS trigger_merchants_updated_at ON merchants;

DROP FUNCTION IF EXISTS update_merchants_updated_at ();

DROP TABLE IF EXISTS merchants;