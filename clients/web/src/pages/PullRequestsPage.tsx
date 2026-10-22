import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { CheckCircle2, GitPullRequest, Pencil, Plus, Search, XCircle } from 'lucide-react';
import { api } from '@/lib/api';
import type { AppConfig, ProtocolCommand, PullRequest } from '@/types/idios';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/components/ui/primitives';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { formatRelative } from '@/components/ui/IssueTable';
import { useWorkspaceStore } from '@/store/workspace-store';

export function PullRequestsPage() {
  const navigate = useNavigate();
  const [items, setItems] = useState<PullRequest[]>([]);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState('all');
  const [error, setError] = useState('');
  const { load } = useWorkspaceStore();
  async function refresh() { setError(''); try { const [prs, catalog, configPayload] = await Promise.all([api.pullRequests(), api.protocol(), api.config()]); setItems(prs); setCommands(catalog.commands); setConfig(configPayload.config); } catch (err) { setError(err instanceof Error ? err.message : 'Unable to load pull requests.'); } }
  useEffect(() => { void refresh(); }, []);
  const filtered = useMemo(() => items.filter((pr) => (status === 'all' || pr.status === status) && `${pr.title} ${pr.description} ${pr.repository_name} ${pr.issue_title} ${pr.number}`.toLowerCase().includes(query.toLowerCase())), [items, query, status]);
  function run(id: string, values: Record<string, unknown> = {}) { const command = commands.find((item) => item.id === id); if (!command) { setError(`Action ${id} is not in the shared protocol catalog.`); return; } const templateValues = id === 'pull_request.create' ? { title: config?.PULL_REQUEST_TEMPLATE.title_prefix ?? '', description: config?.PULL_REQUEST_TEMPLATE.body ?? '' } : {}; setInitialValues({ ...templateValues, ...values }); setSelectedCommand(command); }
  async function complete() { await Promise.all([refresh(), load(true)]); }
  return <div className="page-stack">
    <PageHeading eyebrow="REVIEW & COMPLETION" title="Pull requests" description="A PR can group several issues. Merging creates one simulated IDIOS commit, completes every linked issue, and adds an entry to the repository history—without using Git." actions={<Button onClick={() => run('pull_request.create')}><Plus size={16} /> New pull request</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="command-browser-toolbar"><label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search PRs, issues, repositories…" /></label><label className="filter-select"><select value={status} onChange={(event) => setStatus(event.target.value)} aria-label="Filter pull request status"><option value="all">All statuses</option><option value="open">Open</option><option value="accepted">Accepted</option><option value="closed">Closed</option></select></label></div>
    <div className="pr-summary-strip"><div><strong>{items.filter((pr) => pr.status === 'open').length}</strong><span>Open for review</span></div><div><strong>{items.filter((pr) => pr.status === 'accepted').length}</strong><span>Accepted</span></div><div><strong>{items.filter((pr) => pr.status === 'closed').length}</strong><span>Closed</span></div></div>
    {filtered.length ? <div className="entity-card-list">{filtered.map((pr) => <Card className="entity-record-card" key={pr.id}><div className="entity-record-icon entity-record-icon--green"><GitPullRequest size={18} /></div><div className="entity-record-main"><div className="entity-record-title"><h2>{pr.title}</h2><Badge tone={pr.is_draft ? 'neutral' : pr.status === 'accepted' ? 'green' : pr.status === 'closed' ? 'neutral' : 'blue'}>{pr.is_draft ? 'Draft' : pr.status}</Badge></div><p>{pr.description || 'No review summary added.'}</p><div className="entity-record-meta"><code>{pr.repository_name || 'Repository'}!{pr.number}</code><span>{(pr.issue_ids?.length ?? 1) > 1 ? `Includes ${pr.issue_ids?.length} issues` : `For ${pr.issue_title || `issue #${pr.issue_number}`}`}</span><span>{pr.source_branch_name || 'unassigned'} → {pr.target_branch_name || 'default'}</span><span>Updated {formatRelative(pr.updated_at)}</span></div></div><div className="entity-record-actions"><Button variant="ghost" onClick={() => navigate(`/issues?issue=${encodeURIComponent(pr.issue_id)}`)}>Open issue</Button>{pr.repository_id && <Button variant="ghost" onClick={() => navigate(`/repositories?repo=${encodeURIComponent(pr.repository_id)}`)}>Repository</Button>}{pr.status === 'open' && <><Button variant="ghost" onClick={() => run('pull_request.edit', { ref: pr.id, title: pr.title, description: pr.description })}><Pencil size={14} /> Edit</Button>{pr.is_draft ? <Button variant="secondary" onClick={() => run('pull_request.mark_ready', { ref: pr.id })}><CheckCircle2 size={14} /> Ready for review</Button> : <Button variant="secondary" onClick={() => run('pull_request.accept', { ref: pr.id })}><CheckCircle2 size={14} /> Accept</Button>}<Button variant="ghost" onClick={() => run('pull_request.close', { ref: pr.id })}><XCircle size={14} /> Close</Button></>}<Button variant="danger" onClick={() => run('pull_request.delete', { ref: pr.id })}>Remove</Button></div></Card>)}</div> : <Card><EmptyState title={query ? 'No matching pull requests' : 'Nothing to review yet'} body={query ? 'Change the search or status filter.' : 'Create a pull request from an issue when the work is ready for review.'} icon={<GitPullRequest size={22} />} /></Card>}
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
  </div>;
}
