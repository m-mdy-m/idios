import { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle, Archive, Bell, BookOpen, BrainCircuit, CalendarClock, CheckSquare,
  FolderKanban, GitPullRequest, Layers3, Quote, RefreshCw, Search, Tags, Trash2,
} from 'lucide-react';
import { api } from '@/lib/api';
import type {
  Concept, Issue, Label, Milestone, Note, Notification, Project, ProtocolCommand,
  PullRequest, Quote as QuoteType, Repository, ScheduledJob,
} from '@/types/idios';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/components/ui/primitives';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { useWorkspaceStore } from '@/store/workspace-store';

type RemovalKind = 'repository' | 'issue' | 'pull_request' | 'project' | 'concept' | 'note' | 'quote' | 'job' | 'label' | 'milestone' | 'notification';
type RemovalCandidate = {
  id: string; kind: RemovalKind; label: string; detail: string;
  command: string; input: Record<string, unknown>;
};

const categories: Array<{ kind: RemovalKind; label: string; icon: typeof Archive; description: string }> = [
  { kind: 'repository', label: 'Repositories', icon: Archive, description: 'Projects, issues, PRs, branches, and repository settings.' },
  { kind: 'issue', label: 'Issues', icon: CheckSquare, description: 'Individual work items and their relationships.' },
  { kind: 'pull_request', label: 'Pull requests', icon: GitPullRequest, description: 'Local review and acceptance records.' },
  { kind: 'project', label: 'Projects', icon: FolderKanban, description: 'Groups of related Issues.' },
  { kind: 'concept', label: 'Concepts', icon: BrainCircuit, description: 'Reusable knowledge graph nodes.' },
  { kind: 'note', label: 'Notes', icon: BookOpen, description: 'Captured notes and their links.' },
  { kind: 'quote', label: 'Quotes', icon: Quote, description: 'Saved quotations and source details.' },
  { kind: 'job', label: 'Scheduled jobs', icon: CalendarClock, description: 'Reminder schedules and their run history.' },
  { kind: 'label', label: 'Labels', icon: Tags, description: 'Workspace and repository labels.' },
  { kind: 'milestone', label: 'Milestones', icon: Layers3, description: 'Repository- and project-scoped milestones.' },
  { kind: 'notification', label: 'Notifications', icon: Bell, description: 'Local inbox entries.' },
];

function candidatesFor(kind: RemovalKind, data: {
  repositories: Repository[]; issues: Issue[]; pullRequests: PullRequest[]; projects: Project[];
  concepts: Concept[]; notes: Note[]; quotes: QuoteType[]; jobs: ScheduledJob[];
  labels: Label[]; milestones: Milestone[]; notifications: Notification[];
}): RemovalCandidate[] {
  const make = (id: string, label: string, detail: string, command: string, inputKey: 'id' | 'ref' = 'ref'): RemovalCandidate => ({
    id, kind, label, detail, command, input: { [inputKey]: id },
  });
  switch (kind) {
    case 'repository': return data.repositories.map((item) => make(item.id, item.name, item.description || 'No description', 'repository.delete'));
    case 'issue': return data.issues.map((item) => make(item.id, `${item.repository_name || 'Repository'}#${item.number} · ${item.title}`, `${item.status.replace('_', ' ')} · priority ${item.priority || 'normal'}${item.blocked ? ' · BLOCKED' : ''}`, 'issue.delete'));
    case 'pull_request': return data.pullRequests.map((item) => make(item.id, `${item.repository_name || 'Repository'}!${item.number} · ${item.title}`, `${item.status} · for ${item.repository_name || 'repository'}#${item.issue_number}`, 'pull_request.delete'));
    case 'project': return data.projects.map((item) => make(item.id, item.name, item.description || 'No description', 'project.delete'));
    case 'concept': return data.concepts.map((item) => make(item.id, item.name, item.description || 'No description', 'concept.delete'));
    case 'note': return data.notes.map((item) => make(item.id, item.title, item.linked_entity_label || 'Not linked to an item', 'note.delete', 'id'));
    case 'quote': return data.quotes.map((item) => make(item.id, `“${item.text.length > 100 ? `${item.text.slice(0, 97)}…` : item.text}”`, [item.author, item.source].filter(Boolean).join(' · ') || 'No attribution', 'quote.delete', 'id'));
    case 'job': return data.jobs.map((item) => make(item.id, item.name, `${item.completed_at ? 'completed' : item.enabled ? 'enabled' : 'paused'} · ${item.schedule_type}${item.next_run_at ? ` · next ${new Date(item.next_run_at).toLocaleString()}` : ''}`, 'job.delete', 'id'));
    case 'label': return data.labels.map((item) => make(item.id, item.name, `${item.scope_type}${item.repository_name ? ` · ${item.repository_name}` : ''}${item.description ? ` · ${item.description}` : ''}`, 'label.delete'));
    case 'milestone': return data.milestones.map((item) => make(item.id, item.title, `${item.scope_type}${item.scope_name ? ` · ${item.scope_name}` : ''}${item.due_date ? ` · due ${item.due_date}` : ''}`, 'milestone.delete'));
    case 'notification': return data.notifications.map((item) => make(item.id, item.title, `${item.read_at ? 'read' : 'unread'} · ${new Date(item.created_at).toLocaleString()}${item.body ? ` · ${item.body}` : ''}`, 'notification.delete', 'id'));
  }
}

export function RemovePage() {
  const [data, setData] = useState<Parameters<typeof candidatesFor>[1] | null>(null);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [kind, setKind] = useState<RemovalKind | null>(null);
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<RemovalCandidate | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const { load } = useWorkspaceStore();

  async function refresh() {
    setLoading(true); setError('');
    try {
      const [repositories, issues, pullRequests, projects, concepts, notes, quotes, jobs, labels, milestones, notifications, catalog] = await Promise.all([
        api.repositories(), api.issues(), api.pullRequests(), api.projects(), api.concepts(), api.notes(), api.quotes(), api.jobs(), api.labels(), api.milestones(), api.notifications(), api.protocol(),
      ]);
      setData({ repositories, issues, pullRequests, projects, concepts, notes, quotes, jobs, labels, milestones, notifications });
      setCommands(catalog.commands);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to load removable workspace items.'); }
    finally { setLoading(false); }
  }
  useEffect(() => { void refresh(); }, []);

  const items = useMemo(() => {
    if (!kind || !data) return [];
    return candidatesFor(kind, data).filter((item) => `${item.label} ${item.detail}`.toLowerCase().includes(query.toLowerCase()));
  }, [data, kind, query]);
  const counts = useMemo(() => {
    const result = {} as Record<RemovalKind, number>;
    for (const category of categories) result[category.kind] = data ? candidatesFor(category.kind, data).length : 0;
    return result;
  }, [data]);

  function remove(item: RemovalCandidate) {
    const command = commands.find((candidate) => candidate.id === item.command);
    if (!command) { setError(`The shared protocol does not expose ${item.command}.`); return; }
    setSelected({ ...item, input: item.kind === 'repository' ? { ...item.input, __confirm_target: item.label } : item.input });
  }
  async function complete() {
    setSelected(null);
    await Promise.all([refresh(), load(true)]);
  }

  return <div className="page-stack">
    <PageHeading eyebrow="WORKSPACE MAINTENANCE" title="Remove items" description="One place to inspect and remove records. Every destructive action goes through the shared Go protocol and its workflow checks." actions={<Button variant="secondary" onClick={() => void refresh()} disabled={loading}><RefreshCw size={15} className={loading ? 'spin' : ''} /> Refresh</Button>} />
    <div className="remove-warning"><span><AlertTriangle size={18} /></span><div><strong>Deletion is permanent</strong><p>Repository deletion removes its issues, pull requests, completions, branches, labels, milestones and settings. The Engine will reject deletion when protected relationships exist.</p></div></div>
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    {!kind ? <div className="remove-categories-grid">{categories.map((category) => <button type="button" className="remove-category-card" key={category.kind} onClick={() => { setKind(category.kind); setQuery(''); }}><span className="remove-category-card__icon"><category.icon size={18} /></span><span className="remove-category-card__copy"><strong>{category.label}</strong><small>{category.description}</small></span><span className="remove-category-card__count">{counts[category.kind]}</span></button>)}</div>
      : <Card className="remove-items-panel"><div className="remove-items-header"><div><button className="text-back-link" onClick={() => { setKind(null); setQuery(''); }}>← All categories</button><h2>{categories.find((category) => category.kind === kind)?.label}</h2><p>{items.length} matching record{items.length === 1 ? '' : 's'} · Choose a target, then review the exact action.</p></div><Badge>{counts[kind]} total</Badge></div><label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter by name or details…" /></label>
        {loading && !data ? <div className="loading-state"><RefreshCw size={16} className="spin" /> Loading workspace records…</div> : items.length ? <div className="remove-items-list">{items.map((item) => <div className="remove-item-row" key={`${item.kind}:${item.id}`}><div className="remove-item-row__icon"><Trash2 size={15} /></div><div className="remove-item-row__body"><strong>{item.label}</strong><small>{item.detail}</small></div><Button variant="danger" onClick={() => remove(item)}><Trash2 size={14} /> Remove</Button></div>)}</div> : <EmptyState title="Nothing to remove here" body={query ? 'Try a different search phrase.' : 'This category is empty.'} icon={<Trash2 size={21} />} />}
      </Card>}
    <CommandRunner command={commands.find((command) => command.id === selected?.command) ?? null} open={Boolean(selected)} initialValues={selected?.input} onClose={() => setSelected(null)} onSuccess={complete} />
  </div>;
}
