package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/ports"
	"math"
	"strings"
)

type GraphService struct{ store ports.GraphStore }

func (s *GraphService) GraphLayout(ctx context.Context) (domain.GraphLayout, error) {
	layout, err := s.store.GetGraphLayout(ctx)
	if err != nil {
		return domain.GraphLayout{}, err
	}
	return layout, nil
}

func finiteGraphNumber(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// SaveGraphLayout strictly validates mode, coordinates and viewport settings before persistence.

func (s *GraphService) SaveGraphLayout(ctx context.Context, layout domain.GraphLayout) (domain.GraphLayout, error) {
	if layout.Mode != "organic" && layout.Mode != "flowchart" {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph mode must be organic or flowchart"))
	}
	if len(layout.Positions) > 2 {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout contains unsupported layout modes"))
	}
	for mode, positions := range layout.Positions {
		if mode != "organic" && mode != "flowchart" {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("unsupported graph layout mode %q", mode))
		}
		if len(positions) > 5000 {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout for %s contains more than 5000 nodes", mode))
		}
		for id, point := range positions {
			if strings.TrimSpace(id) == "" || len(id) > 256 {
				return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph node id must contain between 1 and 256 characters"))
			}
			if !finiteGraphNumber(point.X) || !finiteGraphNumber(point.Y) || point.X < -120 || point.X > 1080 || point.Y < -120 || point.Y > 690 {
				return domain.GraphLayout{}, invalidInput(fmt.Errorf("position for graph node %q is outside the supported canvas bounds", id))
			}
		}
	}
	if len(layout.Views) > 2 {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout contains unsupported viewport modes"))
	}
	for mode, view := range layout.Views {
		if mode != "organic" && mode != "flowchart" {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("unsupported graph viewport mode %q", mode))
		}
		if !finiteGraphNumber(view.X) || !finiteGraphNumber(view.Y) || !finiteGraphNumber(view.Scale) || view.X < -10000 || view.X > 10000 || view.Y < -10000 || view.Y > 10000 || view.Scale < 0.35 || view.Scale > 3.2 {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("viewport for %s must use x/y within ±10000 and scale between 0.35 and 3.2", mode))
		}
	}
	if layout.Positions == nil {
		layout.Positions = map[string]map[string]domain.GraphPosition{}
	}
	if layout.Views == nil {
		layout.Views = map[string]domain.GraphViewport{}
	}
	if layout.Shapes == nil {
		layout.Shapes = map[string]string{}
	}
	if len(layout.Shapes) > 5000 {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout contains more than 5000 customized node shapes"))
	}
	for id, shape := range layout.Shapes {
		if strings.TrimSpace(id) == "" || len(id) > 256 {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph node id must contain between 1 and 256 characters"))
		}
		switch shape {
		case "circle", "square", "rounded", "diamond", "hexagon", "ellipse", "pill", "document":
		default:
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("unsupported graph node shape %q", shape))
		}
	}
	for _, mode := range []string{"organic", "flowchart"} {
		if layout.Positions[mode] == nil {
			layout.Positions[mode] = map[string]domain.GraphPosition{}
		}
		if layout.Views[mode].Scale == 0 {
			layout.Views[mode] = domain.GraphViewport{X: 0, Y: 0, Scale: 1}
		}
	}
	if err := s.store.SaveGraphLayout(ctx, layout); err != nil {
		return domain.GraphLayout{}, err
	}
	return layout, nil
}

// ResetGraphLayout restores the generated arrangement for one mode, without disturbing the
// saved node positions in the other mode. The selected mode remains selected.

func (s *GraphService) ResetGraphLayout(ctx context.Context, mode string) (domain.GraphLayout, error) {
	if mode == "" {
		mode = "organic"
	}
	if mode != "organic" && mode != "flowchart" {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph mode must be organic or flowchart"))
	}
	layout, err := s.store.GetGraphLayout(ctx)
	if err != nil {
		return domain.GraphLayout{}, err
	}
	layout.Mode = mode
	if layout.Positions == nil {
		layout.Positions = map[string]map[string]domain.GraphPosition{}
	}
	layout.Positions[mode] = map[string]domain.GraphPosition{}
	if layout.Views == nil {
		layout.Views = map[string]domain.GraphViewport{}
	}
	layout.Views[mode] = domain.GraphViewport{X: 0, Y: 0, Scale: 1}
	if err := s.store.SaveGraphLayout(ctx, layout); err != nil {
		return domain.GraphLayout{}, err
	}
	return layout, nil
}

func (s *GraphService) Graph(ctx context.Context, from string, depth int) (domain.Graph, error) {
	g, err := s.store.ListGraph(ctx)
	if err != nil {
		return g, err
	}
	if strings.TrimSpace(from) == "" {
		return g, nil
	}
	if depth < 0 {
		depth = 0
	}
	start := -1
	for idx, n := range g.Nodes {
		if strings.EqualFold(n.Reference, from) || strings.EqualFold(n.ID, from) || strings.EqualFold(n.Type+":"+n.Label, from) {
			start = idx
			break
		}
	}
	if start < 0 {
		return domain.Graph{}, fmt.Errorf("%w: graph start node %q (try issue:ARLIZ#12 or concept:CPU)", domain.ErrNotFound, from)
	}
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
		adj[e.Target] = append(adj[e.Target], e.Source)
	}
	dist := map[string]int{g.Nodes[start].ID: 0}
	queue := []string{g.Nodes[start].ID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if dist[cur] >= depth {
			continue
		}
		for _, next := range adj[cur] {
			if _, ok := dist[next]; ok {
				continue
			}
			dist[next] = dist[cur] + 1
			queue = append(queue, next)
		}
	}
	ids := map[string]bool{}
	for id := range dist {
		ids[id] = true
	}
	filtered := domain.Graph{Nodes: []domain.GraphNode{}, Edges: []domain.GraphEdge{}}
	for _, n := range g.Nodes {
		if ids[n.ID] {
			filtered.Nodes = append(filtered.Nodes, n)
		}
	}
	for _, e := range g.Edges {
		if ids[e.Source] && ids[e.Target] {
			filtered.Edges = append(filtered.Edges, e)
		}
	}
	return filtered, nil
}

// Workspace experience APIs keep notes, quotes, reminders, notifications and profile data behind
// the same Store port as the core workflow entities.
