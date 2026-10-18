-- IDIOS Engine schema, version 1.
--
-- One CREATE statement per object, grouped by responsibility: ownership (repositories,
-- branches), work (issues, dependencies, pull requests, completions), organisation
-- (projects, labels, milestones), knowledge (concepts, relations) and bookkeeping
-- (settings, activity). Applied in one transaction; see migrations package.

-- Ownership ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS repositories (
 id TEXT PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE UNIQUE,
 description TEXT NOT NULL DEFAULT '', default_branch_id TEXT NOT NULL REFERENCES branches(id) DEFERRABLE INITIALLY DEFERRED,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS branches (
 id TEXT PRIMARY KEY, repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 name TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0 CHECK(is_default IN (0,1)),
 base_branch_id TEXT REFERENCES branches(id) ON DELETE SET NULL,
 forked_from_completion_id TEXT REFERENCES completions(id) ON DELETE SET NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 UNIQUE(repository_id,name)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_one_default_branch_per_repo ON branches(repository_id) WHERE is_default=1;

-- Work ---------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS issues (
 id TEXT PRIMARY KEY, number INTEGER NOT NULL, repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'open'
 CHECK(status IN ('open','in_progress','done','cancelled')),
 priority TEXT NOT NULL DEFAULT '', due_date TEXT, parent_issue_id TEXT REFERENCES issues(id) ON DELETE RESTRICT,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL, closed_at TEXT,
 UNIQUE(repository_id,number), CHECK(parent_issue_id IS NULL OR parent_issue_id<>id)
);

CREATE INDEX IF NOT EXISTS idx_issues_repo_status ON issues(repository_id,status,number);
CREATE INDEX IF NOT EXISTS idx_issues_parent ON issues(parent_issue_id);

CREATE TABLE IF NOT EXISTS issue_dependencies (
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 prerequisite_issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 created_at TEXT NOT NULL, PRIMARY KEY(issue_id,prerequisite_issue_id), CHECK(issue_id<>prerequisite_issue_id)
);

CREATE INDEX IF NOT EXISTS idx_issue_dependencies_prerequisite ON issue_dependencies(prerequisite_issue_id);

CREATE TABLE IF NOT EXISTS pull_requests (
 id TEXT PRIMARY KEY, number INTEGER NOT NULL, repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT, title TEXT NOT NULL, description TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','accepted','closed')),
 target_branch_id TEXT NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
 source_branch_id TEXT REFERENCES branches(id) ON DELETE RESTRICT,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL, accepted_at TEXT,
 UNIQUE(repository_id,number)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_one_open_pr_per_issue ON pull_requests(issue_id) WHERE status='open';
CREATE UNIQUE INDEX IF NOT EXISTS idx_one_accepted_pr_per_issue ON pull_requests(issue_id) WHERE status='accepted';

CREATE TABLE IF NOT EXISTS completions (
 id TEXT PRIMARY KEY, repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 branch_id TEXT NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 pull_request_id TEXT NOT NULL UNIQUE REFERENCES pull_requests(id) ON DELETE RESTRICT,
 summary TEXT NOT NULL, sequence INTEGER NOT NULL, parent_completion_id TEXT REFERENCES completions(id) ON DELETE RESTRICT,
 created_at TEXT NOT NULL, UNIQUE(branch_id,sequence)
);

CREATE INDEX IF NOT EXISTS idx_completions_branch_sequence ON completions(branch_id,sequence);

-- Organisation -------------------------------------------------------------

CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE UNIQUE, description TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS project_issues (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 created_at TEXT NOT NULL, PRIMARY KEY(project_id,issue_id)
);

CREATE TABLE IF NOT EXISTS labels (
 id TEXT PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE, description TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '',
 scope_type TEXT NOT NULL CHECK(scope_type IN ('global','repository')),
 repository_id TEXT REFERENCES repositories(id) ON DELETE CASCADE, created_at TEXT NOT NULL,
 CHECK((scope_type='global' AND repository_id IS NULL) OR (scope_type='repository' AND repository_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_global_label_name ON labels(name COLLATE NOCASE) WHERE scope_type='global';
CREATE UNIQUE INDEX IF NOT EXISTS idx_repo_label_name ON labels(repository_id,name COLLATE NOCASE) WHERE scope_type='repository';

CREATE TABLE IF NOT EXISTS issue_labels (
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 label_id TEXT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
 created_at TEXT NOT NULL, PRIMARY KEY(issue_id,label_id)
);

CREATE TABLE IF NOT EXISTS milestones (
 id TEXT PRIMARY KEY, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
 scope_type TEXT NOT NULL CHECK(scope_type IN ('repository','project')), scope_id TEXT NOT NULL,
 due_date TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_milestone_scope_title ON milestones(scope_type,scope_id,title COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS issue_milestones (
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 milestone_id TEXT NOT NULL REFERENCES milestones(id) ON DELETE CASCADE,
 created_at TEXT NOT NULL, PRIMARY KEY(issue_id,milestone_id)
);

-- Knowledge ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS concepts (
 id TEXT PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE UNIQUE, description TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS relations (
 id TEXT PRIMARY KEY, source_type TEXT NOT NULL, source_id TEXT NOT NULL,
 target_type TEXT NOT NULL, target_id TEXT NOT NULL, relation_type TEXT NOT NULL,
 created_at TEXT NOT NULL, UNIQUE(source_type,source_id,target_type,target_id,relation_type)
);

CREATE INDEX IF NOT EXISTS idx_relations_source ON relations(source_type,source_id);
CREATE INDEX IF NOT EXISTS idx_relations_target ON relations(target_type,target_id);

-- Bookkeeping --------------------------------------------------------------

CREATE TABLE IF NOT EXISTS repository_settings (
 repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
 key TEXT NOT NULL, value TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(repository_id,key)
);

CREATE TABLE IF NOT EXISTS activity_events (
 id TEXT PRIMARY KEY, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
 action TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_activity_entity ON activity_events(entity_type,entity_id,created_at DESC);
