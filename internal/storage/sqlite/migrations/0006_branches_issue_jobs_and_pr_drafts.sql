-- Branches can hold issue/job context and pull requests can be drafts.
ALTER TABLE issues ADD COLUMN branch_id TEXT REFERENCES branches(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_issues_branch ON issues(branch_id, repository_id, number);
ALTER TABLE scheduled_jobs ADD COLUMN branch_id TEXT REFERENCES branches(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_branch ON scheduled_jobs(branch_id, enabled, next_run_at);
ALTER TABLE pull_requests ADD COLUMN is_draft INTEGER NOT NULL DEFAULT 0 CHECK(is_draft IN (0,1));

ALTER TABLE workspace_profile ADD COLUMN focus TEXT NOT NULL DEFAULT '';
ALTER TABLE workspace_profile ADD COLUMN topics TEXT NOT NULL DEFAULT '';
ALTER TABLE workspace_profile ADD COLUMN website TEXT NOT NULL DEFAULT '';
