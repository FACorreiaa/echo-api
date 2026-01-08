-- +goose Up
-- Migration: 0021_plan_item_configs
-- Description: Add user-configurable item types for dynamic financial primitives

-- Behavior types (internal logic for math)
CREATE TYPE item_behavior AS ENUM (
  'outflow',    -- Reduces surplus (expenses)
  'inflow',     -- Adds to surplus (income)  
  'asset',      -- Tracked on balance sheet (+)
  'liability'   -- Tracked on balance sheet (-)
);

-- Target tabs where items appear in the UI
CREATE TYPE target_tab AS ENUM (
  'budgets',
  'recurring',
  'goals',
  'income',
  'portfolio',
  'liabilities'
);

-- User-configurable item types
CREATE TABLE plan_item_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    label TEXT NOT NULL, -- Display name: "Budget", "Investment"
    short_code TEXT NOT NULL, -- Button code: "B", "I", "D"
    behavior item_behavior NOT NULL, -- Math logic
    target_tab target_tab NOT NULL, -- Which tab shows this type
    color_hex TEXT DEFAULT '#007AFF',
    icon TEXT DEFAULT 'circle',
    is_system BOOLEAN DEFAULT false, -- System defaults can't be deleted
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_user_code UNIQUE (user_id, short_code)
);

-- Indexes
CREATE INDEX idx_plan_item_configs_user ON plan_item_configs (user_id);

CREATE INDEX idx_plan_item_configs_tab ON plan_item_configs (user_id, target_tab);

-- Add config reference to existing plan_items table
ALTER TABLE plan_items
ADD COLUMN config_id UUID REFERENCES plan_item_configs (id) ON DELETE SET NULL;

-- Index for filtering items by config
CREATE INDEX idx_plan_items_config ON plan_items (config_id);

-- Seed system defaults when a new user is created
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION seed_default_item_configs()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO plan_item_configs (user_id, label, short_code, behavior, target_tab, color_hex, icon, is_system, sort_order) VALUES
    (NEW.id, 'Budget', 'B', 'outflow', 'budgets', '#22c55e', 'wallet', true, 1),
    (NEW.id, 'Recurring', 'R', 'outflow', 'recurring', '#f59e0b', 'repeat', true, 2),
    (NEW.id, 'Savings Goal', 'S', 'outflow', 'goals', '#6366f1', 'target', true, 3),
    (NEW.id, 'Income', 'IN', 'inflow', 'income', '#14b8a6', 'trending-up', true, 4),
    (NEW.id, 'Investment', 'I', 'asset', 'portfolio', '#8b5cf6', 'bar-chart-2', true, 5),
    (NEW.id, 'Debt', 'D', 'liability', 'liabilities', '#ef4444', 'credit-card', true, 6);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_seed_item_configs
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION seed_default_item_configs();

-- Seed configs for existing users
INSERT INTO plan_item_configs (user_id, label, short_code, behavior, target_tab, color_hex, icon, is_system, sort_order)
SELECT 
  u.id,
  unnest(ARRAY['Budget', 'Recurring', 'Savings Goal', 'Income', 'Investment', 'Debt']),
  unnest(ARRAY['B', 'R', 'S', 'IN', 'I', 'D']),
  unnest(ARRAY['outflow', 'outflow', 'outflow', 'inflow', 'asset', 'liability']::item_behavior[]),
  unnest(ARRAY['budgets', 'recurring', 'goals', 'income', 'portfolio', 'liabilities']::target_tab[]),
  unnest(ARRAY['#22c55e', '#f59e0b', '#6366f1', '#14b8a6', '#8b5cf6', '#ef4444']),
  unnest(ARRAY['wallet', 'repeat', 'target', 'trending-up', 'bar-chart-2', 'credit-card']),
  true,
  unnest(ARRAY[1, 2, 3, 4, 5, 6])
FROM users u
WHERE NOT EXISTS (
  SELECT 1 FROM plan_item_configs WHERE user_id = u.id
);

-- Comments
COMMENT ON
TABLE plan_item_configs IS 'User-configurable item types for dynamic financial primitives';

COMMENT ON COLUMN plan_item_configs.behavior IS 'Mathematical behavior: outflow/inflow affects surplus, asset/liability affects net worth';

COMMENT ON COLUMN plan_item_configs.target_tab IS 'Which UI tab displays items of this type';

COMMENT ON COLUMN plan_item_configs.is_system IS 'System defaults cannot be deleted by user';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_seed_item_configs ON users;

DROP FUNCTION IF EXISTS seed_default_item_configs ();

ALTER TABLE plan_items DROP COLUMN IF EXISTS config_id;

DROP TABLE IF EXISTS plan_item_configs;

DROP TYPE IF EXISTS target_tab;

DROP TYPE IF EXISTS item_behavior;