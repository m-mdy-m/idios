-- Explicit local association between a provider-linked repository and an IDIOS Project.
-- This table never changes the upstream provider or IDIOS-native repository records.
CREATE TABLE hub_project_repositories (
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  hub_repository_id TEXT NOT NULL REFERENCES hub_repositories(id) ON DELETE CASCADE,
  relationship TEXT NOT NULL DEFAULT '',
  is_primary INTEGER NOT NULL DEFAULT 0 CHECK(is_primary IN (0,1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(project_id, hub_repository_id)
);
CREATE UNIQUE INDEX idx_hub_project_repositories_one_primary
  ON hub_project_repositories(project_id) WHERE is_primary=1;
CREATE INDEX idx_hub_project_repositories_repository
  ON hub_project_repositories(hub_repository_id, project_id);
CREATE TRIGGER trg_hub_project_repositories_promote_primary
AFTER DELETE ON hub_project_repositories
WHEN OLD.is_primary=1
BEGIN
  UPDATE hub_project_repositories
  SET is_primary=1, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
  WHERE project_id=OLD.project_id
    AND hub_repository_id=(
      SELECT hub_repository_id FROM hub_project_repositories
      WHERE project_id=OLD.project_id
      ORDER BY created_at, hub_repository_id LIMIT 1
    )
    AND NOT EXISTS (
      SELECT 1 FROM hub_project_repositories
      WHERE project_id=OLD.project_id AND is_primary=1
    );
END;
