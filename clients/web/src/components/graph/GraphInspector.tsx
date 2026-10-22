import { ArrowDownRight, GitBranch, Info } from 'lucide-react';
import { Badge, Button, Card } from '@/components/ui/primitives';
import { GRAPH_COLORS, graphTypeLabel } from '@/lib/graph/constants';
import type { GraphEdge, GraphNode } from '@/types/idios';

type Props = {
  selected: GraphNode | null;
  nodes: GraphNode[];
  edges: GraphEdge[];
  onFocusGraph: (node: GraphNode) => void;
  onCenter: (node: GraphNode) => void;
  onSelectRelated: (node: GraphNode) => void;
};

export function GraphInspector({ selected, nodes, edges, onFocusGraph, onCenter, onSelectRelated }: Props) {
  const selectedEdges = selected ? edges.filter((edge) => edge.source === selected.id || edge.target === selected.id) : [];
  return <Card className="graph-inspector">
    {selected ? <>
      <div className="inspector-eyebrow"><span className="inspector-dot" style={{ background: GRAPH_COLORS[selected.type] || 'var(--graph-other)' }} />{graphTypeLabel(selected.type)}</div>
      <h2>{selected.label}</h2>
      <p className="inspector-reference">{selected.reference || selected.id}</p>
      {selected.status && <Badge tone={selected.status === 'done' || selected.status === 'accepted' ? 'green' : selected.status === 'in_progress' ? 'blue' : selected.status === 'open' ? 'amber' : 'neutral'}>{selected.status.replaceAll('_', ' ')}</Badge>}
      {selected.description && <div className="graph-inspector-description">{selected.description}</div>}
      <div className="graph-inspector-actions">
        <Button variant="secondary" className="inspector-focus-button" onClick={() => onFocusGraph(selected)}><GitBranch size={14} /> Focus graph here</Button>
        <Button variant="ghost" onClick={() => onCenter(selected)}><ArrowDownRight size={14} /> Center</Button>
      </div>
      <div className="inspector-divider" />
      <p className="eyebrow">DIRECT RELATIONS</p>
      <strong className="inspector-count">{selectedEdges.length} connection{selectedEdges.length === 1 ? '' : 's'}</strong>
      <div className="inspector-edges">{selectedEdges.map((edge, index) => {
        const related = nodes.find((node) => node.id === (edge.source === selected.id ? edge.target : edge.source));
        return <button type="button" className="inspector-edge" key={`${edge.source}-${edge.target}-${edge.type}-${index}`} onClick={() => related && onSelectRelated(related)} disabled={!related}>
          <span className="inspector-edge-line"><ArrowDownRight size={13} /></span>
          <span><strong>{related?.label || 'Unknown node'}</strong><small>{edge.label || edge.type.replaceAll('_', ' ')}</small></span>
        </button>;
      })}{!selectedEdges.length && <p className="table-secondary">No directly connected nodes at this depth.</p>}</div>
      <div className="inspector-note"><Info size={15} /><span>Only <code>depends_on</code> relations can block an issue. Other links describe ownership, context, or semantic relationships.</span></div>
    </> : <div className="inspector-empty"><GitBranch size={22} /><h3>Node inspector</h3><p>Choose a node on the map to see its description, reference, status, and connected entities.</p></div>}
  </Card>;
}
