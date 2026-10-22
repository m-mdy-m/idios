package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"idios/internal/domain"
)

// GetGraphLayout returns the saved workspace graph layout. A first-run workspace receives a
// deterministic default but is not written until the user changes or resets the layout.
func (s *Store) GetGraphLayout(ctx context.Context) (domain.GraphLayout, error) {
	row, ok, err := s.p().row(ctx, `SELECT layout_json FROM graph_layout WHERE scope='workspace'`)
	if err != nil {
		return domain.GraphLayout{}, err
	}
	if !ok {
		return domain.DefaultGraphLayout(), nil
	}
	var layout domain.GraphLayout
	if err := json.Unmarshal([]byte(str(row, "layout_json")), &layout); err != nil {
		return domain.GraphLayout{}, fmt.Errorf("decode saved graph layout: %w", err)
	}
	normalizeGraphLayout(&layout)
	return layout, nil
}

// SaveGraphLayout atomically replaces the single workspace layout document.
func (s *Store) SaveGraphLayout(ctx context.Context, layout domain.GraphLayout) error {
	normalizeGraphLayout(&layout)
	encoded, err := json.Marshal(layout)
	if err != nil {
		return fmt.Errorf("encode graph layout: %w", err)
	}
	return s.run(ctx, func(p provider) error {
		_, err := p.changes(ctx, `INSERT INTO graph_layout(scope,layout_json,updated_at) VALUES('workspace',?,?)
            ON CONFLICT(scope) DO UPDATE SET layout_json=excluded.layout_json, updated_at=excluded.updated_at`, string(encoded), nowText())
		return err
	})
}

func normalizeGraphLayout(layout *domain.GraphLayout) {
	if layout.Mode != "organic" && layout.Mode != "flowchart" {
		layout.Mode = "organic"
	}
	if layout.Positions == nil {
		layout.Positions = map[string]map[string]domain.GraphPosition{}
	}
	if layout.Positions["organic"] == nil {
		layout.Positions["organic"] = map[string]domain.GraphPosition{}
	}
	if layout.Positions["flowchart"] == nil {
		layout.Positions["flowchart"] = map[string]domain.GraphPosition{}
	}
	if layout.Views == nil {
		layout.Views = map[string]domain.GraphViewport{}
	}
	for _, mode := range []string{"organic", "flowchart"} {
		view, ok := layout.Views[mode]
		if !ok || view.Scale < 0.35 || view.Scale > 3.2 {
			layout.Views[mode] = domain.GraphViewport{X: 0, Y: 0, Scale: 1}
		}
	}
}
