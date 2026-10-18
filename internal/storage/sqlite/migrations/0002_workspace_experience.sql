-- Workspace experience: editable notes, rotating quotes, scheduling, notifications, pins and profile.
CREATE TABLE IF NOT EXISTS app_settings (
 key TEXT PRIMARY KEY,
 value TEXT NOT NULL,
 updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notes (
 id TEXT PRIMARY KEY,
 title TEXT NOT NULL,
 content TEXT NOT NULL DEFAULT '',
 linked_entity_type TEXT NOT NULL DEFAULT '',
 linked_entity_id TEXT NOT NULL DEFAULT '',
 graph_visible INTEGER NOT NULL DEFAULT 0 CHECK(graph_visible IN (0,1)),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notes_linked_entity ON notes(linked_entity_type,linked_entity_id);
CREATE INDEX IF NOT EXISTS idx_notes_updated ON notes(updated_at DESC);

CREATE TABLE IF NOT EXISTS quotes (
 id TEXT PRIMARY KEY,
 text TEXT NOT NULL,
 author TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 CHECK(length(trim(text)) > 0)
);
CREATE INDEX IF NOT EXISTS idx_quotes_order ON quotes(created_at,id);

CREATE TABLE IF NOT EXISTS scheduled_jobs (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 action_type TEXT NOT NULL DEFAULT 'custom-reminder',
 interval_minutes INTEGER NOT NULL CHECK(interval_minutes >= 1),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
 notify INTEGER NOT NULL DEFAULT 1 CHECK(notify IN (0,1)),
 next_run_at TEXT NOT NULL,
 last_run_at TEXT,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jobs_due ON scheduled_jobs(enabled,next_run_at);

CREATE TABLE IF NOT EXISTS notifications (
 id TEXT PRIMARY KEY,
 title TEXT NOT NULL,
 body TEXT NOT NULL DEFAULT '',
 level TEXT NOT NULL DEFAULT 'info',
 read_at TEXT,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at DESC);

CREATE TABLE IF NOT EXISTS workspace_profile (
 singleton_id TEXT PRIMARY KEY CHECK(singleton_id='default'),
 username TEXT NOT NULL DEFAULT 'workspace-owner',
 full_name TEXT NOT NULL DEFAULT 'Workspace owner',
 bio TEXT NOT NULL DEFAULT '',
 readme TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO workspace_profile(singleton_id,username,full_name,bio,readme,updated_at)
VALUES('default','workspace-owner','Workspace owner','A local-first workspace for projects, learning and ideas.','Welcome to my IDIOS workspace.\n\nI use this space to connect repositories, issues, concepts and the work behind them.',strftime('%Y-%m-%dT%H:%M:%fZ','now'));

CREATE TABLE IF NOT EXISTS workspace_pins (
 entity_type TEXT NOT NULL,
 entity_id TEXT NOT NULL,
 label TEXT NOT NULL DEFAULT '',
 color TEXT NOT NULL DEFAULT 'accent',
 position INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 PRIMARY KEY(entity_type,entity_id)
);
CREATE INDEX IF NOT EXISTS idx_workspace_pins_position ON workspace_pins(position,created_at);

CREATE TABLE IF NOT EXISTS job_runs (
 id TEXT PRIMARY KEY,
 job_id TEXT NOT NULL REFERENCES scheduled_jobs(id) ON DELETE CASCADE,
 status TEXT NOT NULL,
 message TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_job_runs_job ON job_runs(job_id,created_at DESC);
