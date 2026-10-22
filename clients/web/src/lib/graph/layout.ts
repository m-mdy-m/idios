import type { GraphEdge, GraphLayoutMode, GraphNode } from '@/types/idios';

export interface GraphPoint { x: number; y: number }

// Deterministic force-directed placement. Spatial buckets keep repulsion cheaper than
// all-pairs comparison on larger workspaces; this layout is a starting point, not a lock.
export function layoutGraph(nodes: GraphNode[], edges: GraphEdge[], mode: GraphLayoutMode = 'organic'): Map<string, GraphPoint> {
  if (mode === 'flowchart') return layoutFlowchart(nodes);
  return layoutOrganic(nodes, edges);
}

function layoutFlowchart(nodes: GraphNode[]): Map<string, GraphPoint> {
  const map = new Map<string, GraphPoint>();
  if (!nodes.length) return map;
  // Stable semantic lanes make common workflow directions read left-to-right.
  const laneFor: Record<string, number> = {
    project: 0, concept: 0, repository: 1, issue: 2, note: 2,
    pull_request: 3, branch: 4, completion: 5,
  };
  // Labels and milestones are intentionally auxiliary and hidden until branch hover.
  // Six active lanes therefore use the whole canvas without reserving an empty seventh lane.
  const lanes = Array.from({ length: 6 }, () => [] as GraphNode[]);
  for (const node of nodes) lanes[laneFor[node.type] ?? 2].push(node);
  for (const lane of lanes) lane.sort((a, b) => a.type.localeCompare(b.type) || a.label.localeCompare(b.label));
  const left = 68; const right = 892; const top = 40; const bottom = 520;
  const laneWidth = (right - left) / Math.max(1, lanes.length - 1);
  lanes.forEach((lane, laneIndex) => {
    lane.forEach((node, nodeIndex) => {
      const spacing = (bottom - top) / Math.max(1, lane.length + 1);
      map.set(node.id, { x: left + laneIndex * laneWidth, y: top + (nodeIndex + 1) * spacing });
    });
  });
  return map;
}

function layoutOrganic(nodes: GraphNode[], edges: GraphEdge[]): Map<string, GraphPoint> {
    const map = new Map<string, { x: number; y: number }>();
    const count = nodes.length;
    if (!count) return map;
    const cx = 480; const cy = 285;
    const state = new Map<string, { x: number; y: number; vx: number; vy: number }>();
    const hash = (value: string) => { let h = 2166136261; for (const char of value) { h ^= char.charCodeAt(0); h = Math.imul(h, 16777619); } return h >>> 0; };
    nodes.forEach((node, index) => {
      const seeded = hash(node.id);
      const angle = (seeded % 100000) / 100000 * Math.PI * 2;
      const ring = 80 + Math.sqrt(index + 1) * (count > 80 ? 21 : 27);
      state.set(node.id, { x: cx + Math.cos(angle) * ring, y: cy + Math.sin(angle) * ring, vx: 0, vy: 0 });
    });
    const nodeLinks: Array<[{ x: number; y: number; vx: number; vy: number }, { x: number; y: number; vx: number; vy: number }]> = [];
    for (const edge of edges) {
      const a = state.get(edge.source); const b = state.get(edge.target);
      if (a && b) nodeLinks.push([a, b]);
    }
    const iterations = count > 280 ? 34 : count > 120 ? 48 : 68;
    const cellSize = 108;
    for (let iteration = 0; iteration < iterations; iteration += 1) {
      const cooling = 0.72 * (1 - iteration / iterations) + 0.08;
      // Spatial buckets avoid comparing every node with every other node on a large graph.
      const buckets = new Map<string, Array<{ x: number; y: number; vx: number; vy: number }>>();
      for (const node of nodes) {
        const point = state.get(node.id)!;
        const key = `${Math.floor(point.x / cellSize)},${Math.floor(point.y / cellSize)}`;
        const bucket = buckets.get(key) ?? []; bucket.push(point); buckets.set(key, bucket);
      }
      for (let i = 0; i < count; i += 1) {
        const a = state.get(nodes[i].id)!;
        let fx = (cx - a.x) * 0.0025;
        let fy = (cy - a.y) * 0.0025;
        const cellX = Math.floor(a.x / cellSize); const cellY = Math.floor(a.y / cellSize);
        for (let gx = cellX - 1; gx <= cellX + 1; gx += 1) for (let gy = cellY - 1; gy <= cellY + 1; gy += 1) {
          for (const b of buckets.get(`${gx},${gy}`) ?? []) {
            if (a === b) continue;
            let dx = a.x - b.x; let dy = a.y - b.y;
            let distance2 = dx * dx + dy * dy;
            if (distance2 < 16) { dx += (i % 2 ? 1 : -1) * 3; dy += (i % 3 ? 1 : -1) * 3; distance2 = dx * dx + dy * dy; }
            const force = Math.min(2.3, 1450 / Math.max(100, distance2));
            fx += dx * force; fy += dy * force;
          }
        }
        a.vx = (a.vx + fx) * 0.62;
        a.vy = (a.vy + fy) * 0.62;
        a.x += Math.max(-8, Math.min(8, a.vx * cooling));
        a.y += Math.max(-8, Math.min(8, a.vy * cooling));
      }
      for (const [a, b] of nodeLinks) {
        const dx = b.x - a.x; const dy = b.y - a.y;
        const distance = Math.max(1, Math.hypot(dx, dy));
        const spring = (distance - 118) * 0.009 * cooling;
        const sx = dx / distance * spring; const sy = dy / distance * spring;
        a.x += sx; a.y += sy; b.x -= sx; b.y -= sy;
      }
    }
    const bounds = nodes.reduce((acc, node) => { const point = state.get(node.id)!; return { minX: Math.min(acc.minX, point.x), maxX: Math.max(acc.maxX, point.x), minY: Math.min(acc.minY, point.y), maxY: Math.max(acc.maxY, point.y) }; }, { minX: Number.POSITIVE_INFINITY, maxX: Number.NEGATIVE_INFINITY, minY: Number.POSITIVE_INFINITY, maxY: Number.NEGATIVE_INFINITY });
    const scale = Math.min(1, 850 / Math.max(1, bounds.maxX - bounds.minX), 480 / Math.max(1, bounds.maxY - bounds.minY));
    const offsetX = cx - ((bounds.minX + bounds.maxX) / 2) * scale; const offsetY = cy - ((bounds.minY + bounds.maxY) / 2) * scale;
    nodes.forEach((node) => { const point = state.get(node.id)!; map.set(node.id, { x: point.x * scale + offsetX, y: point.y * scale + offsetY }); });
    return map;
}
