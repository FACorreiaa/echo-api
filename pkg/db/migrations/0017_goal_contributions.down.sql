-- +goose Down
ALTER TABLE recurring_subscriptions
DROP COLUMN IF EXISTS occurrence_count;

DROP TABLE IF EXISTS goal_contributions;