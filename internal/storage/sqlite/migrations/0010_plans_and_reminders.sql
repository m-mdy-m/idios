-- Dated daily plans with task-level completion and once-per-day reminder delivery.
CREATE TABLE IF NOT EXISTS plans (
 id TEXT PRIMARY KEY,
 title TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 plan_date TEXT NOT NULL,
 linked_entity_type TEXT NOT NULL DEFAULT '' CHECK(linked_entity_type IN ('','issue','repository')),
 linked_entity_id TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 CHECK(length(plan_date)=10)
);
CREATE INDEX IF NOT EXISTS idx_plans_date ON plans(plan_date,created_at);
CREATE INDEX IF NOT EXISTS idx_plans_linked_entity ON plans(linked_entity_type,linked_entity_id);

CREATE TABLE IF NOT EXISTS plan_tasks (
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
 title TEXT NOT NULL,
 position INTEGER NOT NULL DEFAULT 0,
 done INTEGER NOT NULL DEFAULT 0 CHECK(done IN (0,1)),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_plan_tasks_plan_position ON plan_tasks(plan_id,position,created_at);

CREATE TABLE IF NOT EXISTS plan_reminder_deliveries (
 plan_date TEXT NOT NULL,
 period TEXT NOT NULL CHECK(period IN ('morning','midday','evening')),
 delivered_at TEXT NOT NULL,
 PRIMARY KEY(plan_date,period)
);

ALTER TABLE notifications ADD COLUMN related_entity_type TEXT NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN related_entity_id TEXT NOT NULL DEFAULT '';
