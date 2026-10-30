import { useEffect, useMemo, useState, type CSSProperties } from 'react';
import { Bookmark, CalendarRange, Plus, Search, Tags, Trash2, X } from 'lucide-react';
import { api } from '@/lib/api';
import type { Label, Milestone, ProtocolCommand } from '@/types/idios';
import { Badge, Button, EmptyState, PageHeading } from '@/design-system';
import { CommandRunner } from '@/features/commands';
import { formatRelative } from '@/lib/format-relative';
import { useWorkspaceStore } from '@/store/workspace-store';

type PlanningTab = 'labels' | 'milestones';
type ScopeFilter = 'all' | 'global' | 'repository' | 'project';
const labelColor = (value: string) => /^#[0-9a-f]{6}$/i.test(value) ? value : '#1f6feb';

export function PlanningPage() {
  const [tab, setTab] = useState<PlanningTab>('labels');
  const [labels, setLabels] = useState<Label[]>([]);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [scope, setScope] = useState<ScopeFilter>('all');
  const { load } = useWorkspaceStore();

  async function refresh() {
    setError('');
    try { const [allLabels, allMilestones, catalog] = await Promise.all([api.labels(), api.milestones(), api.protocol()]); setLabels(allLabels); setMilestones(allMilestones); setCommands(catalog.commands); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to load labels and milestones.'); }
  }
  useEffect(() => { void refresh(); }, []);
  const filteredLabels = useMemo(() => labels.filter((label) => {
    const matchesQuery = `${label.name} ${label.description} ${label.repository_name ?? ''} ${label.scope_type}`.toLowerCase().includes(query.trim().toLowerCase());
    return matchesQuery && (scope === 'all' || label.scope_type === scope);
  }), [labels, query, scope]);
  const filteredMilestones = useMemo(() => milestones.filter((item) => {
    const matchesQuery = `${item.title} ${item.description} ${item.scope_name ?? ''} ${item.scope_type}`.toLowerCase().includes(query.trim().toLowerCase());
    return matchesQuery && (scope === 'all' || item.scope_type === scope);
  }), [milestones, query, scope]);
  function run(id: string, values: Record<string, unknown> = {}) { const command = commands.find((item) => item.id === id); if (!command) return; setInitialValues(values); setSelectedCommand(command); }
  async function complete() { setSelectedCommand(null); await Promise.all([refresh(), load(true)]); }
  function selectTab(next: PlanningTab) { setTab(next); setQuery(''); setScope('all'); }

  return <div className="page-stack planning-page">
    <PageHeading eyebrow="WORK ORGANIZATION" title="Labels & milestones" description="Use color labels to spot important work quickly, and milestones to group issues around a goal or date." actions={<Button onClick={() => run(tab === 'labels' ? 'label.create' : 'milestone.create')}><Plus size={16} /> New {tab === 'labels' ? 'label' : 'milestone'}</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="planning-toolbar">
      <div className="planning-tabs" role="tablist" aria-label="Organization type">
        <button role="tab" aria-selected={tab === 'labels'} className={tab === 'labels' ? 'planning-tab planning-tab--active' : 'planning-tab'} onClick={() => selectTab('labels')}><Tags size={15} /> Labels <Badge>{labels.length}</Badge></button>
        <button role="tab" aria-selected={tab === 'milestones'} className={tab === 'milestones' ? 'planning-tab planning-tab--active' : 'planning-tab'} onClick={() => selectTab('milestones')}><CalendarRange size={15} /> Milestones <Badge>{milestones.length}</Badge></button>
      </div>
      <label className="inline-search"><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={tab === 'labels' ? 'Find a label…' : 'Find a milestone…'} aria-label={tab === 'labels' ? 'Search labels' : 'Search milestones'} />{query && <button type="button" aria-label="Clear search" onClick={() => setQuery('')}><X size={13} /></button>}</label>
    </div>
    <div className="planning-scope-filters" role="tablist" aria-label="Filter by scope">
      {(tab === 'labels' ? [['all', 'All labels'], ['global', 'Workspace'], ['repository', 'Repository']] : [['all', 'All milestones'], ['repository', 'Repository'], ['project', 'Project']]).map(([value, label]) => <button type="button" key={value} role="tab" aria-selected={scope === value} className={scope === value ? 'planning-scope-filter planning-scope-filter--active' : 'planning-scope-filter'} onClick={() => setScope(value as ScopeFilter)}>{label}<span>{value === 'all' ? (tab === 'labels' ? labels.length : milestones.length) : tab === 'labels' ? labels.filter((item) => item.scope_type === value).length : milestones.filter((item) => item.scope_type === value).length}</span></button>)}
    </div>
    <section className="planning-browser panel" aria-label={tab === 'labels' ? 'Labels library' : 'Milestone library'}>
      <div className="planning-browser__heading"><div><h2>{tab === 'labels' ? 'Label library' : 'Milestones'}</h2><span>{tab === 'labels' ? `${filteredLabels.length} of ${labels.length} labels` : `${filteredMilestones.length} of ${milestones.length} milestones`}</span></div><Button variant="secondary" onClick={() => run(tab === 'labels' ? 'label.create' : 'milestone.create')}><Plus size={14} /> Add {tab === 'labels' ? 'label' : 'milestone'}</Button></div>
      {tab === 'labels' ? filteredLabels.length ? <div className="planning-card-grid">{filteredLabels.map((label) => <article className="planning-label-card" key={label.id} style={{ '--label-color': labelColor(label.color) } as CSSProperties}>
        <div className="planning-label-card__top"><span className="planning-label-pill"><i />{label.name}</span><button type="button" className="icon-button planning-row-delete" title={`Delete label ${label.name}`} aria-label={`Delete label ${label.name}`} onClick={() => run('label.delete', { ref: label.id })}><Trash2 size={14} /></button></div>
        <p>{label.description || 'No description'}</p><div className="planning-label-card__footer"><Badge>{label.scope_type === 'global' ? 'Workspace' : 'Repository'}</Badge>{label.repository_name && <span>{label.repository_name}</span>}</div>
      </article>)}</div> : <EmptyState title={query || scope !== 'all' ? 'No labels match' : 'No labels yet'} body={query || scope !== 'all' ? 'Try a different search or scope.' : 'Create labels like important, hotfix, research, or needs-review to make related work easy to spot.'} icon={<Tags size={21} />} /> : filteredMilestones.length ? <div className="planning-card-grid planning-card-grid--milestones">{filteredMilestones.map((milestone) => <article className="planning-milestone-card" key={milestone.id}>
        <div className="planning-milestone-card__top"><span className="planning-milestone-icon"><Bookmark size={16} /></span><button type="button" className="icon-button planning-row-delete" title={`Delete milestone ${milestone.title}`} aria-label={`Delete milestone ${milestone.title}`} onClick={() => run('milestone.delete', { ref: milestone.id })}><Trash2 size={14} /></button></div>
        <strong>{milestone.title}</strong><p>{milestone.description || 'No description'}</p><div className="planning-milestone-card__footer"><Badge tone="amber">{milestone.scope_type}</Badge><span>{milestone.scope_name || milestone.scope_id}</span><time>{milestone.due_date ? `Due ${milestone.due_date}` : `Updated ${formatRelative(milestone.updated_at)}`}</time></div>
      </article>)}</div> : <EmptyState title={query || scope !== 'all' ? 'No milestones match' : 'No milestones yet'} body={query || scope !== 'all' ? 'Try a different search or scope.' : 'Create a milestone to group a set of issues around a goal or deadline.'} icon={<CalendarRange size={21} />} />}
    </section>
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
  </div>;
}
