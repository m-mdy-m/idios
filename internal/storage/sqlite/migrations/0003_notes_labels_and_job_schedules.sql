-- Notes can reuse several named labels; jobs support interval and calendar schedules.
ALTER TABLE scheduled_jobs ADD COLUMN schedule_type TEXT NOT NULL DEFAULT 'interval'
    CHECK(schedule_type IN ('interval','once','daily','weekly','monthly'));
ALTER TABLE scheduled_jobs ADD COLUMN schedule_time TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduled_jobs ADD COLUMN schedule_weekday INTEGER NOT NULL DEFAULT 0
    CHECK(schedule_weekday BETWEEN 0 AND 6);
ALTER TABLE scheduled_jobs ADD COLUMN schedule_day INTEGER NOT NULL DEFAULT 0
    CHECK(schedule_day BETWEEN 0 AND 31);
ALTER TABLE scheduled_jobs ADD COLUMN completed_at TEXT;

CREATE TABLE IF NOT EXISTS note_labels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    color TEXT NOT NULL DEFAULT 'accent',
    created_at TEXT NOT NULL,
    CHECK(length(trim(name)) > 0)
);
CREATE INDEX IF NOT EXISTS idx_note_labels_name ON note_labels(name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS note_label_assignments (
    note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    label_id TEXT NOT NULL REFERENCES note_labels(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    PRIMARY KEY(note_id, label_id)
);
CREATE INDEX IF NOT EXISTS idx_note_label_assignments_label ON note_label_assignments(label_id, note_id);
