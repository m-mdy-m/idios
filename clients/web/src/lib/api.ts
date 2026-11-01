import type {
  ActivityEvent,
  ActivityDayCount,
  AppConfig,
  Plan,
  PlanTask,
  ConfigPayload,
  DashboardData,
  GraphData,
  GraphLayout,
  GraphLayoutMode,
  HealthStatus,
  Issue,
  Note,
  Notification,
  Project,
  ProjectRepository,
  Repository,
  Branch, CompletionRecord, Concept, Label, Milestone, PullRequest, ProtocolCatalog, ProtocolOption, LogPayload, Quote, RepositorySetting, RepositoryFile, RepositoryTag, RepositoryTagComparison, ScheduledJob, WorkspacePin, UpdateCheck, UpdateStatus, FeatureCatalog, MonitoringSnapshot, ShelfState, ShelfBook, ShelfBookRelation, ShelfAsk, ShelfAnnotation, ShelfAnnotationRect, ShelfReadingSession,
  HubStatus, HubConnection, HubRepository, HubCollection, HubRepositoryPreferences, HubRepositoryRelation,
  HubDiscoveryResult, HubBranch, HubTreeEntry, HubFile, HubCommit, HubIssue, HubChangeRequest, HubProvider, HubProjectRepository,
} from '@/types/idios';
import { redactLocalPaths } from '@/lib/privacy';

export class APIError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(redactLocalPaths(message));
    this.name = 'APIError';
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const payload = (await response.json()) as { error?: string | { message?: string }; message?: string };
      const detail = typeof payload.error === 'string' ? payload.error : payload.error?.message;
      message = redactLocalPaths(detail || payload.message || message);
    } catch {
      // Keep the useful status message when the server did not return JSON.
    }
    throw new APIError(message, response.status);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

const json = (method: string, body?: unknown): RequestInit => ({
  method,
  ...(body === undefined ? {} : { body: JSON.stringify(body) }),
});


type AIChatRequest = {
  provider_id?: string;
  model?: string;
  task?: string;
  messages: Array<{ role: 'system' | 'user' | 'assistant'; content: string }>;
  temperature?: number;
  max_tokens?: number;
  timeout_seconds?: number;
  reasoning_effort?: 'low' | 'medium' | 'high';
  privacy_sensitive?: boolean;
  response_format?: AIResponseFormat;
};

async function streamAIChat(
  body: AIChatRequest,
  onDelta: (text: string) => void,
  signal?: AbortSignal,
): Promise<{ provider_id: string; model: string; content: string; duration_ms: number }> {
  const startedAt = performance.now();
  const response = await fetch('/api/ai/chat/stream', {
    method: 'POST',
    headers: { Accept: 'text/event-stream', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    ...(signal ? { signal } : {}),
  });
  if (!response.ok) {
    let message = 'AI request failed (' + response.status + ')';
    try {
      const payload = await response.json() as { error?: string | { message?: string }; message?: string };
      const detail = typeof payload.error === 'string' ? payload.error : payload.error?.message;
      message = redactLocalPaths(detail || payload.message || message);
    } catch {
      // Preserve the HTTP status when the server did not return JSON.
    }
    throw new APIError(message, response.status);
  }
  if (!response.body) throw new Error('The AI provider opened a stream without a readable response body.');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let providerID = body.provider_id ?? '';
  let model = body.model ?? '';
  let completed = false;

  const dispatch = (frame: string) => {
    let eventName = '';
    const data: string[] = [];
    for (const line of frame.split(/\r?\n/)) {
      if (line.startsWith('event:')) eventName = line.slice(6).trim();
      else if (line.startsWith('data:')) data.push(line.slice(5).trimStart());
    }
    if (!data.length) return;
    const payloadText = data.join('\n');
    let payload: Record<string, unknown>;
    try { payload = JSON.parse(payloadText) as Record<string, unknown>; }
    catch { throw new Error('The AI stream returned an invalid event.'); }

    if (eventName === 'error' || payload.type === 'error') {
      const detail = typeof payload.message === 'string' ? payload.message : 'The AI provider stopped before completing the translation.';
      const code = typeof payload.code === 'string' ? payload.code : 'stream_failed';
      throw new APIError('[' + code + '] ' + redactLocalPaths(detail), 502);
    }
    if (typeof payload.provider_id === 'string') providerID = payload.provider_id;
    if (typeof payload.model === 'string') model = payload.model;
    if (payload.type === 'delta' && typeof payload.text === 'string' && payload.text) onDelta(payload.text);
    if (eventName === 'done' || payload.type === 'done') completed = true;
  };

  try {
    while (true) {
      const part = await reader.read();
      buffer += decoder.decode(part.value, { stream: !part.done });
      let boundary = buffer.search(/\r?\n\r?\n/);
      while (boundary >= 0) {
        const frame = buffer.slice(0, boundary);
        const separator = buffer.slice(boundary).match(/^\r?\n\r?\n/)?.[0] ?? '\n\n';
        buffer = buffer.slice(boundary + separator.length);
        dispatch(frame);
        boundary = buffer.search(/\r?\n\r?\n/);
      }
      if (part.done) break;
    }
    if (buffer.trim()) dispatch(buffer);
  } finally {
    if (signal?.aborted) {
      try { await reader.cancel(signal.reason); } catch { /* The stream may already be closed. */ }
    }
    reader.releaseLock();
  }
  if (!completed) throw new Error('The AI stream ended before the provider confirmed completion. Any received text is still available for retry.');
  return { provider_id: providerID, model, content: '', duration_ms: Math.round(performance.now() - startedAt) };
}

export type AIExecutionMode = 'cloud-first' | 'local-first' | 'offline-only';
export type AIProviderKind = 'openrouter' | 'openai-compatible' | 'ollama' | 'custom-adapter';
export type AIRuntimeConfig = {
  mode: AIExecutionMode;
  allow_local_fallback: boolean;
  allow_cloud_fallback: boolean;
  default_model: string;
  task_models: Record<string, string>;
  max_concurrent: number;
  max_queued: number;
  workflow_timeout_seconds: number;
  max_agents: number;
  max_discussion_rounds: number;
};
export type AIProviderAdapter = { id: string; name: string; version?: string; description?: string; protocol?: string; built_in: boolean };
export type AIResponseFormat = { type: 'text' | 'json_object' | 'json_schema'; name?: string; strict?: boolean; schema?: Record<string, unknown> };
export type AIHealth = { state: 'unknown' | 'healthy' | 'unavailable' | 'unauthorized' | 'unsupported' | string; checked_at?: string; latency_ms?: number; error_code?: string; message?: string };
export type AIProvider = { id: string; name: string; type: AIProviderKind; base_url: string; adapter_id?: string; default_model?: string; enabled: boolean; timeout_seconds: number; max_concurrent: number; trusted_local: boolean; has_api_key: boolean; local: boolean; health: AIHealth };
export type AIProviderInput = { id?: string; name: string; type: AIProviderKind; base_url: string; adapter_id?: string; adapter_config?: Record<string, unknown>; api_key?: string; default_model?: string; enabled: boolean; timeout_seconds?: number; max_concurrent?: number; trusted_local?: boolean };
export type AIModelPerformance = { measured_at?: string; response_duration_ms?: number; time_to_first_token_ms?: number; load_duration_ms?: number; prompt_tokens?: number; generated_tokens?: number; prompt_tokens_per_second?: number; tokens_per_second?: number };
export type AIModel = { provider_id: string; id: string; name?: string; architecture?: string; family?: string; parameter_size?: string; quantization_level?: string; format?: string; size_bytes?: number; modified_at?: string; metadata?: Record<string, unknown>; capabilities?: Record<string, boolean>; reasoning?: boolean; supported_parameters?: string[]; context_size?: number; loaded?: boolean; discovered_at?: string; availability?: 'unknown' | 'available' | 'unavailable' | 'blocked' | string; availability_message?: string; probe_checked_at?: string; probe_latency_ms?: number; performance?: AIModelPerformance };
export type AIStatus = { mode: AIExecutionMode; allow_local_fallback: boolean; allow_cloud_fallback: boolean; providers: AIProvider[]; models: AIModel[]; default_model?: string; running_tasks: number; queued_tasks: number; last_refresh?: string; resource_limits: AIRuntimeConfig };
export type AIAgent = { id: string; name: string; description?: string; role: string; instructions: string; tasks?: string[]; preferred_model?: string; skill_ids?: string[]; skill_instructions?: string; output_format?: string; max_tokens?: number; enabled: boolean };
export type AIAPIError = { code: string; message: string };
export type AIAgentContribution = { round?: number; agent_id: string; agent_name: string; provider_id?: string; model?: string; local: boolean; status: string; content?: string; source_ids?: string[]; warnings?: string[]; error?: AIAPIError };
export type AITask = { id: string; kind: string; status: 'pending' | 'running' | 'completed' | 'partially_completed' | 'failed' | 'cancelled'; created_at: string; updated_at: string; provider_id?: string; model?: string; result?: { provider_id: string; model: string; content: string; duration_ms: number }; workflow?: { book_id: string; question: string; contributions: AIAgentContribution[]; synthesis?: string; disagreements?: string[]; warnings?: string[] }; error?: AIAPIError };
export type AIDiscussionRequest = { book_id: string; book_title: string; page_start?: number; page_end?: number; question: string; context: Array<{ id: string; book_id?: string; page?: number; start?: number; end?: number; text: string }>; agent_ids?: string[]; parallel?: boolean; rounds?: number; reasoning_effort?: 'low' | 'medium' | 'high'; skip_synthesis?: boolean; previous_contributions?: AIAgentContribution[] };

async function runCommand<T>(command: string, input: Record<string, unknown> = {}): Promise<T> {
  const response = await request<{ result: T }>('/api/protocol/execute', json('POST', { command, input }));
  return response.result;
}

export const api = {
  health: () => request<HealthStatus>('/api/health'),
  checkUpdate: () => request<UpdateCheck>('/api/update/check'),
  updateStatus: () => request<UpdateStatus>('/api/update/status'),
  installUpdate: () => request<{ status: string }>('/api/update/install', json('POST')),
  dashboard: () => request<DashboardData>('/api/dashboard'),
  repositories: () => request<Repository[]>('/api/repositories'),
  hubStatus: () => request<HubStatus>('/api/hub/status'),
  hubConnections: () => request<HubConnection[]>('/api/hub/connections'),
  hubOAuthStart: (provider: HubProvider, host = '') => {
    const params = new URLSearchParams({ provider });
    if (host.trim()) params.set('host', host.trim());
    return request<{ authorization_url: string }>(`/api/hub/oauth/start?${params}`);
  },
  connectHubProvider: (body: { provider: HubProvider; host?: string; access_token: string }) => request<HubConnection>('/api/hub/connections', json('POST', body)),
  deleteHubConnection: (ref: string) => request<{ deleted: boolean }>(`/api/hub/connections/${encodeURIComponent(ref)}`, json('DELETE')),
  discoverHubRepositories: (connectionID: string, q = '', page = 1) => {
    const params = new URLSearchParams({ connection_id: connectionID, page: String(page) });
    if (q.trim()) params.set('q', q.trim());
    return request<HubDiscoveryResult>(`/api/hub/discover?${params}`);
  },
  hubRepositories: (filters: { provider?: HubProvider; q?: string; collection_id?: string; pinned?: boolean } = {}) => {
    const params = new URLSearchParams();
    if (filters.provider) params.set('provider', filters.provider);
    if (filters.q?.trim()) params.set('q', filters.q.trim());
    if (filters.collection_id) params.set('collection_id', filters.collection_id);
    if (filters.pinned) params.set('pinned', 'true');
    return request<HubRepository[]>(`/api/hub/repositories${params.size ? `?${params}` : ''}`);
  },
  importHubRepository: (body: { connection_id: string; external_id: string; full_path: string }) => request<HubRepository>('/api/hub/repositories/import', json('POST', body)),
  unlinkHubRepository: (ref: string) => request<{ unlinked: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}`, json('DELETE')),
  syncHubRepository: (ref: string) => request<HubRepository>(`/api/hub/repositories/${encodeURIComponent(ref)}/sync`, json('POST')),
  hubRepositoryPreferences: (ref: string) => request<HubRepositoryPreferences>(`/api/hub/repositories/${encodeURIComponent(ref)}/preferences`),
  hubRepositoryProjects: (ref: string) => request<HubProjectRepository[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/projects`),
  linkHubRepositoryProject: (ref: string, body: { project_id: string; relationship: string; primary?: boolean }) => request<{ linked: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/projects`, json('POST', body)),
  unlinkHubRepositoryProject: (ref: string, projectID: string) => request<{ unlinked: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/projects/${encodeURIComponent(projectID)}`, json('DELETE')),
  setHubRepositoryProjectPrimary: (ref: string, projectID: string) => request<{ primary: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/projects/${encodeURIComponent(projectID)}/primary`, json('PUT')),
  projectHubRepositories: (projectID: string) => request<HubProjectRepository[]>(`/api/projects/${encodeURIComponent(projectID)}/hub-repositories`),
  saveHubRepositoryPreferences: (ref: string, body: { local_alias: string; private_note: string; pinned: boolean }) => request<HubRepositoryPreferences>(`/api/hub/repositories/${encodeURIComponent(ref)}/preferences`, json('PATCH', body)),
  hubCollections: () => request<HubCollection[]>('/api/hub/collections'),
  createHubCollection: (body: { name: string; description: string; position?: number }) => request<HubCollection>('/api/hub/collections', json('POST', body)),
  updateHubCollection: (ref: string, body: { name: string; description: string; position?: number }) => request<HubCollection>(`/api/hub/collections/${encodeURIComponent(ref)}`, json('PUT', body)),
  deleteHubCollection: (ref: string) => request<{ deleted: boolean }>(`/api/hub/collections/${encodeURIComponent(ref)}`, json('DELETE')),
  addHubCollectionRepository: (collectionID: string, repositoryID: string) => request<{ added: boolean }>(`/api/hub/collections/${encodeURIComponent(collectionID)}/repositories/${encodeURIComponent(repositoryID)}`, json('PUT')),
  removeHubCollectionRepository: (collectionID: string, repositoryID: string) => request<{ removed: boolean }>(`/api/hub/collections/${encodeURIComponent(collectionID)}/repositories/${encodeURIComponent(repositoryID)}`, json('DELETE')),
  hubRepositoryRelations: (ref: string) => request<HubRepositoryRelation[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/relations`),
  createHubRepositoryRelation: (ref: string, body: { target_repository_id: string; relation_type: string; description?: string }) => request<HubRepositoryRelation>(`/api/hub/repositories/${encodeURIComponent(ref)}/relations`, json('POST', body)),
  deleteHubRepositoryRelation: (ref: string) => request<{ deleted: boolean }>(`/api/hub/relations/${encodeURIComponent(ref)}`, json('DELETE')),
  hubBranches: (ref: string) => request<HubBranch[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/branches`),
  hubTree: (ref: string, branch = '') => request<HubTreeEntry[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/tree?${new URLSearchParams(branch ? { ref: branch } : {})}`),
  hubFile: (ref: string, path: string, branch = '') => {
    const params = new URLSearchParams({ path }); if (branch) params.set('ref', branch);
    return request<HubFile>(`/api/hub/repositories/${encodeURIComponent(ref)}/file?${params}`);
  },
  saveHubFile: (ref: string, body: { path: string; branch: string; content: string; commit_message: string; expected_sha?: string; create_only?: boolean }) => request<HubFile>(`/api/hub/repositories/${encodeURIComponent(ref)}/file`, json('PUT', body)),
  hubCommits: (ref: string, branch = '', page = 1) => {
    const params = new URLSearchParams({ page: String(page) }); if (branch) params.set('ref', branch);
    return request<HubCommit[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/commits?${params}`);
  },
  hubIssues: (ref: string, state = 'open', page = 1) => request<HubIssue[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/issues?${new URLSearchParams({ state, page: String(page) })}`),
  createHubIssue: (ref: string, body: {
    title: string; body: string; labels?: string[]; status?: string; assignee?: string;
    private?: boolean; discussion_disabled?: boolean; custom_fields?: Record<string, string>;
  }) => request<HubIssue>(`/api/hub/repositories/${encodeURIComponent(ref)}/issues`, json('POST', body)),
  commentHubIssue: (ref: string, number: number, body: string) => request<{ commented: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/issues/${number}/comments`, json('POST', { body })),
  updateHubIssue: (ref: string, number: number, body: {
    title?: string; body?: string; state?: string; assignee?: string; labels?: string[];
    private?: boolean; discussion_disabled?: boolean; custom_fields?: Record<string, string>;
  }) => request<HubIssue>(`/api/hub/repositories/${encodeURIComponent(ref)}/issues/${number}`, json('PATCH', body)),
  closeHubIssue: (ref: string, number: number) => request<{ closed: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/issues/${number}/close`, json('POST')),
  hubChanges: (ref: string, state = 'open', page = 1) => request<HubChangeRequest[]>(`/api/hub/repositories/${encodeURIComponent(ref)}/changes?${new URLSearchParams({ state, page: String(page) })}`),
  createHubChange: (ref: string, body: { title: string; body: string; source_branch: string; target_branch: string }) => request<HubChangeRequest>(`/api/hub/repositories/${encodeURIComponent(ref)}/changes`, json('POST', body)),
  closeHubChange: (ref: string, number: number) => request<{ closed: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/changes/${number}/close`, json('POST')),
  approveHubChange: (ref: string, number: number) => request<{ approved: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/changes/${number}/approve`, json('POST')),
  mergeHubChange: (ref: string, number: number) => request<{ merged: boolean }>(`/api/hub/repositories/${encodeURIComponent(ref)}/changes/${number}/merge`, json('POST')),

  createRepository: (body: { name: string; description: string; default_branch?: string }) => runCommand<Repository>('repository.create', { ...body, default_branch: body.default_branch || 'main' }),
  issue: (ref: string) => request<Issue>(`/api/issues/${encodeURIComponent(ref)}`),
  issueDependencies: (ref: string) => request<Issue[]>(`/api/issues/${encodeURIComponent(ref)}/dependencies`),
  issues: (filters: { repo?: string; status?: string } = {}) => {
    const query = new URLSearchParams();
    if (filters.repo) query.set('repo', filters.repo);
    if (filters.status && filters.status !== 'all') query.set('status', filters.status);
    return request<Issue[]>(`/api/issues${query.size ? `?${query}` : ''}`);
  },
  createIssue: (body: { repository: string; title: string; description: string; priority: string; issue_type?: string; due_date?: string; parent?: string; branch?: string }) => runCommand<Issue>('issue.create', { repo: body.repository, title: body.title, description: body.description, priority: body.priority, type: body.issue_type ?? 'task', due: body.due_date ?? '', parent: body.parent ?? '', branch: body.branch ?? '' }),
  startIssue: (ref: string) => runCommand<Issue>('issue.start', { ref }),
  cancelIssue: (ref: string) => runCommand<Issue>('issue.cancel', { ref }),
  projects: () => request<Project[]>('/api/projects'),
  createProject: (body: { name: string; description: string }) => runCommand<Project>('project.create', body),
  projectIssues: (ref: string) => request<Issue[]>(`/api/projects/${encodeURIComponent(ref)}/issues`),
  addProjectIssue: (ref: string, issue: string) => runCommand<{ added: boolean }>('project.add_issue', { project: ref, issue }),
  projectRepositories: (ref: string) => request<ProjectRepository[]>(`/api/projects/${encodeURIComponent(ref)}/repositories`),
  linkProjectRepository: (ref: string, body: { repository: string; relationship: string; primary: boolean }) => request<ProjectRepository[]>(`/api/projects/${encodeURIComponent(ref)}/repositories`, json('POST', body)),
  unlinkProjectRepository: (ref: string, repository: string) => request<{ unlinked: boolean }>(`/api/projects/${encodeURIComponent(ref)}/repositories/${encodeURIComponent(repository)}`, json('DELETE')),
  setPrimaryProjectRepository: (ref: string, repository: string) => request<{ primary: boolean }>(`/api/projects/${encodeURIComponent(ref)}/repositories/${encodeURIComponent(repository)}/primary`, json('PUT')),
  graph: (depth = 2, from = '') => {
    const query = new URLSearchParams({ depth: String(depth) });
    if (from) query.set('from', from);
    return request<GraphData>(`/api/graph?${query}`);
  },
  graphLayout: () => request<GraphLayout>('/api/graph/layout'),
  saveGraphLayout: (layout: GraphLayout) => request<GraphLayout>('/api/graph/layout', json('PUT', layout)),
  resetGraphLayout: (mode: GraphLayoutMode) => request<GraphLayout>('/api/graph/layout/reset', json('POST', { mode })),
  plans: (filters: { from?: string; to?: string } = {}) => {
    const query = new URLSearchParams(); if (filters.from) query.set('from', filters.from); if (filters.to) query.set('to', filters.to);
    return request<Plan[]>(`/api/plans${query.size ? `?${query}` : ''}`);
  },
  createPlan: (body: { title: string; description: string; plan_date: string; linked_entity_type?: string; linked_entity_id?: string; tasks: string[] }) => request<Plan>('/api/plans', json('POST', body)),
  updatePlan: (ref: string, body: { title: string; description: string; plan_date: string; linked_entity_type?: string; linked_entity_id?: string }) => request<Plan>(`/api/plans/${encodeURIComponent(ref)}`, json('PUT', body)),
  deletePlan: (ref: string) => request<void>(`/api/plans/${encodeURIComponent(ref)}`, json('DELETE')),
  addPlanTask: (ref: string, title: string) => request<PlanTask>(`/api/plans/${encodeURIComponent(ref)}/tasks`, json('POST', { title })),
  setPlanTaskDone: (ref: string, task: string, done: boolean) => request<PlanTask>(`/api/plans/${encodeURIComponent(ref)}/tasks/${encodeURIComponent(task)}`, json('PATCH', { done })),
  shelf: () => request<ShelfState>('/api/shelf'),
  scanShelf: () => request<ShelfState>('/api/shelf/scan', json('POST', {})),
  setShelfRoot: (root_path: string) => request<ShelfState>('/api/shelf/settings', json('PUT', { root_path })),
  shelfBooks: () => request<ShelfBook[]>('/api/shelf/books'),
  shelfReadingSessions: (bookID = '') => request<ShelfReadingSession[]>(`/api/shelf/reading-sessions${bookID ? `?book_id=${encodeURIComponent(bookID)}` : ''}`),
  createShelfReadingSession: (body: { id?: string; book_id: string; started_at: string; ended_at: string; start_page: number; end_page: number }) => request<ShelfReadingSession>('/api/shelf/reading-sessions', json('POST', body)),
  shelfBook: (ref: string) => request<ShelfBook>(`/api/shelf/books/${encodeURIComponent(ref)}`),
  shelfReadme: (ref: string) => request<{ content: string }>(`/api/shelf/books/${encodeURIComponent(ref)}/readme`),
  shelfPDFURL: (ref: string) => `/api/shelf/books/${encodeURIComponent(ref)}/pdf`,
  shelfAnnotations: (ref: string) => request<ShelfAnnotation[]>(`/api/shelf/books/${encodeURIComponent(ref)}/annotations`),
  createShelfAnnotation: (ref: string, body: { page: number; quote: string; note?: string; color: ShelfAnnotation['color']; rectangles: ShelfAnnotationRect[] }) => request<ShelfAnnotation>(`/api/shelf/books/${encodeURIComponent(ref)}/annotations`, json('POST', body)),
  updateShelfAnnotation: (ref: string, body: { note: string; color: ShelfAnnotation['color'] }) => request<ShelfAnnotation>(`/api/shelf/annotations/${encodeURIComponent(ref)}`, json('PATCH', body)),
  deleteShelfAnnotation: async (ref: string) => { await request<void>(`/api/shelf/annotations/${encodeURIComponent(ref)}`, json('DELETE')); },
  setShelfBookTags: (ref: string, tags: string[]) => request<ShelfBook>(`/api/shelf/books/${encodeURIComponent(ref)}/tags`, json('PUT', { tags })),
  setShelfBookProgress: (ref: string, body: { status: string; current_page: number; total_pages: number }) => request<ShelfBook>(`/api/shelf/books/${encodeURIComponent(ref)}/progress`, json('PATCH', body)),
  shelfBookNotes: (ref: string) => request<Note[]>(`/api/shelf/books/${encodeURIComponent(ref)}/notes`),
  createShelfNote: (ref: string, body: { title: string; content: string; labels?: string[] }) => request<Note>(`/api/shelf/books/${encodeURIComponent(ref)}/notes`, json('POST', body)),
  shelfBookAsks: (ref: string) => request<ShelfAsk[]>(`/api/shelf/books/${encodeURIComponent(ref)}/asks`),
  shelfAsks: (status: 'open' | 'answered' | 'all' = 'open', bookID = '') => { const q = new URLSearchParams(); if (status) q.set('status', status); if (bookID) q.set('book_id', bookID); return request<ShelfAsk[]>(`/api/shelf/asks?${q}`); },
  createShelfAsk: (ref: string, body: { question: string; context?: string; reason?: string }) => request<ShelfAsk>(`/api/shelf/books/${encodeURIComponent(ref)}/asks`, json('POST', body)),
  answerShelfAsk: (ref: string, answer: string) => request<ShelfAsk>(`/api/shelf/asks/${encodeURIComponent(ref)}`, json('PATCH', { answer })),
  shelfRelations: (bookID = '') => request<ShelfBookRelation[]>(`/api/shelf/relations${bookID ? `?book_id=${encodeURIComponent(bookID)}` : ''}`),
  createShelfRelation: (body: { source_book_id: string; target_book_id: string; relation_type: string }) => request<ShelfBookRelation>('/api/shelf/relations', json('POST', body)),
  deleteShelfRelation: async (ref: string) => { await request<void>(`/api/shelf/relations/${encodeURIComponent(ref)}`, json('DELETE')); },
  shelfGraph: () => request<GraphData>('/api/shelf/graph'),
  aiStatus: () => request<AIStatus>('/api/ai/status'),
  aiConfig: () => request<AIRuntimeConfig>('/api/ai/config'),
  updateAIConfig: (cfg: AIRuntimeConfig) => request<AIRuntimeConfig>('/api/ai/config', json('PUT', cfg)),
  aiAdapters: () => request<AIProviderAdapter[]>('/api/ai/adapters'),
  aiProviders: () => request<AIProvider[]>('/api/ai/providers'),
  createAIProvider: (provider: AIProviderInput) => request<AIProvider>('/api/ai/providers', json('POST', { provider })),
  updateAIProvider: (id: string, provider: AIProviderInput, clearAPIKey = false) => request<AIProvider>(`/api/ai/providers/${encodeURIComponent(id)}`, json('PUT', { provider, clear_api_key: clearAPIKey })),
  deleteAIProvider: (id: string) => request<{ deleted: boolean }>(`/api/ai/providers/${encodeURIComponent(id)}`, json('DELETE')),
  testAIProvider: (id: string) => request<AIHealth>(`/api/ai/providers/${encodeURIComponent(id)}/test`, json('POST')),
  aiModels: (providerID = '') => request<{ models: AIModel[]; last_refresh?: string }>(`/api/ai/models${providerID ? `?provider_id=${encodeURIComponent(providerID)}` : ''}`),
  refreshAIModels: (providerID: string) => request<{ models: AIModel[]; provider_id: string; last_refresh?: string }>('/api/ai/models/refresh', json('POST', { provider_id: providerID })),
  testAIModel: (providerID: string, modelID: string) => request<AIModel>('/api/ai/models/test', json('POST', { provider_id: providerID, model_id: modelID })),
  aiChat: (body: { provider_id?: string; model?: string; task?: string; messages: Array<{ role: 'system' | 'user' | 'assistant'; content: string }>; temperature?: number; max_tokens?: number; timeout_seconds?: number; reasoning_effort?: 'low' | 'medium' | 'high'; privacy_sensitive?: boolean; response_format?: AIResponseFormat }) => request<{ provider_id: string; model: string; content: string; duration_ms: number }>('/api/ai/chat', json('POST', body)),
  aiChatStream: streamAIChat,
  createAITask: (body: { provider_id?: string; model?: string; task?: string; messages: Array<{ role: 'system' | 'user' | 'assistant'; content: string }>; temperature?: number; max_tokens?: number; timeout_seconds?: number; privacy_sensitive?: boolean; response_format?: AIResponseFormat }) => request<AITask>('/api/ai/tasks', json('POST', body)),
  aiTasks: () => request<AITask[]>('/api/ai/tasks'),
  aiTask: (id: string) => request<AITask>(`/api/ai/tasks/${encodeURIComponent(id)}`),
  cancelAITask: (id: string) => request<{ status: string }>(`/api/ai/tasks/${encodeURIComponent(id)}/cancel`, json('POST')),
  aiAgents: () => request<AIAgent[]>('/api/ai/agents'),
  saveAIAgent: (agent: AIAgent) => agent.id ? request<AIAgent>(`/api/ai/agents/${encodeURIComponent(agent.id)}`, json('PUT', agent)) : request<AIAgent>('/api/ai/agents', json('POST', agent)),
  deleteAIAgent: (id: string) => request<{ deleted: boolean }>(`/api/ai/agents/${encodeURIComponent(id)}`, json('DELETE')),
  startAIAgentRun: (body: AIDiscussionRequest) => request<AITask>('/api/ai/agents/runs', json('POST', body)),
  aiAgentRun: (id: string) => request<AITask>(`/api/ai/agents/runs/${encodeURIComponent(id)}`),
  cancelAIAgentRun: (id: string) => request<{ status: string }>(`/api/ai/agents/runs/${encodeURIComponent(id)}/cancel`, json('POST')),
  shelfAIModels: (body: { base_url: string; api_key?: string; probe?: boolean }) => request<{ data?: Array<{ id?: string; name?: string; availability?: 'available' | 'unavailable'; latency_ms?: number; error?: string; capabilities?: { reasoning?: boolean } }>; tested?: boolean }>('/api/shelf/ai/models', json('POST', body)),
  shelfAIChat: (body: { base_url: string; api_key?: string; model: string; reasoning_effort?: 'low' | 'medium' | 'high'; messages: Array<{ role: 'system' | 'user' | 'assistant'; content: string }> }) => request<{ id?: string; model?: string; choices?: Array<{ reasoning_content?: string; reasoning_details?: Array<{ type?: string; text?: string; summary?: string; content?: string }>; message?: { content?: string | Array<{ type?: string; text?: string }>; reasoning_content?: string; reasoning?: string; thinking?: string; analysis?: string; reasoning_details?: Array<{ type?: string; text?: string; summary?: string; content?: string }>; refusal?: string } }>; error?: { message?: string } }>('/api/shelf/ai/chat', json('POST', body)),
  createShelfBookRepository: (ref: string, body: { name: string; description: string }) => request<Repository>(`/api/shelf/books/${encodeURIComponent(ref)}/repository`, json('POST', body)),
  linkShelfBookRepository: (ref: string, repository: string) => request<ShelfBook>(`/api/shelf/books/${encodeURIComponent(ref)}/repository`, json('PUT', { repository })),
  notes: () => request<Note[]>('/api/notes'),
  createNote: (body: { title: string; content: string; labels?: string[] }) => runCommand<Note>('note.create', { title: body.title, content: body.content, labels: (body.labels ?? []).join(', ') }),
  deleteNote: async (ref: string) => { await runCommand<{ deleted: boolean }>('note.delete', { id: ref }); },
  notifications: (limit = 50) => request<Notification[]>(`/api/notifications?limit=${Math.min(200, Math.max(1, limit))}`),
  markNotificationsRead: () => runCommand<{ read: boolean }>('notification.read_all'),
  activity: (filters: { type?: string; id?: string; limit?: number } = {}) => { const query = new URLSearchParams(); if (filters.type) query.set('type', filters.type); if (filters.id) query.set('id', filters.id); if (filters.limit) query.set('limit', String(filters.limit)); return request<ActivityEvent[]>(`/api/activity${query.size ? `?${query}` : ''}`); },
  activityDays: (from: string, to: string, timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC') => { const query = new URLSearchParams({ from, to, timezone }); return request<ActivityDayCount[]>(`/api/activity/days?${query}`); },
  recordActivityPresence: (localDate: string) => request<{ status: string }>('/api/activity/presence', json('POST', { local_date: localDate })),
  logs: (limit = 100) => request<LogPayload>(`/api/logs?limit=${Math.min(1000, Math.max(1, limit))}`),
  clearLogs: () => request<void>('/api/logs', json('DELETE')),
  monitoring: () => request<MonitoringSnapshot>('/api/monitoring'),
  config: () => request<ConfigPayload>('/api/config'),
  updateConfig: (cfg: AppConfig) => request<{ status: string; config_path: string; restart_required?: boolean }>('/api/config', json('PUT', { config: cfg })),
  pullRequests: (filters: { repo?: string; status?: string } = {}) => {
    const query = new URLSearchParams(); if (filters.repo) query.set('repo', filters.repo); if (filters.status && filters.status !== 'all') query.set('status', filters.status);
    return request<PullRequest[]>(`/api/pull-requests${query.size ? `?${query}` : ''}`);
  },
  pullRequest: (ref: string) => request<PullRequest>(`/api/pull-requests/${encodeURIComponent(ref)}`),
  pullRequestIssues: (ref: string) => request<Issue[]>(`/api/pull-requests/${encodeURIComponent(ref)}/issues`),
  concepts: () => request<Concept[]>('/api/concepts'),
  concept: (ref: string) => request<Concept>(`/api/concepts/${encodeURIComponent(ref)}`),
  branches: (repo: string) => request<Branch[]>(`/api/branches?repo=${encodeURIComponent(repo)}`),
  repositoryBranches: (ref: string) => request<Branch[]>(`/api/repositories/${encodeURIComponent(ref)}/branches`),
  repositorySettings: (ref: string) => request<RepositorySetting[]>(`/api/repositories/${encodeURIComponent(ref)}/settings`),
  repositoryFiles: (ref: string) => request<RepositoryFile[]>(`/api/repositories/${encodeURIComponent(ref)}/files`),
  saveRepositoryFile: (ref: string, body: { path: string; content: string }) => request<RepositoryFile>(`/api/repositories/${encodeURIComponent(ref)}/files`, json('PUT', body)),
  deleteRepositoryFile: (ref: string, filePath: string) => request<{ deleted: boolean }>(`/api/repositories/${encodeURIComponent(ref)}/files?path=${encodeURIComponent(filePath)}`, json('DELETE')),
  repositoryTags: (ref: string) => request<RepositoryTag[]>(`/api/repositories/${encodeURIComponent(ref)}/tags`),
  createRepositoryTag: (ref: string, body: { name: string; message: string }) => request<RepositoryTag>(`/api/repositories/${encodeURIComponent(ref)}/tags`, json('POST', body)),
  repositoryTag: (ref: string, tag: string) => request<{ tag: RepositoryTag; files: RepositoryFile[] }>(`/api/repositories/${encodeURIComponent(ref)}/tags/${encodeURIComponent(tag)}`),
  compareRepositoryTag: (ref: string, tag: string) => request<RepositoryTagComparison>(`/api/repositories/${encodeURIComponent(ref)}/tags/${encodeURIComponent(tag)}/compare`),
  repositoryTagArchiveURL: (ref: string, tag: string) => `/api/repositories/${encodeURIComponent(ref)}/tags/${encodeURIComponent(tag)}/archive`,
  completions: (repo = '') => request<CompletionRecord[]>(`/api/completions${repo ? `?repo=${encodeURIComponent(repo)}` : ''}`),
  labels: (scope = '') => request<Label[]>(`/api/labels${scope ? `?scope=${encodeURIComponent(scope)}` : ''}`),
  milestones: (scope = '', owner = '') => { const q = new URLSearchParams(); if (scope) q.set('scope', scope); if (owner) q.set('owner', owner); return request<Milestone[]>(`/api/milestones${q.size ? `?${q}` : ''}`); },
  issueLabels: () => request<Record<string, Label[]>>('/api/issue-labels'),
  issueMilestones: () => request<Record<string, Milestone[]>>('/api/issue-milestones'),
  quotes: () => request<Quote[]>('/api/quotes'),
  createQuote: (body: { text: string; author?: string; source?: string; source_url?: string; category?: string }) => runCommand<Quote>('quote.create', body),
  nextQuote: () => request<Quote>('/api/quotes/next'),
  jobs: () => request<ScheduledJob[]>('/api/jobs'),
  profile: () => request<{ username: string; full_name: string; focus?: string; topics?: string; website?: string; bio: string; readme: string; updated_at: string }>('/api/profile'),
  pins: () => request<WorkspacePin[]>('/api/pins'),
  protocol: () => request<ProtocolCatalog>('/api/protocol'),
  features: () => request<FeatureCatalog>('/api/features'),
  protocolOptions: (source: string, entityType = '') => { const query = new URLSearchParams({ source }); if (entityType) query.set('entity_type', entityType); return request<ProtocolOption[]>(`/api/protocol/options?${query}`); },
  executeCommand: (command: string, input: Record<string, unknown> = {}) => request<{ protocol_version: number; command: string; result: unknown }>('/api/protocol/execute', json('POST', { command, input })),
  shutdown: () => request<{ status: string }>('/api/server/shutdown', json('POST')),
};
