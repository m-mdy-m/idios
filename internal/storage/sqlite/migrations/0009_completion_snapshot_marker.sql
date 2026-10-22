-- Distinguish newly captured empty file snapshots from legacy completions that predate
-- simulated repository files. A snapshot marker is required even when a commit deletes every file.
ALTER TABLE completions ADD COLUMN file_snapshot_ready INTEGER NOT NULL DEFAULT 0 CHECK(file_snapshot_ready IN (0,1));
