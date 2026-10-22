package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"idios/internal/domain"
)

func TestGraphLayoutPersistsAcrossStoreRestartAndKeepsModesSeparate(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "graph-layout.sqlite3")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	layout := domain.GraphLayout{
		Mode: "flowchart",
		Positions: map[string]map[string]domain.GraphPosition{
			"organic":   {"repository:one": {X: 421, Y: 237}},
			"flowchart": {"issue:one": {X: 553, Y: 311}},
		},
		Views: map[string]domain.GraphViewport{
			"organic":   {X: 10, Y: -12, Scale: 1.1},
			"flowchart": {X: -40, Y: 25, Scale: 1.4},
		},
	}
	if err := store.SaveGraphLayout(ctx, layout); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetGraphLayout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "flowchart" {
		t.Fatalf("mode = %q, want flowchart", got.Mode)
	}
	if point := got.Positions["organic"]["repository:one"]; point != (domain.GraphPosition{X: 421, Y: 237}) {
		t.Fatalf("organic position = %+v", point)
	}
	if point := got.Positions["flowchart"]["issue:one"]; point != (domain.GraphPosition{X: 553, Y: 311}) {
		t.Fatalf("flowchart position = %+v", point)
	}
	if view := got.Views["flowchart"]; view != (domain.GraphViewport{X: -40, Y: 25, Scale: 1.4}) {
		t.Fatalf("flowchart viewport = %+v", view)
	}
}

func TestGraphLayoutDefaultsWithoutSavedRow(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "graph-layout-default.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got, err := store.GetGraphLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "organic" || got.Positions["organic"] == nil || got.Positions["flowchart"] == nil {
		t.Fatalf("unexpected graph layout defaults: %+v", got)
	}
}
