import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router';
import { BookOpen, ChevronLeft, LibraryBig, LoaderCircle, Network, Plus } from 'lucide-react';
import { api } from '@/lib/api';
import type { GraphData, GraphNode, ShelfBook } from '@/types/idios';
import { Button, EmptyState, ErrorNotice, Field, PageHeading } from '@/design-system';
import { GraphCanvas } from '@/features/graph';
import { BookCover, ReadingBadge, relationOptions } from '@/features/shelf/components/ShelfCommon';

export function ShelfGraphPage() {
  const [graph, setGraph] = useState<GraphData>({ nodes: [], edges: [] });
  const [books, setBooks] = useState<ShelfBook[]>([]);
  const [selected, setSelected] = useState<GraphNode | null>(null);
  const [targetBook, setTargetBook] = useState('');
  const [relationType, setRelationType] = useState('related_to');
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    setLoading(true); setError('');
    try { const [shelf, data] = await Promise.all([api.shelf(), api.shelfGraph()]); setBooks(shelf.books.filter((book) => book.available)); setGraph(data); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to load Shelf graph.'); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const selectedBook = selected?.type === 'shelf_book' ? books.find((book) => book.id === selected.id.replace(/^shelf_book:/, '')) : undefined;
  const incidentEdges = selected ? graph.edges.filter((edge) => edge.source === selected.id || edge.target === selected.id) : [];
  const nodeSizes = Object.fromEntries(books.map((book) => {
    const pages = Math.max(0, book.total_pages || 0);
    // Circle diameter scales gently with total pages; books without page metadata stay small.
    const diameter = pages > 0 ? Math.min(18, Math.max(6, 6 + Math.log2(pages + 1) * 1.15)) : 7;
    return [`shelf_book:${book.id}`, diameter];
  }));
  async function addConnection(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selectedBook || !targetBook) return; setBusy(true); setError('');
    try { await api.createShelfRelation({ source_book_id: selectedBook.id, target_book_id: targetBook, relation_type: relationType }); await load(); setSelected({ ...selected!, description: selected!.description }); setTargetBook(''); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to connect these books.'); }
    finally { setBusy(false); }
  }
  return <div className="page-stack shelf-page shelf-graph-page">
    <div className="shelf-breadcrumb"><Link to="/shelf"><LibraryBig size={14} /> Shelf</Link><ChevronLeft size={13} className="shelf-breadcrumb__separator" /><span>Knowledge graph</span></div>
    <PageHeading eyebrow="THE LIBRARY AS A NETWORK" title="Shelf graph" description="Connect books by prerequisite, contrast, implementation or shared ideas. Drag nodes to arrange your map; scroll over the canvas to zoom it." actions={<Link to="/shelf" className="button button--secondary"><BookOpen size={14} /> Back to books</Link>} />
    {error && <ErrorNotice message={error} onRetry={() => void load()} />}
    <section className="shelf-graph-layout"><div className="panel shelf-graph-canvas-panel"><div className="shelf-panel-heading"><div><span className="eyebrow">INTERACTIVE CANVAS</span><h2><Network size={16} /> Your reading map</h2></div><span className="shelf-graph-save-note"><span /> Drag to organize</span></div>
      {loading ? <div className="shelf-loading"><LoaderCircle size={19} className="spin" /> Mapping your shelf…</div> : graph.nodes.length === 0 ? <EmptyState icon={<Network size={22} />} title="Your graph begins with a book" body="Connect a library folder and add relationships between books to start building your map." /> : <GraphCanvas nodes={graph.nodes} edges={graph.edges} nodeSizes={nodeSizes} selectedNodeId={selected?.id} onSelectNode={(node) => { setSelected(node); if (node.type === 'shelf_book') setTargetBook(''); }} />}
    </div><aside className="panel shelf-graph-inspector"><div className="shelf-panel-heading"><div><span className="eyebrow">NODE INSPECTOR</span><h2>{selectedBook ? 'Selected book' : 'Explore the map'}</h2></div></div>
      {selectedBook ? <div className="shelf-graph-inspector__body"><div className="shelf-graph-inspector__book"><BookCover book={selectedBook} small /><div><strong>{selectedBook.title}</strong><ReadingBadge status={selectedBook.reading_status} /></div></div><p>{selectedBook.summary || 'No README summary available.'}</p><div className="shelf-graph-inspector__stats"><span><b>{selectedBook.tags.length}</b> tags</span><span><b>{incidentEdges.length}</b> connections</span></div><div className="shelf-chip-list">{selectedBook.tags.map((tag) => <span className="shelf-chip" key={tag}>#{tag}</span>)}</div><Link className="button button--secondary shelf-graph-inspector__open" to={`/shelf/${encodeURIComponent(selectedBook.id)}`}><BookOpen size={14} /> Open book details</Link>
        <form className="shelf-graph-add-edge" onSubmit={(event) => void addConnection(event)}><h3>Add relationship</h3><Field label="Connect to"><select className="input" value={targetBook} onChange={(event) => setTargetBook(event.target.value)} required><option value="">Choose a book…</option>{books.filter((item) => item.id !== selectedBook.id).map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></Field><Field label="Relationship type"><select className="input" value={relationType} onChange={(event) => setRelationType(event.target.value)}>{relationOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></Field><Button className="shelf-button-block" type="submit" loading={busy} disabled={!targetBook}><Plus size={14} /> Add edge</Button></form>
      </div> : <div className="shelf-graph-inspector__empty"><div><Network size={22} /></div><h3>Select a book node</h3><p>Click a node to inspect it. Drag nodes to place related books together, then open one to capture notes and questions.</p><span>{graph.nodes.filter((node) => node.type === 'shelf_book').length} books · {graph.edges.length} connections</span></div>}
    </aside></section>
    <div className="shelf-graph-legend"><span><i className="shelf-legend-dot shelf-legend-dot--book" /> Book</span><span><i className="shelf-legend-line" /> Typed book relationship</span><span><i className="shelf-legend-dot shelf-legend-dot--repo" /> Companion repository</span><span>Relationships describe knowledge; they do not block work in IDIOS.</span></div>
  </div>;
}
