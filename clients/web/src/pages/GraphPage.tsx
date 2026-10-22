import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';
import { GitBranch, Info, RotateCw, Search, Workflow } from 'lucide-react';
import { api } from '@/lib/api';
import type { GraphData, GraphNode } from '@/types/idios';
import { Badge, Button, Card, LoadingState, PageHeading } from '@/components/ui/primitives';
import { GraphCanvas, type GraphCanvasHandle } from '@/components/graph/GraphCanvas';
import { GraphInspector } from '@/components/graph/GraphInspector';

export function GraphPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [graph, setGraph] = useState<GraphData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [depth, setDepth] = useState(() => {
    const requested = Number(searchParams.get('depth') || 2);
    return Number.isFinite(requested) ? Math.max(1, Math.min(5, requested)) : 2;
  });
  const [focus, setFocus] = useState(() => searchParams.get('from') || '');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<GraphNode | null>(null);
  const canvasRef = useRef<GraphCanvasHandle | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const value = await api.graph(depth, focus);
      setGraph(value);
      setSelected((current) => value.nodes.find((node) => node.id === current?.id) ?? null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load the knowledge graph.');
    } finally {
      setLoading(false);
    }
  }, [depth, focus]);

  useEffect(() => { void load(); }, [load]);

  const filteredNodes = useMemo(() => (graph?.nodes ?? []).filter((node) => (
    `${node.label} ${node.type} ${node.reference} ${node.status} ${node.description ?? ''}`
      .toLowerCase().includes(query.trim().toLowerCase())
  )), [graph, query]);
  const nodeSet = useMemo(() => new Set(filteredNodes.map((node) => node.id)), [filteredNodes]);
  const edges = useMemo(() => (graph?.edges ?? []).filter((edge) => nodeSet.has(edge.source) && nodeSet.has(edge.target)), [graph, nodeSet]);
  const selectedEdges = useMemo(() => selected ? edges.filter((edge) => edge.source === selected.id || edge.target === selected.id) : [], [edges, selected]);

  useEffect(() => {
    if (selected && !nodeSet.has(selected.id)) setSelected(null);
  }, [nodeSet, selected]);

  function showAll() {
    setFocus('');
    setSearchParams({});
    setSelected(null);
    canvasRef.current?.resetView();
  }

  function focusGraphHere(node: GraphNode) {
    setFocus(node.reference || node.id);
    setSearchParams({ from: node.reference || node.id, depth: String(depth) });
    canvasRef.current?.resetView();
  }

  function selectRelated(node: GraphNode) {
    setSelected(node);
    canvasRef.current?.focusNode(node.id);
  }

  if (loading && !graph) return <LoadingState />;

  return <div className="page-stack">
    <PageHeading eyebrow="CONNECTED KNOWLEDGE" title="Knowledge graph" description="Explore connected work with type-specific shapes, hidden label and milestone subgraphs, and organic or flowchart layouts that persist in your local Engine." actions={<Button variant="secondary" onClick={() => void load()} loading={loading}><RotateCw size={15} /> Refresh graph</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="graph-stats-row">
      <div className="graph-stat"><GitBranch size={16} /><strong>{graph?.nodes.length ?? 0}</strong><span>nodes</span></div>
      <div className="graph-stat"><Workflow size={16} /><strong>{graph?.edges.length ?? 0}</strong><span>relations</span></div>
      <div className="graph-stat"><Info size={16} /><span>Only <code>depends_on</code> blocks work</span></div>
      <div className="graph-controls">
        {focus && <Button variant="ghost" onClick={showAll}><GitBranch size={14} /> Show all</Button>}
        <label className="inline-search"><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find a node…" /></label>
        <label className="filter-select"><span>Depth</span><select value={depth} disabled={!focus} title={focus ? 'Change focused graph depth' : 'Focus a node to choose graph depth'} onChange={(event) => setDepth(Number(event.target.value))}><option value={1}>1 level</option><option value={2}>2 levels</option><option value={3}>3 levels</option><option value={4}>4 levels</option><option value={5}>5 levels</option></select></label>
      </div>
    </div>
    <div className="graph-layout">
      <Card className="graph-canvas-card"><GraphCanvas ref={canvasRef} nodes={filteredNodes} edges={edges} selectedNodeId={selected?.id} onSelectNode={setSelected} /></Card>
      <GraphInspector selected={selected} nodes={filteredNodes} edges={selectedEdges} onFocusGraph={focusGraphHere} onCenter={(node) => canvasRef.current?.focusNode(node.id)} onSelectRelated={selectRelated} />
    </div>
    <div className="graph-usage-note"><Badge tone="neutral">Engine-persisted layout</Badge><span>Node positions, zoom, pan, and the active layout mode are stored in the local IDIOS database. Hover over a branch to reveal its related labels and milestones. Reset positions returns the current mode to its generated layout.</span></div>
  </div>;
}
