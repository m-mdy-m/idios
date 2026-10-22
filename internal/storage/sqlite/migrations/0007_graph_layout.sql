-- Persist graph mode, per-mode node coordinates and camera state across Engine restarts.
CREATE TABLE graph_layout (
    scope TEXT PRIMARY KEY NOT NULL CHECK (scope = 'workspace'),
    layout_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
