-- Projects can group multiple repositories and identify the primary implementation.
CREATE TABLE IF NOT EXISTS project_repositories (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 relationship TEXT NOT NULL DEFAULT '',
 is_primary INTEGER NOT NULL DEFAULT 0 CHECK(is_primary IN (0,1)),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(project_id, repository_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_repositories_one_primary
 ON project_repositories(project_id) WHERE is_primary=1;
CREATE INDEX IF NOT EXISTS idx_project_repositories_repository
 ON project_repositories(repository_id, project_id);

-- Keep an associated project usable when its primary repository is removed.
CREATE TRIGGER IF NOT EXISTS trg_project_repositories_promote_primary
AFTER DELETE ON project_repositories
WHEN OLD.is_primary=1
BEGIN
 UPDATE project_repositories SET is_primary=1, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
 WHERE project_id=OLD.project_id
   AND repository_id=(SELECT repository_id FROM project_repositories WHERE project_id=OLD.project_id ORDER BY created_at, repository_id LIMIT 1)
   AND NOT EXISTS (SELECT 1 FROM project_repositories WHERE project_id=OLD.project_id AND is_primary=1);
END;
