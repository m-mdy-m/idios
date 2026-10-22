import type {
  ActivityEvent,
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
  Branch, CompletionRecord, Concept, Label, Milestone, PullRequest, ProtocolCatalog, ProtocolOption, LogPayload, Quote, RepositorySetting, RepositoryFile, RepositoryTag, RepositoryTagComparison, ScheduledJob, WorkspacePin, UpdateCheck, UpdateStatus, FeatureCatalog, MonitoringSnapshot,
} from '@/types/idios';

export class APIError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
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
      const payload = (await response.json()) as { error?: string; message?: string };
      message = payload.error || payload.message || message;
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
  notes: () => request<Note[]>('/api/notes'),
  createNote: (body: { title: string; content: string; labels?: string[] }) => runCommand<Note>('note.create', { title: body.title, content: body.content, labels: (body.labels ?? []).join(', ') }),
  deleteNote: async (ref: string) => { await runCommand<{ deleted: boolean }>('note.delete', { id: ref }); },
  notifications: (limit = 50) => request<Notification[]>(`/api/notifications?limit=${Math.min(200, Math.max(1, limit))}`),
  markNotificationsRead: () => runCommand<{ read: boolean }>('notification.read_all'),
  activity: (filters: { type?: string; id?: string; limit?: number } = {}) => { const query = new URLSearchParams(); if (filters.type) query.set('type', filters.type); if (filters.id) query.set('id', filters.id); if (filters.limit) query.set('limit', String(filters.limit)); return request<ActivityEvent[]>(`/api/activity${query.size ? `?${query}` : ''}`); },
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
