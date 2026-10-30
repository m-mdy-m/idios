import { useEffect, useMemo, useState } from 'react';
import { CheckSquare, Filter, Plus, Search, SlidersHorizontal, ListPlus } from 'lucide-react';
import { useNavigate, useSearchParams } from 'react-router';
import { api } from '@/lib/api';
import { useWorkspaceStore } from '@/store/workspace-store';
import { Badge, Button, Card, PageHeading } from '@/design-system';
import { IssueTable } from '@/features/issues';
import { IssueDetails } from '@/features/issues';
import { IssueCreateDialog } from '@/features/issues';
import { CommandRunner } from '@/features/commands';
import type { AppConfig, Issue, Label, Milestone, ProtocolCommand } from '@/types/idios';

export function IssuesPage() {
  const { dashboard, load } = useWorkspaceStore();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState('all');
  const [repository, setRepository] = useState('all');
  const [labelFilter, setLabelFilter] = useState('all');
  const [milestoneFilter, setMilestoneFilter] = useState('all');
  const [groupBy, setGroupBy] = useState<'repository' | 'status' | 'none'>('repository');
  const [labelsByIssue, setLabelsByIssue] = useState<Record<string, Label[]>>({});
  const [milestonesByIssue, setMilestonesByIssue] = useState<Record<string, Milestone[]>>({});
  const [availableLabels, setAvailableLabels] = useState<Label[]>([]);
  const [availableMilestones, setAvailableMilestones] = useState<Milestone[]>([]);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState('');
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);
  const [bulkCreateOpen, setBulkCreateOpen] = useState(false);

  useEffect(() => {
    let active = true;
    void Promise.all([api.config(), api.protocol(), api.issueLabels(), api.issueMilestones(), api.labels(), api.milestones()]).then(([configPayload, catalog, labels, milestones, available, allMilestones]) => {
      if (!active) return;
      setConfig(configPayload.config);
      setCommands(catalog.commands);
      setLabelsByIssue(labels);
      setMilestonesByIssue(milestones);
      setAvailableLabels(available);
      setAvailableMilestones(allMilestones);
    }).catch((err: unknown) => {
      if (active) setError(err instanceof Error ? err.message : 'Unable to load issue templates and the shared action catalog.');
    });
    return () => { active = false; };
  }, []);


  useEffect(() => {
    if (searchParams.get('bulk') !== '1' || !commands.some((item) => item.id === 'issue.bulk_create')) return;
    setBulkCreateOpen(true);
    const next = new URLSearchParams(searchParams);
    next.delete('bulk');
    setSearchParams(next, { replace: true });
  }, [searchParams, commands, setSearchParams]);

  useEffect(() => {
    const target = searchParams.get('issue');
    if (!target || !dashboard?.issues.length) return;
    const match = dashboard.issues.find((item) => item.id === target || `${item.repository_name}#${item.number}` === target);
    if (match) {
      setSelectedIssue(match);
      const next = new URLSearchParams(searchParams);
      next.delete('issue');
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, dashboard?.issues, setSearchParams]);

  const filtered = useMemo(() => (dashboard?.issues ?? []).filter((issue) => {
    const matchesText = `${issue.title} ${issue.description} ${issue.repository_name} ${issue.number} ${issue.branch_name ?? ''}`.toLowerCase().includes(query.toLowerCase());
    const matchesLabel = labelFilter === 'all' || (labelsByIssue[issue.id] ?? []).some((item) => item.id === labelFilter);
    const matchesMilestone = milestoneFilter === 'all' || (milestonesByIssue[issue.id] ?? []).some((item) => item.id === milestoneFilter);
    return matchesText && (status === 'all' || issue.status === status) && (repository === 'all' || issue.repository_id === repository) && matchesLabel && matchesMilestone;
  }), [dashboard, query, status, repository, labelFilter, milestoneFilter, labelsByIssue, milestonesByIssue]);

  const milestoneOptions = useMemo(() => [...availableMilestones].sort((a, b) => a.title.localeCompare(b.title)), [availableMilestones]);
  const groupedIssues = useMemo(() => {
    if (groupBy === 'none') return [{ key: 'all', title: 'All matching issues', items: filtered }];
    const groups = new Map<string, Issue[]>();
    for (const issue of filtered) {
      const key = groupBy === 'repository' ? issue.repository_name || 'Repository' : issue.status;
      groups.set(key, [...(groups.get(key) ?? []), issue]);
    }
    const order = groupBy === 'status' ? ['open', 'in_progress', 'done', 'cancelled'] : [];
    return [...groups.entries()].sort(([a], [b]) => order.length ? order.indexOf(a) - order.indexOf(b) : a.localeCompare(b)).map(([key, items]) => ({ key, title: groupBy === 'status' ? ({ open: 'Open', in_progress: 'In progress', done: 'Closed / done', cancelled: 'Cancelled' } as Record<string, string>)[key] ?? key : key, items }));
  }, [filtered, groupBy]);

  async function afterIssueChange() { await load(true); }

  return <div className="page-stack">
    <PageHeading eyebrow="WORK TRACKER" title="Issues" description="Keep the next action visible. The detail view brings workflow, dependencies, labels, milestones and knowledge links together—using the same Go protocol and application rules as the CLI." actions={<><Button variant="secondary" onClick={() => setBulkCreateOpen(true)} disabled={!commands.some((item) => item.id === 'issue.bulk_create')}><ListPlus size={16} /> Bulk create</Button><Button onClick={() => setOpen(true)} disabled={!dashboard?.repositories.length}><Plus size={16} /> New issue</Button></>} />
    {error && !open && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="issue-summary-row"><div className="issue-summary-copy"><span className="section-symbol"><CheckSquare size={17} /></span><div><strong>{filtered.length} issue{filtered.length === 1 ? '' : 's'}</strong><p>{dashboard?.metrics.in_progress_issues ?? 0} in progress · {dashboard?.metrics.blocked_issues ?? 0} blocked · {dashboard?.metrics.done_issues ?? 0} completed</p></div></div><div className="filter-tags"><Badge tone="amber">{dashboard?.metrics.open_issues ?? 0} open</Badge><Badge tone="blue">{dashboard?.metrics.in_progress_issues ?? 0} active</Badge></div></div>
    <Card className="list-panel issues-browser-panel">
      <div className="filter-bar issues-filter-bar">
        <label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search issues, branch, repository…" /></label>
        <label className="filter-select"><SlidersHorizontal size={14} /><select value={status} onChange={(event) => setStatus(event.target.value)} aria-label="Filter by status"><option value="all">All statuses</option><option value="open">Open</option><option value="in_progress">In progress</option><option value="done">Done / closed</option><option value="cancelled">Cancelled</option></select></label>
        <label className="filter-select"><Filter size={14} /><select value={repository} onChange={(event) => setRepository(event.target.value)} aria-label="Filter by repository"><option value="all">All repositories</option>{(dashboard?.repositories ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
        <label className="filter-select"><select value={labelFilter} onChange={(event) => setLabelFilter(event.target.value)} aria-label="Filter by label"><option value="all">All labels</option>{availableLabels.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
        <label className="filter-select"><select value={milestoneFilter} onChange={(event) => setMilestoneFilter(event.target.value)} aria-label="Filter by milestone"><option value="all">All milestones</option>{milestoneOptions.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></label>
        <label className="filter-select"><select value={groupBy} onChange={(event) => setGroupBy(event.target.value as 'repository' | 'status' | 'none')} aria-label="Group issues by"><option value="repository">Group by repository</option><option value="status">Group by status</option><option value="none">No grouping</option></select></label>
        {(query || status !== 'all' || repository !== 'all' || labelFilter !== 'all' || milestoneFilter !== 'all') && <Button variant="ghost" onClick={() => { setQuery(''); setStatus('all'); setRepository('all'); setLabelFilter('all'); setMilestoneFilter('all'); }}>Clear filters</Button>}
      </div>
      <div className="issue-grouped-results">{groupedIssues.length ? groupedIssues.map((group) => <section className="issue-group-section" key={group.key}><div className="issue-group-heading"><h3>{group.title}</h3><span>{group.items.length}</span></div><IssueTable issues={group.items} onSelect={setSelectedIssue} emptyTitle="No matching issues" emptyBody="Try another search or reset the filters." /></section>) : <IssueTable issues={[]} onSelect={setSelectedIssue} emptyTitle="No issues match these filters" emptyBody="Try another search or reset the filters to see more of your workspace." />}</div>
    </Card>

    <IssueCreateDialog open={open} repositories={dashboard?.repositories ?? []} initialRepositoryId={dashboard?.repositories[0]?.id} onClose={() => setOpen(false)} onCreated={afterIssueChange} />
    <CommandRunner command={commands.find((item) => item.id === 'issue.bulk_create') ?? null} open={bulkCreateOpen} onClose={() => setBulkCreateOpen(false)} onSuccess={afterIssueChange} />
    <IssueDetails issue={selectedIssue} commands={commands} open={Boolean(selectedIssue)} onClose={() => setSelectedIssue(null)} onChanged={afterIssueChange} onOpenGraph={() => { const ref = selectedIssue ? `${selectedIssue.repository_name || 'Repository'}#${selectedIssue.number}` : ''; setSelectedIssue(null); navigate(`/graph?from=${encodeURIComponent(ref)}`); }} pullRequestTemplate={config?.PULL_REQUEST_TEMPLATE} />
  </div>;
}
