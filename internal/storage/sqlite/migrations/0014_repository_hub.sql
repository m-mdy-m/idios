-- Repository Hub keeps external provider identity separate from IDIOS-native repositories.
CREATE TABLE hub_connections (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL CHECK(provider IN ('github','gitlab','sourceforge')),
  host TEXT NOT NULL,
  account_id TEXT NOT NULL,
  account_name TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  scopes_json TEXT NOT NULL DEFAULT '[]',
  credential_ciphertext TEXT NOT NULL,
  installation_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'connected' CHECK(status IN ('connected','reauthorize','error')),
  last_error TEXT NOT NULL DEFAULT '',
  connected_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(provider, host, account_id)
);
CREATE INDEX hub_connections_provider_idx ON hub_connections(provider, host, account_name);

CREATE TABLE hub_oauth_states (
  state_hash TEXT PRIMARY KEY,
  provider TEXT NOT NULL CHECK(provider IN ('github','gitlab','sourceforge')),
  host TEXT NOT NULL,
  code_verifier TEXT NOT NULL DEFAULT '',
  redirect_uri TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE INDEX hub_oauth_states_expires_idx ON hub_oauth_states(expires_at);

CREATE TABLE hub_repositories (
  id TEXT PRIMARY KEY,
  connection_id TEXT NOT NULL REFERENCES hub_connections(id) ON DELETE CASCADE,
  provider TEXT NOT NULL CHECK(provider IN ('github','gitlab','sourceforge')),
  host TEXT NOT NULL,
  external_id TEXT NOT NULL,
  namespace TEXT NOT NULL,
  name TEXT NOT NULL,
  full_path TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  web_url TEXT NOT NULL,
  clone_url TEXT NOT NULL DEFAULT '',
  clone_ssh_url TEXT NOT NULL DEFAULT '',
  default_branch TEXT NOT NULL DEFAULT '',
  language TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL DEFAULT '',
  repository_type TEXT NOT NULL DEFAULT 'git',
  permission TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '[]',
  open_issue_count INTEGER NOT NULL DEFAULT 0 CHECK(open_issue_count >= 0),
  open_change_request_count INTEGER NOT NULL DEFAULT 0 CHECK(open_change_request_count >= 0),
  sync_state TEXT NOT NULL DEFAULT 'never' CHECK(sync_state IN ('never','ok','stale','error')),
  last_synced_at TEXT,
  last_error TEXT NOT NULL DEFAULT '',
  linked_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(provider, host, external_id)
);
CREATE INDEX hub_repositories_connection_idx ON hub_repositories(connection_id, namespace, name);
CREATE INDEX hub_repositories_provider_idx ON hub_repositories(provider, host, full_path);

CREATE TABLE hub_repository_preferences (
  repository_id TEXT PRIMARY KEY REFERENCES hub_repositories(id) ON DELETE CASCADE,
  local_alias TEXT NOT NULL DEFAULT '',
  pinned INTEGER NOT NULL DEFAULT 0 CHECK(pinned IN (0,1)),
  updated_at TEXT NOT NULL
);

CREATE TABLE hub_collections (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL COLLATE NOCASE UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE hub_collection_repositories (
  collection_id TEXT NOT NULL REFERENCES hub_collections(id) ON DELETE CASCADE,
  repository_id TEXT NOT NULL REFERENCES hub_repositories(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY(collection_id, repository_id)
);
CREATE INDEX hub_collection_repositories_repo_idx ON hub_collection_repositories(repository_id);

CREATE TABLE hub_repository_relations (
  id TEXT PRIMARY KEY,
  source_repository_id TEXT NOT NULL REFERENCES hub_repositories(id) ON DELETE CASCADE,
  target_repository_id TEXT NOT NULL REFERENCES hub_repositories(id) ON DELETE CASCADE,
  relation_type TEXT NOT NULL CHECK(relation_type IN ('depends_on','related_to','implements','documentation_for')),
  description TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  CHECK(source_repository_id <> target_repository_id),
  UNIQUE(source_repository_id,target_repository_id,relation_type)
);
CREATE INDEX hub_repository_relations_target_idx ON hub_repository_relations(target_repository_id);
