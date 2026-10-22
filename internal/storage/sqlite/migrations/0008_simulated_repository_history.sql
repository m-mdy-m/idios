-- IDIOS simulated repository history: this is workflow metadata, not a real Git repository.
ALTER TABLE issues ADD COLUMN issue_type TEXT NOT NULL DEFAULT 'task' CHECK(issue_type IN ('task','bug','feature','research'));

CREATE TABLE IF NOT EXISTS pull_request_issues (
 pull_request_id TEXT NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 position INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 PRIMARY KEY(pull_request_id, issue_id)
);
CREATE INDEX IF NOT EXISTS idx_pr_issues_issue ON pull_request_issues(issue_id,pull_request_id);
INSERT OR IGNORE INTO pull_request_issues(pull_request_id,issue_id,position,created_at)
 SELECT id,issue_id,0,created_at FROM pull_requests;
DROP INDEX IF EXISTS idx_one_open_pr_per_issue;
DROP INDEX IF EXISTS idx_one_accepted_pr_per_issue;

ALTER TABLE completions ADD COLUMN commit_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS repository_files (
 id TEXT PRIMARY KEY,
 repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 path TEXT NOT NULL,
 content TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL,
 updated_in_completion_id TEXT REFERENCES completions(id) ON DELETE SET NULL,
 UNIQUE(repository_id,path)
);
CREATE INDEX IF NOT EXISTS idx_repository_files_repo ON repository_files(repository_id,path);

CREATE TABLE IF NOT EXISTS completion_files (
 completion_id TEXT NOT NULL REFERENCES completions(id) ON DELETE CASCADE,
 path TEXT NOT NULL,
 content TEXT NOT NULL,
 PRIMARY KEY(completion_id,path)
);
CREATE TABLE IF NOT EXISTS repository_tags (
 id TEXT PRIMARY KEY,
 repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 message TEXT NOT NULL DEFAULT '',
 completion_sequence INTEGER NOT NULL DEFAULT 0,
 completion_id TEXT REFERENCES completions(id) ON DELETE SET NULL,
 previous_tag_id TEXT REFERENCES repository_tags(id) ON DELETE SET NULL,
 created_at TEXT NOT NULL,
 UNIQUE(repository_id,name)
);
CREATE INDEX IF NOT EXISTS idx_repository_tags_repo_created ON repository_tags(repository_id,created_at DESC);
CREATE TABLE IF NOT EXISTS repository_tag_files (
 tag_id TEXT NOT NULL REFERENCES repository_tags(id) ON DELETE CASCADE,
 path TEXT NOT NULL,
 content TEXT NOT NULL,
 PRIMARY KEY(tag_id,path)
);
CREATE TABLE IF NOT EXISTS repository_tag_history (
 tag_id TEXT NOT NULL REFERENCES repository_tags(id) ON DELETE CASCADE,
 completion_id TEXT NOT NULL REFERENCES completions(id) ON DELETE CASCADE,
 PRIMARY KEY(tag_id,completion_id)
);

-- Existing and future repositories start with a local README. Files are editable virtual files
-- owned by the IDIOS Engine; they never touch the user's disk or invoke Git.
INSERT OR IGNORE INTO repository_files(id,repository_id,path,content,updated_at)
 SELECT lower(hex(randomblob(16))),id,'README.md','# ' || name || char(10) || char(10) || coalesce(description,'') || char(10) || char(10) || '> Managed by IDIOS simulated repository workspace.' ,created_at FROM repositories;
CREATE TRIGGER IF NOT EXISTS trg_repo_seed_readme AFTER INSERT ON repositories
BEGIN
 INSERT OR IGNORE INTO repository_files(id,repository_id,path,content,updated_at)
 VALUES(lower(hex(randomblob(16))),NEW.id,'README.md','# ' || NEW.name || char(10) || char(10) || NEW.description || char(10),NEW.created_at);
END;
