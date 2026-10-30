import { useEffect, useMemo, useState } from 'react';
import { Activity, ArrowLeft, ArrowRight, Command, Search, ShieldCheck, Terminal } from 'lucide-react';
import { api } from '@/lib/api';
import type { ProtocolCommand } from '@/types/idios';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/design-system';
import { CommandRunner } from '@/features/commands';
import { useWorkspaceStore } from '@/store/workspace-store';

const categoryDescriptions: Record<string, string> = {
  Issues: 'Create, update, and organize your work.',
  Repositories: 'Manage repositories and their settings.',
  Projects: 'Group issues into larger pieces of work.',
  Notes: 'Capture and connect personal notes.',
  Quotes: 'Save quotes and manage reminders.',
  Planning: 'Manage labels, milestones, and planning data.',
  Notifications: 'Review and control workspace notifications.',
};

export function CommandsPage() {
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selected, setSelected] = useState<ProtocolCommand | null>(null);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('');
  const [error, setError] = useState('');
  const { load } = useWorkspaceStore();
  useEffect(() => { let active = true; void api.protocol().then((payload) => { if (active) setCommands(payload.commands); }).catch((err: unknown) => setError(err instanceof Error ? err.message : 'Unable to load the shared command catalog.')); return () => { active = false; }; }, []);
  const categories = useMemo(() => Array.from(new Set(commands.map((command) => command.category))).sort(), [commands]);
  const filtered = useMemo(() => commands.filter((command) => (!category || category === 'All actions' || command.category === category) && `${command.title} ${command.description} ${command.category} ${command.id}`.toLowerCase().includes(query.trim().toLowerCase())), [commands, category, query]);
  const categoryCounts = useMemo(() => categories.map((name) => ({ name, count: commands.filter((command) => command.category === name).length })), [categories, commands]);
  async function complete() { await load(true); }
  function chooseCategory(name: string) { setCategory(name); setQuery(''); }
  function showCategories() { setCategory(''); setQuery(''); }

  return <div className="page-stack command-center-page">
    <PageHeading eyebrow="ACTION CATALOG" title="Commands" description="Find an action, open its form, and run it. The same commands power both the Web client and CLI." actions={<Badge tone="green"><ShieldCheck size={12} /> Protocol v1</Badge>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <section className="command-summary-strip" aria-label="Command catalog summary"><div className="command-summary-strip__icon"><Terminal size={18} /></div><div><strong>{commands.length} available actions</strong><span>Commands are grouped by what they do. Choose a group or search by name.</span></div><div className="command-summary-strip__status"><Activity size={14} /> Shared Go Engine</div></section>
    <div className="command-browser-toolbar command-browser-toolbar--simple"><label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => { setQuery(event.target.value); if (event.target.value) setCategory(''); }} placeholder="Search commands, actions, or descriptions…" aria-label="Search commands" /><kbd>⌘ K</kbd></label>{(category || query) && <Button variant="secondary" onClick={showCategories}><ArrowLeft size={14} /> All categories</Button>}</div>

    {!query.trim() && !category ? <section className="command-category-browser" aria-label="Command categories"><div className="command-category-browser__heading"><h2>Browse by category</h2><span>{categories.length} groups</span></div><div className="command-category-tiles">{categoryCounts.map(({ name, count }) => <button className="command-category-tile" key={name} onClick={() => chooseCategory(name)}><span className="command-category-tile__icon"><Command size={17} /></span><span className="command-category-tile__body"><strong>{name}</strong><small>{categoryDescriptions[name] ?? `Browse ${count} related actions.`}</small></span><span className="command-category-tile__count">{count}</span><ArrowRight size={15} className="command-category-tile__arrow" /></button>)}</div><button className="command-all-link" onClick={() => chooseCategory('All actions')}>Show every command <ArrowRight size={14} /></button></section> : <section className="command-results-section" aria-label="Command results"><div className="command-results-section__heading"><div><h2>{query.trim() ? 'Search results' : category}</h2><span>{filtered.length} {filtered.length === 1 ? 'action' : 'actions'}</span></div>{category && category !== 'All actions' && <Badge>{category}</Badge>}</div><Card className="command-list">{filtered.length ? filtered.map((command) => <button type="button" className="command-row" key={command.id} onClick={() => setSelected(command)}><span className="command-row__icon"><Command size={16} /></span><span className="command-row__body"><strong>{command.title}</strong><small>{command.description}</small><code>{command.id}</code></span><span className="command-row__field-count">{command.fields?.length ?? 0} fields</span><span className="command-row__run">Open <ArrowRight size={14} /></span></button>) : <EmptyState title="No matching actions" body="Try another search or choose a different category." icon={<Search size={22} />} />}</Card></section>}

    <CommandRunner command={selected} open={Boolean(selected)} onClose={() => setSelected(null)} onSuccess={complete} />
  </div>;
}
