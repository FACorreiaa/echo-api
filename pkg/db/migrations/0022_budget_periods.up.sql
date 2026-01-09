-- +goose Up
-- Migration: 0022_budget_periods
-- Purpose: Add monthly budget versioning with period-specific values and history tracking

-- ============================================================================
-- Budget Periods - Monthly snapshots per plan
-- ============================================================================

CREATE TABLE budget_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    plan_id UUID NOT NULL REFERENCES user_plans (id) ON DELETE CASCADE,
    year INTEGER NOT NULL CHECK (
        year >= 2000
        AND year <= 2100
    ),
    month INTEGER NOT NULL CHECK (
        month >= 1
        AND month <= 12
    ),
    is_locked BOOLEAN DEFAULT false,
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (plan_id, year, month)
);

CREATE INDEX idx_budget_periods_plan ON budget_periods (plan_id);

CREATE INDEX idx_budget_periods_date ON budget_periods (year, month);

-- ============================================================================
-- Budget Period Items - Item values per period (overrides plan_items.budgeted_minor)
-- ============================================================================

CREATE TABLE budget_period_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    period_id UUID NOT NULL REFERENCES budget_periods (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES plan_items (id) ON DELETE CASCADE,
    budgeted_minor BIGINT NOT NULL DEFAULT 0,
    actual_minor BIGINT NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (period_id, item_id)
);

CREATE INDEX idx_budget_period_items_period ON budget_period_items (period_id);

CREATE INDEX idx_budget_period_items_item ON budget_period_items (item_id);

-- ============================================================================
-- Budget History - Audit trail for budget changes
-- ============================================================================

CREATE TABLE budget_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    period_item_id UUID NOT NULL REFERENCES budget_period_items (id) ON DELETE CASCADE,
    old_budgeted_minor BIGINT,
    new_budgeted_minor BIGINT,
    old_actual_minor BIGINT,
    new_actual_minor BIGINT,
    changed_by UUID REFERENCES users (id),
    changed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_budget_history_period_item ON budget_history (period_item_id);

CREATE INDEX idx_budget_history_changed_at ON budget_history (changed_at);

-- ============================================================================
-- Trigger: Auto-create budget period items when period is created
-- ============================================================================

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_period_items_from_plan()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO budget_period_items (period_id, item_id, budgeted_minor, actual_minor)
  SELECT 
    NEW.id,
    pi.id,
    pi.budgeted_minor,
    0
  FROM plan_items pi
  JOIN plan_categories pc ON pi.category_id = pc.id
  JOIN plan_category_groups pcg ON pc.group_id = pcg.id
  WHERE pcg.plan_id = NEW.plan_id;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_create_period_items
  AFTER INSERT ON budget_periods
  FOR EACH ROW
  EXECUTE FUNCTION create_period_items_from_plan();

-- ============================================================================
-- Trigger: Track budget changes in history
-- ============================================================================

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION track_budget_changes()
RETURNS TRIGGER AS $$
BEGIN
  IF OLD.budgeted_minor IS DISTINCT FROM NEW.budgeted_minor 
     OR OLD.actual_minor IS DISTINCT FROM NEW.actual_minor THEN
    INSERT INTO budget_history (
      period_item_id, 
      old_budgeted_minor, 
      new_budgeted_minor,
      old_actual_minor,
      new_actual_minor
    ) VALUES (
      NEW.id,
      OLD.budgeted_minor,
      NEW.budgeted_minor,
      OLD.actual_minor,
      NEW.actual_minor
    );
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_track_budget_history
  AFTER UPDATE ON budget_period_items
  FOR EACH ROW
  EXECUTE FUNCTION track_budget_changes();

-- ============================================================================
-- Update timestamp trigger
-- ============================================================================

CREATE TRIGGER trg_budget_periods_updated
  BEFORE UPDATE ON budget_periods
  FOR EACH ROW
  EXECUTE FUNCTION update_plan_updated_at();

CREATE TRIGGER trg_budget_period_items_updated
  BEFORE UPDATE ON budget_period_items
  FOR EACH ROW
  EXECUTE FUNCTION update_plan_updated_at();