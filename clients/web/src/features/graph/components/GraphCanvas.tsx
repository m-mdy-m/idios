import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react';
import { Crosshair, Minus, Plus, RotateCcw, RotateCw, Shapes } from 'lucide-react';
import { api } from '@/lib/api';
import { EmptyState } from '@/design-system';
import { GRAPH_COLORS, graphTypeLabel } from '@/lib/graph/constants';
import { layoutGraph, type GraphPoint } from '@/lib/graph/layout';
import type { GraphEdge, GraphLayout, GraphNode, GraphNodeShape, GraphPoint as LayoutGraphPoint, GraphViewport } from '@/types/idios';

type Gesture = {
  kind: 'pan' | 'node';
  pointerId: number;
  nodeId?: string;
  startX: number;
  startY: number;
  originX: number;
  originY: number;
  factorX: number;
  factorY: number;
  scale: number;
};
type SaveStatus = 'loading' | 'saving' | 'saved' | 'local';
type NodeGeometry = { shape: 'roundRect' | 'rect' | 'hexagon' | 'diamond' | 'ellipse' | 'document' | 'pill' | 'circle'; width: number; height: number };

const NODE_SHAPE_OPTIONS: Array<{ value: GraphNodeShape; label: string }> = [
  { value: 'circle', label: 'Small circle' },
  { value: 'square', label: 'Square' },
  { value: 'rounded', label: 'Rounded rectangle' },
  { value: 'diamond', label: 'Diamond' },
  { value: 'hexagon', label: 'Hexagon' },
  { value: 'ellipse', label: 'Ellipse' },
  { value: 'pill', label: 'Pill' },
  { value: 'document', label: 'Document' },
];

type GraphCanvasProps = {
  nodes: GraphNode[];
  edges: GraphEdge[];
  selectedNodeId?: string;
  onSelectNode: (node: GraphNode) => void;
  nodeSizes?: Record<string, number>;
};

export type GraphCanvasHandle = {
  focusNode: (nodeId: string) => void;
  resetView: () => void;
  resetLayout: () => void;
};

const LEGACY_POSITION_STORAGE_KEY = 'idios.graph.positions.v1';
const LAYOUT_STORAGE_KEY = 'idios.graph.layout.v2';
const DEFAULT_VIEW: GraphViewport = { x: 0, y: 0, scale: 1 };
const AUXILIARY_TYPES = new Set(['label', 'milestone']);
const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

function emptyLayout(): GraphLayout {
  return {
    mode: 'organic',
    positions: { organic: {} },
    views: { organic: { ...DEFAULT_VIEW } },
    shapes: {},
  };
}

// Persisted layouts may come from an older version that still stored `flowchart`.
// Keep this compatibility shape wider than the current GraphLayout type, whose mode is
// intentionally restricted to `organic`.
type StoredGraphLayout = {
  mode?: string;
  positions?: Record<string, Record<string, LayoutGraphPoint>>;
  views?: Record<string, GraphViewport>;
  shapes?: Record<string, string>;
};

function isGraphNodeShape(value: string): value is GraphNodeShape {
  return NODE_SHAPE_OPTIONS.some((item) => item.value === value);
}

function normalizeLayout(value: StoredGraphLayout | null | undefined): GraphLayout {
  const result = emptyLayout();
  // Migrate layouts saved by older IDIOS versions. Flowchart is no longer a selectable mode;
  // preserve its coordinates only when the old saved layout was using that mode.
  const usedLegacyFlowchart = value?.mode === 'flowchart';
  const points = value?.positions?.organic;
  const legacyPoints = value?.positions?.flowchart;
  const selectedPoints = points && Object.keys(points).length > 0 ? points : (usedLegacyFlowchart ? legacyPoints : points);
  if (selectedPoints && typeof selectedPoints === 'object' && !Array.isArray(selectedPoints)) {
    for (const [id, point] of Object.entries(selectedPoints)) {
      if (point && Number.isFinite(point.x) && Number.isFinite(point.y)) {
        result.positions.organic[id] = { x: clamp(point.x, -120, 1080), y: clamp(point.y, -120, 690) };
      }
    }
  }
  const view = value?.views?.organic ?? (usedLegacyFlowchart ? value?.views?.flowchart : undefined);
  if (view && Number.isFinite(view.x) && Number.isFinite(view.y) && Number.isFinite(view.scale)) {
    result.views.organic = { x: clamp(view.x, -10000, 10000), y: clamp(view.y, -10000, 10000), scale: clamp(view.scale, 0.35, 3.2) };
  }
  for (const [id, shape] of Object.entries(value?.shapes ?? {})) {
    if (id.trim() && isGraphNodeShape(shape)) result.shapes[id] = shape;
  }
  return result;
}

function readLocalLayout(): GraphLayout {
  try {
    const raw = window.localStorage.getItem(LAYOUT_STORAGE_KEY);
    if (raw) return normalizeLayout(JSON.parse(raw) as StoredGraphLayout);
    // One-time migration from the previous browser-only position store.
    const oldRaw = window.localStorage.getItem(LEGACY_POSITION_STORAGE_KEY);
    if (oldRaw) {
      const old = JSON.parse(oldRaw) as Record<string, GraphPoint>;
      return normalizeLayout({ ...emptyLayout(), positions: { organic: old } });
    }
  } catch { /* The browser may have disabled storage. */ }
  return emptyLayout();
}

function geometry(shape: GraphNodeShape = 'circle', dotSize = 8): NodeGeometry {
  switch (shape) {
    // Small colorful dots are the default, keeping the canvas clean even with many nodes.
    case 'circle': return { shape: 'circle', width: dotSize, height: dotSize };
    case 'square': return { shape: 'rect', width: 16, height: 16 };
    case 'rounded': return { shape: 'roundRect', width: 25, height: 16 };
    case 'diamond': return { shape: 'diamond', width: 18, height: 18 };
    case 'hexagon': return { shape: 'hexagon', width: 20, height: 16 };
    case 'ellipse': return { shape: 'ellipse', width: 23, height: 15 };
    case 'pill': return { shape: 'pill', width: 24, height: 13 };
    case 'document': return { shape: 'document', width: 18, height: 20 };
    default: return { shape: 'circle', width: 12, height: 12 };
  }
}

function renderNodeShape(shape: NodeGeometry['shape'], width: number, height: number, color: string, active: boolean) {
  const dot = shape === 'circle';
  const common = { className: 'graph-node__shape', fill: color, fillOpacity: dot ? (active ? 1 : 0.88) : (active ? 0.26 : 0.13), stroke: color, strokeWidth: active ? 2.4 : 1.35 };
  const x = -width / 2; const y = -height / 2;
  switch (shape) {
    case 'roundRect': return <rect {...common} x={x} y={y} width={width} height={height} rx="9" />;
    case 'rect': return <rect {...common} x={x} y={y} width={width} height={height} rx="4" />;
    case 'hexagon': { const inset = Math.min(9, width * 0.22); return <polygon {...common} points={`${x + inset},${y} ${-x - inset},${y} ${-x},0 ${-x - inset},${-y} ${x + inset},${-y} ${x},0`} />; }
    case 'diamond': return <polygon {...common} points={`0,${y} ${-x},0 0,${-y} ${x},0`} />;
    case 'ellipse': return <ellipse {...common} rx={width / 2} ry={height / 2} />;
    case 'document': { const fold = Math.min(12, width * 0.32); const foldHeight = Math.min(12, height * 0.4); return <path {...common} d={`M ${x} ${y} H ${width / 2 - fold} L ${width / 2} ${y + foldHeight} V ${-y} H ${x} Z M ${width / 2 - fold} ${y} V ${y + foldHeight} H ${width / 2}`} />; }
    case 'pill': return <rect {...common} x={x} y={y} width={width} height={height} rx={height / 2} />;
    case 'circle': return <circle {...common} r={Math.min(width, height) / 2} />;
  }
}

export const GraphCanvas = forwardRef<GraphCanvasHandle, GraphCanvasProps>(function GraphCanvas({ nodes, edges, selectedNodeId, onSelectNode, nodeSizes = {} }, forwardedRef) {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const gestureRef = useRef<Gesture | null>(null);
  const saveQueueRef = useRef<Promise<unknown>>(Promise.resolve());
  const saveTimerRef = useRef<number | null>(null);
  const latestSnapshotRef = useRef('');
  const [layoutState, setLayoutState] = useState<GraphLayout>(emptyLayout);
  const [layoutReady, setLayoutReady] = useState(false);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('loading');
  const [draggingNodeId, setDraggingNodeId] = useState<string | null>(null);
  const [hoveredBranchId, setHoveredBranchId] = useState<string | null>(null);
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null);
  const [animatedPositions, setAnimatedPositions] = useState<Map<string, GraphPoint>>(() => new Map());
  const animatedPositionsRef = useRef<Map<string, GraphPoint>>(new Map());
  const positionAnimationRef = useRef<number | null>(null);
  const mode = layoutState.mode;
  const view = layoutState.views[mode] ?? DEFAULT_VIEW;

  useEffect(() => {
    let active = true;
    api.graphLayout().then((remote) => {
      if (!active) return;
      const saved = normalizeLayout(remote);
      const local = readLocalLayout();
      const remoteHasPositions = Object.values(saved.positions).some((map) => Object.keys(map).length > 0);
      if (!remoteHasPositions && !Object.values(local.positions).every((map) => Object.keys(map).length === 0)) {
        saved.positions = local.positions;
        saved.views = local.views;
      }
      setLayoutState(saved);
      setSaveStatus('saved');
    }).catch(() => {
      if (!active) return;
      setLayoutState(readLocalLayout());
      setSaveStatus('local');
    }).finally(() => { if (active) setLayoutReady(true); });
    return () => { active = false; };
  }, []);

  // Persist node coordinates, mode and pan/zoom in SQLite through the local Engine. Saves are
  // serialized and debounced; while dragging, writes wait until the pointer is released.
  useEffect(() => {
    if (!layoutReady || draggingNodeId) return;
    const snapshot: GraphLayout = normalizeLayout(layoutState);
    const signature = JSON.stringify(snapshot);
    latestSnapshotRef.current = signature;
    const timer = window.setTimeout(() => {
      saveTimerRef.current = null;
      setSaveStatus('saving');
      saveQueueRef.current = saveQueueRef.current.catch(() => undefined).then(() => api.saveGraphLayout(snapshot)).then(() => {
        try { window.localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(snapshot)); } catch { /* Cache is optional. */ }
        if (latestSnapshotRef.current === signature) setSaveStatus('saved');
      }).catch(() => {
        try { window.localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(snapshot)); } catch { /* Cache is optional. */ }
        if (latestSnapshotRef.current === signature) setSaveStatus('local');
      });
    }, 450);
    saveTimerRef.current = timer;
    return () => {
      window.clearTimeout(timer);
      if (saveTimerRef.current === timer) saveTimerRef.current = null;
    };
  }, [layoutState, layoutReady, draggingNodeId]);

  const primaryNodes = useMemo(() => nodes.filter((node) => !AUXILIARY_TYPES.has(node.type)), [nodes]);
  const primaryNodeIds = useMemo(() => new Set(primaryNodes.map((node) => node.id)), [primaryNodes]);
  const primaryEdges = useMemo(() => edges.filter((edge) => primaryNodeIds.has(edge.source) && primaryNodeIds.has(edge.target)), [edges, primaryNodeIds]);
  const basePositions = useMemo(() => layoutGraph(primaryNodes, primaryEdges), [primaryEdges, primaryNodes, mode]);
  const positions = useMemo(() => {
    const merged = new Map(basePositions);
    for (const node of nodes) {
      const saved = layoutState.positions[mode]?.[node.id];
      if (saved) merged.set(node.id, saved);
    }
    return merged;
  }, [basePositions, layoutState.positions, mode, nodes]);

  const visibleAuxiliary = useMemo(() => {
    const visible = new Set<string>();
    const selected = nodes.find((node) => node.id === selectedNodeId);
    if (selected && AUXILIARY_TYPES.has(selected.type)) visible.add(selected.id);
    if (!hoveredBranchId) return visible;
    const branch = nodes.find((node) => node.id === hoveredBranchId && node.type === 'branch');
    if (!branch) return visible;
    const repositoryId = edges.find((edge) => edge.type === 'has_branch' && edge.target === branch.id)?.source;
    if (!repositoryId) return visible;
    const ownerIds = new Set<string>([repositoryId]);
    // Include projects linked to the branch's repository and issues under that repository.
    for (const edge of edges) {
      if ((edge.type === 'primary_repository' || edge.type === 'related_repository') && edge.target === repositoryId) ownerIds.add(edge.source);
      if (edge.type === 'owns_issue' && edge.source === repositoryId) ownerIds.add(edge.target);
    }
    for (const edge of edges) {
      if (edge.type === 'contains_issue' && ownerIds.has(edge.source)) ownerIds.add(edge.target);
    }
    for (const edge of edges) {
      const source = nodes.find((node) => node.id === edge.source);
      const target = nodes.find((node) => node.id === edge.target);
      if (source && AUXILIARY_TYPES.has(source.type) && ownerIds.has(edge.target)) visible.add(source.id);
      if (target && AUXILIARY_TYPES.has(target.type) && ownerIds.has(edge.source)) visible.add(target.id);
    }
    return visible;
  }, [edges, hoveredBranchId, nodes, selectedNodeId]);

  const displayNodes = useMemo(() => nodes.filter((node) => !AUXILIARY_TYPES.has(node.type) || visibleAuxiliary.has(node.id)), [nodes, visibleAuxiliary]);
  const displayNodeIds = useMemo(() => new Set(displayNodes.map((node) => node.id)), [displayNodes]);
  const displayEdges = useMemo(() => edges.filter((edge) => displayNodeIds.has(edge.source) && displayNodeIds.has(edge.target)), [displayNodeIds, edges]);

  const displayPositions = useMemo(() => {
    const result = new Map(positions);
    // Reveal labels and milestones as small clusters around their actual owner when hovering a
    // branch. This keeps these secondary entities hidden by default without inventing relations.
    const ownerByAux = new Map<string, string>();
    for (const edge of edges) {
      const source = nodes.find((node) => node.id === edge.source);
      const target = nodes.find((node) => node.id === edge.target);
      if (source && AUXILIARY_TYPES.has(source.type) && target && !AUXILIARY_TYPES.has(target.type) && displayNodeIds.has(source.id)) ownerByAux.set(source.id, target.id);
      if (target && AUXILIARY_TYPES.has(target.type) && source && !AUXILIARY_TYPES.has(source.type) && displayNodeIds.has(target.id) && !ownerByAux.has(target.id)) ownerByAux.set(target.id, source.id);
    }
    const byOwner = new Map<string, GraphNode[]>();
    for (const node of displayNodes) {
      if (!AUXILIARY_TYPES.has(node.type)) continue;
      const ownerId = ownerByAux.get(node.id);
      if (!ownerId) continue;
      const group = byOwner.get(ownerId) ?? [];
      group.push(node); byOwner.set(ownerId, group);
    }
    for (const [ownerId, auxiliaryNodes] of byOwner) {
      const owner = positions.get(ownerId);
      if (!owner) continue;
      auxiliaryNodes.sort((a, b) => a.type.localeCompare(b.type) || a.label.localeCompare(b.label));
      const side = owner.x > 700 ? -1 : 1;
      auxiliaryNodes.forEach((node, index) => {
        if (layoutState.positions[mode]?.[node.id]) return;
        const count = auxiliaryNodes.length;
        result.set(node.id, {
          x: clamp(owner.x + side * 92, 48, 912),
          y: clamp(owner.y + (index - (count - 1) / 2) * 43, 35, 535),
        });
      });
    }
    return result;
  }, [displayNodeIds, displayNodes, edges, layoutState.positions, mode, nodes, positions]);

  // Morph nodes between layouts and when secondary subgraphs appear. Edges use the same
  // interpolated coordinates, so the relationships stay attached to their nodes throughout.
  // While dragging, follow the pointer immediately instead of easing behind it.
  useEffect(() => {
    if (positionAnimationRef.current !== null) {
      window.cancelAnimationFrame(positionAnimationRef.current);
      positionAnimationRef.current = null;
    }

    const target = new Map(displayPositions);
    const current = animatedPositionsRef.current;
    const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
    const shouldAnimate = !draggingNodeId && !prefersReducedMotion;
    const starts = new Map<string, GraphPoint>();
    let hasMovement = false;

    for (const [id, point] of target) {
      const previous = current.get(id);
      starts.set(id, previous ?? point);
      if (previous && (Math.abs(previous.x - point.x) > 0.5 || Math.abs(previous.y - point.y) > 0.5)) {
        hasMovement = true;
      }
    }

    if (!shouldAnimate || !hasMovement) {
      animatedPositionsRef.current = target;
      setAnimatedPositions(target);
      return;
    }

    const duration = 440;
    let startedAt: number | null = null;
    const tick = (now: number) => {
      if (startedAt === null) startedAt = now;
      const progress = Math.min(1, (now - startedAt) / duration);
      // Ease-out cubic gives the layout a soft stop without spring overshoot.
      const eased = 1 - Math.pow(1 - progress, 3);
      const frame = new Map<string, GraphPoint>();
      for (const [id, point] of target) {
        const from = starts.get(id) ?? point;
        frame.set(id, {
          x: from.x + (point.x - from.x) * eased,
          y: from.y + (point.y - from.y) * eased,
        });
      }
      animatedPositionsRef.current = frame;
      setAnimatedPositions(frame);
      if (progress < 1) {
        positionAnimationRef.current = window.requestAnimationFrame(tick);
      } else {
        positionAnimationRef.current = null;
      }
    };
    positionAnimationRef.current = window.requestAnimationFrame(tick);
    return () => {
      if (positionAnimationRef.current !== null) {
        window.cancelAnimationFrame(positionAnimationRef.current);
        positionAnimationRef.current = null;
      }
    };
  }, [displayPositions, draggingNodeId]);

  const renderPositions = useMemo(() => {
    if (draggingNodeId) return displayPositions;
    const result = new Map(displayPositions);
    for (const [id, point] of animatedPositions) {
      if (displayPositions.has(id)) result.set(id, point);
    }
    return result;
  }, [animatedPositions, displayPositions, draggingNodeId]);

  useEffect(() => () => {
    if (positionAnimationRef.current !== null) window.cancelAnimationFrame(positionAnimationRef.current);
  }, []);

  const updateView = useCallback((update: (current: GraphViewport) => GraphViewport) => {
    setLayoutState((current) => {
      const activeMode = current.mode;
      const currentView = current.views[activeMode] ?? DEFAULT_VIEW;
      return { ...current, views: { ...current.views, [activeMode]: update(currentView) } };
    });
  }, []);

  const zoomAt = useCallback((clientX: number, clientY: number, factor: number) => {
    const svg = svgRef.current;
    if (!svg) {
      updateView((current) => ({ ...current, scale: clamp(current.scale * factor, 0.35, 3.2) }));
      return;
    }
    const rect = svg.getBoundingClientRect();
    const cursorX = clamp((clientX - rect.left) * 960 / Math.max(1, rect.width), 0, 960);
    const cursorY = clamp((clientY - rect.top) * 570 / Math.max(1, rect.height), 0, 570);
    updateView((current) => {
      const scale = clamp(current.scale * factor, 0.35, 3.2);
      const worldX = (cursorX - current.x) / current.scale;
      const worldY = (cursorY - current.y) / current.scale;
      return { scale, x: cursorX - worldX * scale, y: cursorY - worldY * scale };
    });
  }, [updateView]);

  function zoomAtCenter(factor: number) {
    const rect = svgRef.current?.getBoundingClientRect();
    if (rect) zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, factor);
  }

  function focusNode(nodeId: string) {
    const point = renderPositions.get(nodeId) ?? positions.get(nodeId);
    if (!point) return;
    updateView((current) => ({ ...current, x: 480 - point.x * current.scale, y: 285 - point.y * current.scale }));
  }

  function resetView() { updateView(() => ({ ...DEFAULT_VIEW })); }
  async function resetLayout() {
    const activeMode = mode;
    // Cancel not-yet-started writes and serialize the reset behind any request already in flight.
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    const cleared = (current: GraphLayout): GraphLayout => ({
      ...current,
      mode: activeMode,
      positions: { ...current.positions, [activeMode]: {} },
      views: { ...current.views, [activeMode]: { ...DEFAULT_VIEW } },
    });
    setLayoutState(cleared);
    const resetRequest = saveQueueRef.current.catch(() => undefined).then(() => api.resetGraphLayout(activeMode));
    saveQueueRef.current = resetRequest;
    try {
      const reset = await resetRequest;
      setLayoutState(normalizeLayout(reset));
      try { window.localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(reset)); } catch { /* Cache is optional. */ }
      setSaveStatus('saved');
    } catch {
      const local = readLocalLayout();
      const fallback = normalizeLayout(cleared(local));
      try { window.localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(fallback)); } catch { /* Cache is optional. */ }
      setLayoutState(fallback);
      setSaveStatus('local');
    }
  }

  function setNodeShape(nextShape: GraphNodeShape) {
    if (!selectedNodeId) return;
    setLayoutState((current) => ({ ...current, shapes: { ...current.shapes, [selectedNodeId]: nextShape } }));
  }

  useImperativeHandle(forwardedRef, () => ({ focusNode, resetView, resetLayout }), [renderPositions, positions, updateView, mode]);

  useEffect(() => {
    const viewport = canvasRef.current;
    if (!viewport) return;
    const onWheel = (event: WheelEvent) => {
      event.preventDefault(); event.stopPropagation();
      zoomAt(event.clientX, event.clientY, event.deltaY < 0 ? 1.1 : 0.91);
    };
    viewport.addEventListener('wheel', onWheel, { passive: false });
    return () => viewport.removeEventListener('wheel', onWheel);
  }, [zoomAt]);

  function beginGesture(event: ReactPointerEvent<SVGSVGElement>) {
    const svg = event.currentTarget;
    const rect = svg.getBoundingClientRect();
    const factorX = 960 / Math.max(1, rect.width);
    const factorY = 570 / Math.max(1, rect.height);
    const target = event.target as Element;
    const nodeElement = target.closest?.('[data-graph-node]');
    const nodeId = nodeElement?.getAttribute('data-graph-node') ?? undefined;
    if (nodeId) {
      const point = renderPositions.get(nodeId) ?? positions.get(nodeId);
      const node = nodes.find((item) => item.id === nodeId);
      if (!point || !node) return;
      event.preventDefault();
      onSelectNode(node);
      if (node.type === 'branch') setHoveredBranchId(node.id);
      gestureRef.current = { kind: 'node', pointerId: event.pointerId, nodeId, startX: event.clientX, startY: event.clientY, originX: point.x, originY: point.y, factorX, factorY, scale: view.scale };
      setDraggingNodeId(nodeId);
    } else {
      gestureRef.current = { kind: 'pan', pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, originX: view.x, originY: view.y, factorX, factorY, scale: view.scale };
    }
    try { svg.setPointerCapture(event.pointerId); } catch { /* Capture may be unavailable during teardown. */ }
  }

  function moveGesture(event: ReactPointerEvent<SVGSVGElement>) {
    const gesture = gestureRef.current;
    if (!gesture || gesture.pointerId !== event.pointerId) return;
    const dx = (event.clientX - gesture.startX) * gesture.factorX;
    const dy = (event.clientY - gesture.startY) * gesture.factorY;
    if (gesture.kind === 'node' && gesture.nodeId) {
      setLayoutState((current) => ({
        ...current,
        positions: {
          ...current.positions,
          [current.mode]: {
            ...current.positions[current.mode],
            [gesture.nodeId!]: {
              x: clamp(gesture.originX + dx / gesture.scale, -120, 1080),
              y: clamp(gesture.originY + dy / gesture.scale, -120, 690),
            },
          },
        },
      }));
      return;
    }
    updateView((current) => ({ ...current, x: gesture.originX + dx, y: gesture.originY + dy }));
  }

  function endGesture(event: ReactPointerEvent<SVGSVGElement>) {
    if (gestureRef.current?.pointerId !== event.pointerId) return;
    gestureRef.current = null;
    setDraggingNodeId(null);
    try { if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId); } catch { /* Already released. */ }
  }

  const savedLabel = saveStatus === 'loading' ? 'LOADING GRAPH' : saveStatus === 'saving' ? 'SAVING TO ENGINE' : saveStatus === 'saved' ? 'SAVED TO ENGINE' : 'LOCAL BACKUP';

  return <>
    <div className="graph-canvas-top">
      <div className="graph-canvas-heading"><strong>Workspace map</strong><p>Drag dots to arrange them. Hover for details; select a dot to change its shape. Dot size represents book volume in Shelf.</p></div>
      <div className="graph-canvas-tools">
        <span className={`graph-live-label ${saveStatus === 'local' ? 'graph-live-label--local' : ''}`} title={saveStatus === 'local' ? 'Engine persistence is unavailable. Changes are cached in this browser.' : 'Layout and viewport are stored in the local IDIOS Engine database.'}><span className={`status-light ${saveStatus === 'local' ? '' : 'status-light--online'}`} /> {savedLabel}</span>
        <div className="graph-canvas-actions">
          <label className="graph-shape-picker"><Shapes size={14} /><span>Shape</span><select aria-label="Selected node shape" value={selectedNodeId ? layoutState.shapes[selectedNodeId] ?? 'circle' : 'circle'} disabled={!selectedNodeId} onChange={(event) => setNodeShape(event.target.value as GraphNodeShape)}>{NODE_SHAPE_OPTIONS.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>
          <div className="graph-zoom-controls">
            <button type="button" title="Zoom out" aria-label="Zoom out" onClick={() => zoomAtCenter(0.82)}><Minus size={14} /></button>
            <span>{Math.round(view.scale * 100)}%</span>
            <button type="button" title="Zoom in" aria-label="Zoom in" onClick={() => zoomAtCenter(1.22)}><Plus size={14} /></button>
            <button type="button" title="Center selected node" aria-label="Center selected node" disabled={!selectedNodeId} onClick={() => selectedNodeId && focusNode(selectedNodeId)}><Crosshair size={14} /></button>
            <button type="button" title="Reset camera position" aria-label="Reset view" onClick={resetView}><RotateCw size={13} /></button>
            <button type="button" title={`Reset ${mode} layout and viewport`} aria-label="Reset graph layout and viewport" onClick={() => void resetLayout()}><RotateCcw size={13} /></button>
          </div>
        </div>
      </div>
    </div>
    {nodes.length ? <div className="graph-canvas" ref={canvasRef}>
      <svg ref={svgRef} viewBox="0 0 960 570" role="group" aria-label={`Knowledge graph with ${displayNodes.length} visible nodes and ${displayEdges.length} visible relations`} onPointerDown={beginGesture} onPointerMove={moveGesture} onPointerUp={endGesture} onPointerCancel={endGesture} onPointerLeave={() => setHoveredBranchId(null)}>
        <defs>
          <pattern id="idios-graph-dot-grid" width="22" height="22" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r="1" fill="var(--graph-grid)" /></pattern>
          <marker id="idios-graph-edge-arrow" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto"><path d="M0,0 L7,3.5 L0,7 Z" fill="var(--graph-edge)" /></marker>
        </defs>
        <rect width="960" height="570" fill="url(#idios-graph-dot-grid)" />
        <g transform={`translate(${view.x} ${view.y}) scale(${view.scale})`}>
          <g className="graph-edges">{displayEdges.map((edge, index) => {
            const a = renderPositions.get(edge.source); const b = renderPositions.get(edge.target);
            if (!a || !b) return null;
            const sourceNode = nodes.find((node) => node.id === edge.source);
            const targetNode = nodes.find((node) => node.id === edge.target);
            if (!sourceNode || !targetNode) return null;
            const highlighted = !selectedNodeId || edge.source === selectedNodeId || edge.target === selectedNodeId;
            const blocking = edge.type === 'depends_on';
            const dx = b.x - a.x; const dy = b.y - a.y;
            const sourceGeometry = geometry(layoutState.shapes[sourceNode.id] ?? 'circle', nodeSizes[sourceNode.id] ?? 8); const targetGeometry = geometry(layoutState.shapes[targetNode.id] ?? 'circle', nodeSizes[targetNode.id] ?? 8);
            const sourcePort = sourceGeometry.shape === 'circle' ? Math.min(sourceGeometry.width, sourceGeometry.height) / 2 / Math.max(0.0001, Math.hypot(dx, dy)) : 1 / Math.max(0.0001, Math.abs(dx) / (sourceGeometry.width / 2), Math.abs(dy) / (sourceGeometry.height / 2));
            const targetPort = targetGeometry.shape === 'circle' ? Math.min(targetGeometry.width, targetGeometry.height) / 2 / Math.max(0.0001, Math.hypot(dx, dy)) : 1 / Math.max(0.0001, Math.abs(dx) / (targetGeometry.width / 2), Math.abs(dy) / (targetGeometry.height / 2));
            const ax = a.x + dx * sourcePort; const ay = a.y + dy * sourcePort;
            const bx = b.x - dx * targetPort; const by = b.y - dy * targetPort;
            const sx = a.x + Math.sign(dx || 1) * (sourceGeometry.width / 2 - 2);
            const tx = b.x - Math.sign(dx || 1) * (targetGeometry.width / 2 - 2);
            const midX = (sx + tx) / 2;
            const path = `M ${ax} ${ay} L ${bx} ${by}`;
            const connectedToSelection = Boolean(selectedNodeId && (edge.source === selectedNodeId || edge.target === selectedNodeId));
            return <g key={`${edge.source}-${edge.target}-${edge.type}-${index}`} opacity={highlighted ? 1 : 0.13}>
              <path d={path} className={`${blocking ? 'graph-edge graph-edge--blocking' : 'graph-edge'} ${connectedToSelection ? 'graph-edge--active' : ''} ${AUXILIARY_TYPES.has(sourceNode.type) || AUXILIARY_TYPES.has(targetNode.type) ? 'graph-edge--auxiliary' : ''}`} markerEnd="url(#idios-graph-edge-arrow)" />
              {(edge.label || blocking) && <text x={midX} y={(a.y + b.y) / 2 - 5} className="graph-edge-label">{edge.label || edge.type.replaceAll('_', ' ')}</text>}
            </g>;
          })}</g>
          <g className="graph-nodes">{displayNodes.map((node) => {
            const point = renderPositions.get(node.id);
            if (!point) return null;
            const active = selectedNodeId === node.id;
            const color = GRAPH_COLORS[node.type] || 'var(--graph-other)';
            const size = geometry(layoutState.shapes[node.id] ?? 'circle', nodeSizes[node.id] ?? 8);
            const aux = AUXILIARY_TYPES.has(node.type);
            const tooltipVisible = hoveredNodeId === node.id && draggingNodeId !== node.id;
            const tooltipWidth = 220;
            const tooltipHeight = node.description ? 76 : 58;
            const tooltipX = point.x > 700 ? -tooltipWidth - 18 : 18;
            const tooltipY = point.y < 105 ? 18 : -tooltipHeight - 12;
            return <g key={node.id} data-graph-node={node.id} transform={`translate(${point.x}, ${point.y})`} className={`graph-node ${active ? 'graph-node--selected' : ''} ${aux ? 'graph-node--auxiliary' : ''} ${draggingNodeId === node.id ? 'graph-node--dragging' : ''}`} role="button" tabIndex={0} aria-label={`${node.type}: ${node.label}`} aria-pressed={active} onPointerEnter={() => { setHoveredNodeId(node.id); if (node.type === 'branch') setHoveredBranchId(node.id); }} onPointerLeave={() => setHoveredNodeId((current) => current === node.id ? null : current)} onFocus={() => { setHoveredNodeId(node.id); if (node.type === 'branch') setHoveredBranchId(node.id); }} onBlur={(event) => { setHoveredNodeId((current) => current === node.id ? null : current); if (!event.currentTarget.parentElement?.contains(event.relatedTarget as Node | null)) setHoveredBranchId(null); }} onClick={() => onSelectNode(node)} onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelectNode(node); if (node.type === 'branch') setHoveredBranchId(node.id); }
            }}>
              <title>{`${node.label}${node.description ? ` — ${node.description}` : ''}`}</title>
              {renderNodeShape(size.shape, size.width, size.height, color, active)}
              {node.type === 'completion' && <circle r={Math.max(1.5, size.width * 0.22)} fill={color} />}
              {tooltipVisible && <g className="graph-hover-card" transform={`translate(${tooltipX}, ${tooltipY})`} pointerEvents="none">
                <rect width={tooltipWidth} height={tooltipHeight} rx="10" className="graph-hover-card__background" />
                <circle cx="15" cy="17" r="4" fill={color} />
                <text x="27" y="21" className="graph-hover-card__title">{node.label.length > 27 ? `${node.label.slice(0, 26)}…` : node.label}</text>
                <text x="13" y="40" className="graph-hover-card__meta">{graphTypeLabel(node.type)}{node.status ? ` · ${node.status}` : ''}</text>
                {node.description && <text x="13" y="57" className="graph-hover-card__description">{node.description.length > 35 ? `${node.description.slice(0, 34)}…` : node.description}</text>}
                <text x="13" y={tooltipHeight - 9} className="graph-hover-card__hint">Click to inspect · Drag to move</text>
              </g>}
            </g>;
          })}</g>
        </g>
      </svg>
    </div> : <EmptyState title="Your graph is waiting" body="As repositories, issues, concepts, and notes connect, IDIOS will show their recorded relationships here." />}
    <div className="graph-legend">{Object.entries(GRAPH_COLORS).map(([type, color]) => <span key={type}><i className={`graph-legend-shape graph-legend-shape--${type}`} style={{ background: color }} />{graphTypeLabel(type)}</span>)}</div>
  </>;
});
