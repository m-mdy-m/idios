export type IssueStatus = 'open' | 'in_progress' | 'done' | 'cancelled'
export type IssueType = 'task' | 'bug' | 'feature' | 'research'

export interface Repository {
  id: string
  name: string
  description: string
  default_branch_id?: string
  created_at: string
  updated_at: string
}

export interface Issue {
  id: string
  number: number
  repository_id: string
  repository_name?: string
  title: string
  description: string
  status: IssueStatus
  type: IssueType
  priority?: string
  due_date?: string
  parent_issue_id?: string
  parent_reference?: string
  branch_id?: string
  branch_name?: string
  blocked: boolean
  blockers?: Array<{ issue_id: string; repository_name: string; number: number; title: string; status: IssueStatus; reason: string }>
  created_at: string
  updated_at: string
  closed_at?: string
}

export interface Project {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface ProjectRepository {
  project_id: string
  repository_id: string
  repository_name: string
  repository_description: string
  relationship: string
  is_primary: boolean
  created_at: string
  updated_at: string
}

export interface ShelfBook {
  id: string
  title: string
  summary?: string
  directory_name: string
  readme_file?: string
  pdf_file?: string
  available: boolean
  reading_status: 'unread' | 'reading' | 'finished' | string
  current_page: number
  total_pages: number
  repository_id?: string
  repository_name?: string
  tags: string[]
  created_at: string
  updated_at: string
}

export interface ShelfReadingSession {
  id: string
  book_id: string
  book_title: string
  started_at: string
  ended_at: string
  duration_seconds: number
  start_page: number
  end_page: number
  created_at: string
}

export interface ShelfState {
  root_path: string
  configured: boolean
  books: ShelfBook[]
}

export interface ShelfAnnotationRect {
  x: number
  y: number
  width: number
  height: number
}

export interface ShelfAnnotation {
  id: string
  book_id: string
  page: number
  quote: string
  note?: string
  color: 'yellow' | 'green' | 'blue' | 'pink' | 'purple' | string
  rectangles: ShelfAnnotationRect[]
  created_at: string
  updated_at: string
}

export interface ShelfBookRelation {
  id: string
  source_book_id: string
  source_book_title: string
  target_book_id: string
  target_book_title: string
  relation_type: 'related_to' | 'prerequisite_for' | 'contrasts_with' | 'implements' | 'references' | 'uses' | string
  created_at: string
}

export interface ShelfAsk {
  id: string
  book_id: string
  book_title: string
  question: string
  context?: string
  reason?: string
  answer?: string
  status: 'open' | 'answered' | string
  created_at: string
  updated_at: string
  answered_at?: string
}

export interface NoteLabel {
  id: string
  name: string
  color: string
}

export interface Note {
  id: string
  title: string
  content: string
  linked_entity_type?: string
  linked_entity_id?: string
  linked_entity_label?: string
  graph_visible: boolean
  labels?: NoteLabel[]
  created_at: string
  updated_at: string
}

export interface Notification {
  id: string
  title: string
  body: string
  level: 'info' | 'warning' | 'error' | string
  related_entity_type?: string
  related_entity_id?: string
  read_at?: string
  created_at: string
}

export interface PlanTask {
  id: string
  plan_id: string
  title: string
  position: number
  done: boolean
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface Plan {
  id: string
  title: string
  description: string
  plan_date: string
  linked_entity_type?: 'issue' | 'repository' | string
  linked_entity_id?: string
  linked_entity_label?: string
  tasks: PlanTask[]
  created_at: string
  updated_at: string
}

export interface PlanReminderConfig {
  enabled: boolean
  desktop_enabled: boolean
  morning_time: string
  midday_time: string
  evening_time: string
}

export interface ActivityEvent {
  id: string
  entity_type: string
  entity_id: string
  action: string
  payload?: string
  created_at: string
}

export interface ActivityDayCount {
  date: string
  event_count: number
}

export interface GraphNode {
  id: string
  type: string
  label: string
  description?: string
  status?: string
  reference?: string
}

export interface GraphEdge {
  source: string
  target: string
  type: string
  label?: string
}

export type GraphLayoutMode = 'organic'
export type GraphNodeShape = 'circle' | 'square' | 'rounded' | 'diamond' | 'hexagon' | 'ellipse' | 'pill' | 'document'

export interface GraphPoint { x: number; y: number }
export interface GraphViewport { x: number; y: number; scale: number }
export interface GraphLayout {
  mode: GraphLayoutMode
  positions: Record<GraphLayoutMode, Record<string, GraphPoint>>
  views: Record<GraphLayoutMode, GraphViewport>
  shapes: Record<string, GraphNodeShape>
}

export interface KnowledgeGraph {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface WorkspaceProfile {
  username: string
  full_name: string
  focus?: string
  topics?: string
  website?: string
  bio: string
  readme: string
  updated_at?: string
}

export interface DashboardMetrics {
  repositories: number
  issues: number
  projects: number
  notes: number
  open_issues: number
  in_progress_issues: number
  blocked_issues: number
  done_issues: number
  cancelled_issues: number
  unread_notifications: number
}

export interface DashboardData {
  profile: WorkspaceProfile
  repositories: Repository[]
  issues: Issue[]
  projects: Project[]
  notes: Note[]
  notifications: Notification[]
  activity: ActivityEvent[]
  metrics: DashboardMetrics
}

export interface CustomWebTheme {
  name: string
  base: 'dark' | 'light'
  background: string
  surface: string
  text: string
  muted: string
  accent: string
  border: string
  success: string
  warning: string
  danger: string
}

export interface AppConfig {
  version: number
  startup: { show_intro: boolean }
  debug: boolean
  appearance: { theme: string; density: string; colors: Record<string, string> }
  themes: Record<string, Record<string, string>>
  notifications: { enabled: boolean; poll_interval_minutes: number; quote_reminders: Record<string, unknown>; plan_reminders: PlanReminderConfig }
  keyboard: Record<string, string>
  REPOSITORY_TEMPLATE: { description: string }
  ISSUE_TEMPLATE: { default: string; templates: Record<string, { name: string; title_prefix: string; body: string }> }
  PULL_REQUEST_TEMPLATE: { title_prefix: string; body: string }
  server: { host: string; port: number; open_browser: boolean; refresh_interval_seconds: number }
  web: { theme: string; custom_themes?: Record<string, CustomWebTheme>; sidebar_collapsed: boolean; compact: boolean }
}

export interface HealthPayload {
  status: 'ok' | string
  version: string
  address: string
  database: string
  started_at: string
  refresh_interval_seconds: number
}

export interface UpdateRelease {
  version: string;
  tag: string;
  url: string;
  notes: string;
}

export interface UpdateCheck {
  current_version: string;
  update_available: boolean;
  release?: UpdateRelease;
}

export interface UpdateStatus {
  status: 'idle' | 'checking' | 'downloading' | 'scheduled' | 'failed' | string;
  error: string;
  version: string;
}

export interface HealthStatus {
  status: string
  version: string
  address: string
  database: string
  started_at: string
  refresh_interval_seconds: number
}

export type GraphData = KnowledgeGraph

export interface ConfigPayload {
  config: AppConfig
  config_path: string
  config_dir: string
  db_path: string
}

export interface PullRequest {
  id: string; number: number; repository_id: string; repository_name?: string;
  issue_id: string; issue_number: number; issue_title?: string; title: string; description: string;
  status: 'open' | 'accepted' | 'closed' | string; target_branch_id: string; target_branch_name?: string;
  source_branch_id?: string; source_branch_name?: string; is_draft?: boolean;
  created_at: string; updated_at: string; accepted_at?: string; issue_ids?: string[];
}
export interface Concept { id: string; name: string; description: string; created_at: string; updated_at: string }
export interface Label { id: string; name: string; description: string; color: string; scope_type: string; repository_id?: string; repository_name?: string; created_at: string }
export interface Milestone { id: string; title: string; description: string; scope_type: string; scope_id: string; scope_name?: string; due_date?: string; created_at: string; updated_at: string }
export interface Quote { id: string; text: string; author?: string; source?: string; source_url?: string; category?: string; created_at: string }
export interface ScheduledJob { id: string; name: string; description?: string; action_type: string; schedule_type: string; interval_minutes?: number; schedule_time?: string; schedule_weekday: number; schedule_day?: number; enabled: boolean; notify: boolean; next_run_at?: string; last_run_at?: string; completed_at?: string; branch_id?: string; branch_name?: string; created_at: string; updated_at: string }
export interface Branch { id: string; repository_id: string; name: string; is_default: boolean; base_branch_id?: string; created_at: string; updated_at: string }
export interface CompletionRecord { id: string; repository_id: string; repository_name?: string; branch_id: string; branch_name?: string; issue_id: string; issue_number: number; pull_request_id: string; summary: string; sequence: number; created_at: string; commit_hash?: string }
export interface RepositoryFile { id: string; repository_id: string; path: string; content: string; updated_at: string; updated_in_completion_id?: string }
export interface RepositoryTag { id: string; repository_id: string; repository_name?: string; name: string; message: string; completion_sequence: number; completion_id?: string; previous_tag_id?: string; previous_tag_name?: string; created_at: string; file_count?: number }
export interface TagFileChange { path: string; status: 'added' | 'modified' | 'deleted' | string; previous_content?: string; content?: string }
export interface RepositoryTagComparison { tag: RepositoryTag; previous_tag: string; changes: TagFileChange[]; history: CompletionRecord[] }
export interface RepositorySetting { key: string; value: string }
export interface WorkspacePin { entity_type: string; entity_id: string; label?: string; color: string; position: number; created_at: string }
export interface ProtocolField { name: string; label: string; type: 'text' | 'textarea' | 'date' | 'datetime' | 'number' | 'select' | 'boolean' | 'color'; required?: boolean; placeholder?: string; help?: string; default?: unknown; options?: string[]; source?: string }
export interface ProtocolCommand { id: string; title: string; category: string; description: string; fields?: ProtocolField[] }
export interface ProtocolCatalog { protocol_version: number; commands: ProtocolCommand[] }
export interface ProtocolOption { label: string; value: string }
export interface FeatureDefinition {
  id: string
  label: string
  group: string
  description: string
  web_path?: string
  icon: string
  web_navigation?: boolean
  web_bottom_navigation?: boolean
  metric?: string
  web_group_order?: number
  web_order?: number
}
export interface FeatureCatalog { catalog_version: number; features: FeatureDefinition[] }

export interface LogPayload { path: string; count: number; lines: string[] }


export interface MonitoringSnapshot {
  status: 'ok' | 'degraded' | string
  observed_at: string
  database: { status: 'ok' | 'error' | string; latency_ms: number }
  process: { uptime_seconds: number; goroutines: number; memory_alloc_bytes: number; heap_in_use_bytes: number }
  http: { requests_total: number; requests_failed: number; average_request_ms: number; last_request_at?: string }
  logs: { path: string; size_bytes: number; modified_at?: string }
}



export interface LocalGitStatus {
  available: boolean;
  path?: string;
  initialized: boolean;
  backend: string;
  branch?: string;
  clean: boolean;
  staged: number;
  unstaged: number;
  untracked: number;
  reason?: string;
}
export interface LocalGitBranch {
  name: string;
  commit_sha: string;
  current: boolean;
}
export interface LocalGitCommit {
  sha: string;
  author: string;
  author_email: string;
  author_date: string;
  committer: string;
  commit_email: string;
  commit_date: string;
  message: string;
}

export type HubProvider = 'github' | 'gitlab' | 'sourceforge';
export interface HubProviderDescriptor { id: HubProvider; label: string; oauth_available?: boolean; capabilities: string[] }
export interface HubStatus {
  credential_storage_ready: boolean;
  credential_storage_error?: string;
  providers: HubProviderDescriptor[];
  connection_count: number;
}
export interface HubConnection {
  id: string;
  provider: HubProvider;
  host: string;
  account_id: string;
  account_name: string;
  display_name: string;
  scopes: string[];
  installation_id?: string;
  status: 'connected' | 'reauthorize' | 'error' | string;
  last_error?: string;
  connected_at: string;
  updated_at: string;
}
export interface HubRepository {
  id: string;
  connection_id: string;
  provider: HubProvider;
  host: string;
  external_id: string;
  namespace: string;
  name: string;
  full_path: string;
  description: string;
  web_url: string;
  clone_url?: string;
  clone_ssh_url?: string;
  default_branch?: string;
  language?: string;
  visibility?: string;
  repository_type: string;
  tracker_mount_point?: string;
  discussion_mount_point?: string;
  permission?: string;
  capabilities: string[];
  open_issue_count: number;
  open_change_request_count: number;
  sync_state: 'never' | 'ok' | 'stale' | 'error' | string;
  last_synced_at?: string;
  last_error?: string;
  linked_at: string;
  updated_at: string;
  local_alias?: string;
  pinned: boolean;
  collection_ids?: string[];
}
export interface HubCollection {
  id: string;
  name: string;
  description?: string;
  position: number;
  repository_count: number;
  created_at: string;
  updated_at: string;
}
export interface HubRepositoryPreferences {
  repository_id: string;
  local_alias?: string;
  private_note?: string;
  pinned: boolean;
  updated_at?: string;
}
export interface HubProjectRepository {
  project_id: string;
  project_name: string;
  repository_id: string;
  repository_name: string;
  full_path: string;
  provider: HubProvider;
  relationship: string;
  is_primary: boolean;
  created_at: string;
  updated_at: string;
}
export interface HubRepositoryRelation {
  id: string;
  source_repository_id: string;
  target_repository_id: string;
  relation_type: 'depends_on' | 'related_to' | 'implements' | 'documentation_for' | string;
  description?: string;
  target_name?: string;
  target_full_path?: string;
  target_provider?: HubProvider;
  created_at: string;
}
export interface HubDiscoveredRepository {
  provider: HubProvider;
  host: string;
  external_id: string;
  namespace: string;
  name: string;
  full_path: string;
  description: string;
  web_url: string;
  clone_url?: string;
  clone_ssh_url?: string;
  default_branch?: string;
  language?: string;
  visibility?: string;
  repository_type: string;
  permission?: string;
  capabilities: string[];
  open_issue_count: number;
  open_change_request_count: number;
  tracker_mount_point?: string;
  discussion_mount_point?: string;
}
export interface HubDiscoveryResult { items: HubDiscoveredRepository[]; page: number }
export interface HubBranch { name: string; commit_sha: string; protected: boolean; default: boolean }
export interface HubTreeEntry { path: string; name: string; type: 'tree' | 'blob' | string; size: number; sha?: string; url?: string }
export interface HubFile { path: string; content: string; sha?: string; last_commit_id?: string; size: number; encoding: string }
export interface HubCommit { sha: string; message: string; author: string; email?: string; committed_at: string; url?: string }
export interface HubIssue {
  id: string; number: number; title: string; body: string; state: string;
  author?: string; assignee?: string; labels?: string[]; private?: boolean;
  discussion_disabled?: boolean; custom_fields?: Record<string, unknown>;
  url?: string; created_at?: string; updated_at?: string;
}
export interface HubChangeRequest {
  id: string; number: number; title: string; body: string; state: string; draft: boolean;
  source_branch: string; target_branch: string; author?: string; url?: string;
  created_at?: string; updated_at?: string;
}
