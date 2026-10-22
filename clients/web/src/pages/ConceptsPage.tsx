import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { BrainCircuit, GitBranch, Plus, Search, Trash2, Link2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { Concept, ProtocolCommand } from '@/types/idios';
import { Button, Card, EmptyState, PageHeading } from '@/components/ui/primitives';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { formatRelative } from '@/components/ui/IssueTable';
import { useWorkspaceStore } from '@/store/workspace-store';
import { Modal } from '@/components/ui/Modal';

export function ConceptsPage() {
  const navigate = useNavigate();
  const [items, setItems] = useState<Concept[]>([]);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const [selectedConcept, setSelectedConcept] = useState<Concept | null>(null);
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const { load } = useWorkspaceStore();
  async function refresh() {
    try { const [concepts, catalog] = await Promise.all([api.concepts(), api.protocol()]); setItems(concepts); setCommands(catalog.commands); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to load concepts.'); }
  }
  useEffect(() => { void refresh(); }, []);
  const filtered = useMemo(() => items.filter((item) => `${item.name} ${item.description}`.toLowerCase().includes(query.trim().toLowerCase())), [items, query]);
  function run(id: string, values: Record<string, unknown> = {}) { const command = commands.find((item) => item.id === id); if (!command) return; setInitialValues(values); setSelectedCommand(command); }
  async function complete() { setSelectedCommand(null); setSelectedConcept(null); await Promise.all([refresh(), load(true)]); }
  return <div className="page-stack concepts-page">
    <PageHeading eyebrow="KNOWLEDGE MODEL" title="Concepts" description="Define a reusable idea once, then connect it to issues, notes, and other knowledge." actions={<Button onClick={() => run('concept.create')}><Plus size={16} /> New concept</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="concepts-toolbar"><label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search concepts…" aria-label="Search concepts" /></label><span>{filtered.length} of {items.length} concepts</span></div>
    {filtered.length ? <div className="concept-grid">{filtered.map((concept, index) => <article className="concept-tile" key={concept.id}>
      <button type="button" className="concept-tile__open" onClick={() => setSelectedConcept(concept)} aria-label={`Open concept ${concept.name}`}>
        <span className={`concept-symbol concept-symbol--${index % 4}`}><BrainCircuit size={17} /></span>
        <span className="concept-tile__body"><strong>{concept.name}</strong><span>{concept.description || 'No description added.'}</span><small>Updated {formatRelative(concept.updated_at)}</small></span>
      </button>
      <div className="concept-tile__actions"><button type="button" className="icon-button" title="View graph" aria-label={`View graph for ${concept.name}`} onClick={() => navigate(`/graph?from=${encodeURIComponent(`concept:${concept.name}`)}`)}><GitBranch size={14} /></button><button type="button" className="icon-button" title="Create relation" aria-label={`Create relation for ${concept.name}`} onClick={() => run('relation.create', { source_type: 'concept', source: concept.id })}><Link2 size={14} /></button><button type="button" className="icon-button concept-tile__delete" title="Delete concept" aria-label={`Delete ${concept.name}`} onClick={() => run('concept.delete', { ref: concept.id })}><Trash2 size={14} /></button></div>
    </article>)}</div> : <Card><EmptyState title={query ? 'No concepts match' : 'Build a reusable knowledge layer'} body={query ? 'Try a different search.' : 'Concepts describe recurring ideas and connect related work. Keep the description short; open a concept to see its full context.'} icon={<BrainCircuit size={22} />} /></Card>}
    <Modal open={Boolean(selectedConcept)} title={selectedConcept?.name || 'Concept'} description="Concept details and connected knowledge" onClose={() => setSelectedConcept(null)} wide>
      {selectedConcept && <div className="concept-detail">
        <div className="concept-detail__summary"><span className="concept-symbol"><BrainCircuit size={20} /></span><div><strong>{selectedConcept.name}</strong><p>{selectedConcept.description || 'No description added yet.'}</p><small>Created {new Date(selectedConcept.created_at).toLocaleDateString()} · Updated {formatRelative(selectedConcept.updated_at)}</small></div></div>
        <div className="concept-detail__actions"><Button onClick={() => { navigate(`/graph?from=${encodeURIComponent(`concept:${selectedConcept.name}`)}`); setSelectedConcept(null); }}><GitBranch size={14} /> Open graph</Button><Button variant="secondary" onClick={() => { run('relation.create', { source_type: 'concept', source: selectedConcept.id }); setSelectedConcept(null); }}><Link2 size={14} /> Create relation</Button><Button variant="danger" onClick={() => { run('concept.delete', { ref: selectedConcept.id }); setSelectedConcept(null); }}><Trash2 size={14} /> Delete</Button></div>
      </div>}
    </Modal>
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
  </div>;
}
