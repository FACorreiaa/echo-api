-- +goose Up
-- Migration: 0020_plan_item_types
-- Description: Add item_type to plan_items to distinguish budgets, recurring, and goals

-- Item type enum for plan items
CREATE TYPE plan_item_type AS ENUM ('budget', 'recurring', 'goal', 'income');

-- Add item_type column to plan_items
ALTER TABLE plan_items
ADD COLUMN item_type plan_item_type DEFAULT 'budget';

-- Add optional link to recurring subscription (for items synced from detected subscriptions)
ALTER TABLE plan_items
ADD COLUMN recurring_subscription_id UUID REFERENCES recurring_subscriptions (id) ON DELETE SET NULL;

-- Add optional link to goal (for savings items linked to goals)
ALTER TABLE plan_items
ADD COLUMN goal_id UUID REFERENCES goals (id) ON DELETE SET NULL;

-- Index for querying by type
CREATE INDEX idx_plan_items_type ON plan_items (plan_id, item_type);

COMMENT ON COLUMN plan_items.item_type IS 'Type of budget item: budget (expense tracking), recurring (subscription), goal (savings target), income';

COMMENT ON COLUMN plan_items.recurring_subscription_id IS 'Links to auto-detected recurring subscription for syncing amounts';

COMMENT ON COLUMN plan_items.goal_id IS 'Links to savings goal for tracking progress';

-- +goose Down
DROP INDEX IF EXISTS idx_plan_items_type;

ALTER TABLE plan_items DROP COLUMN IF EXISTS goal_id;

ALTER TABLE plan_items
DROP COLUMN IF EXISTS recurring_subscription_id;

ALTER TABLE plan_items DROP COLUMN IF EXISTS item_type;

DROP TYPE IF EXISTS plan_item_type;