package tui

import (
	"context"
	"strings"
	"testing"

	"idios/internal/domain"
)

func sampleGraph() domain.Graph {
	return domain.Graph{
		Nodes: []domain.GraphNode{
			{ID: "repository:cpu", Type: "repository", Label: "CPU", Reference: "repo:CPU"},
			{ID: "branch:cpu-main", Type: "branch", Label: "CPU / main", Reference: "branch:CPU/main"},
			{ID: "branch:cpu-dev", Type: "branch", Label: "CPU / dev", Reference: "branch:CPU/dev"},
			{ID: "issue:cpu4", Type: "issue", Label: "CPU#4 · Learn electricity", Status: "open", Reference: "issue:CPU#4"},
			{ID: "issue:cpu5", Type: "issue", Label: "CPU#5 · Build adder", Status: "done", Reference: "issue:CPU#5"},
			{ID: "issue:arliz3", Type: "issue", Label: "ARLIZ#3 · Write chapter three", Status: "blocked", Reference: "issue:ARLIZ#3"},
			{ID: "pull_request:cpu1", Type: "pull_request", Label: "CPU!1 · Electricity notes", Status: "open", Reference: "pr:CPU!1"},
			{ID: "concept:electricity", Type: "concept", Label: "Electricity", Reference: "concept:Electricity"},
		},
		Edges: []domain.GraphEdge{
			{Source: "repository:cpu", Target: "branch:cpu-main", Type: "has_branch"},
			{Source: "repository:cpu", Target: "issue:cpu4", Type: "owns_issue"},
			{Source: "issue:arliz3", Target: "issue:cpu4", Type: "depends_on"},
			{Source: "issue:arliz3", Target: "issue:cpu5", Type: "depends_on"},
			{Source: "issue:cpu4", Target: "pull_request:cpu1", Type: "has_pull_request"},
			{Source: "pull_request:cpu1", Target: "branch:cpu-main", Type: "targets_branch"},
			{Source: "issue:arliz3", Target: "concept:electricity", Type: "uses_concept"},
		},
	}
}

func TestGraphLinkInfoKindsAndFallback(t *testing.T) {
	if phrase, kind := graphLinkInfo("depends_on", true); phrase != "waits for" || kind != linkBlocking {
		t.Fatalf("depends_on forward = %q/%q", phrase, kind)
	}
	if phrase, _ := graphLinkInfo("depends_on", false); phrase != "is needed by" {
		t.Fatalf("depends_on reverse = %q", phrase)
	}
	// Only depends_on may ever be presented as blocking.
	for edgeType, phrase := range graphPhrases {
		if edgeType != "depends_on" && phrase.kind == linkBlocking {
			t.Errorf("%s must not be classified as blocking", edgeType)
		}
	}
	phrase, kind := graphLinkInfo("custom_relation", false)
	if phrase != "custom relation (incoming)" || kind != linkContext {
		t.Fatalf("unknown edge fallback = %q/%q", phrase, kind)
	}
}

func TestGraphLayoutHidesOwnershipLinksUntilRequested(t *testing.T) {
	hidden := buildGraphLayout(sampleGraph(), "", false)
	if hidden.hiddenLinks != 3 {
		t.Fatalf("hidden ownership links = %d, want 3", hidden.hiddenLinks)
	}
	// Both branches only have ownership links, so while those are hidden the branches are hidden too.
	if hidden.hiddenNodes != 2 || hidden.counts["branch"] != 0 {
		t.Fatalf("branches with only ownership links should be hidden: hiddenNodes=%d branch count=%d", hidden.hiddenNodes, hidden.counts["branch"])
	}
	// The repository has only ownership links, so with them hidden it is simply an unlinked node.
	if hidden.linkCount != 4 {
		t.Fatalf("visible links = %d, want 4", hidden.linkCount)
	}

	shown := buildGraphLayout(sampleGraph(), "", true)
	if shown.counts["branch"] != 2 || shown.hiddenLinks != 0 || shown.hiddenNodes != 0 || shown.linkCount != 7 {
		t.Fatalf("with ownership shown: hiddenLinks=%d hiddenNodes=%d links=%d", shown.hiddenLinks, shown.hiddenNodes, shown.linkCount)
	}
}

func TestGraphLayoutPrintsEveryLinkExactlyOnce(t *testing.T) {
	for _, structure := range []bool{false, true} {
		layout := buildGraphLayout(sampleGraph(), "", structure)
		total := 0
		for _, group := range layout.groups {
			total += len(group.links)
		}
		if total != layout.linkCount {
			t.Fatalf("structure=%v: %d link rows for %d links", structure, total, layout.linkCount)
		}
	}
}

func TestGraphLayoutAnchorsLinksOnTheMoreCentralNode(t *testing.T) {
	layout := buildGraphLayout(sampleGraph(), "", false)
	var arliz *graphGroup
	for i := range layout.groups {
		if layout.groups[i].node.ID == "issue:arliz3" {
			arliz = &layout.groups[i]
		}
	}
	if arliz == nil {
		t.Fatal("ARLIZ#3 should own a group")
	}
	if len(arliz.links) != 3 || arliz.links[0].kind != linkBlocking || arliz.links[1].kind != linkBlocking || arliz.links[2].phrase != "uses concept" {
		t.Fatalf("ARLIZ#3 links = %+v (blocking first, then context)", arliz.links)
	}
	for _, group := range layout.groups {
		if group.node.Type == "concept" {
			t.Fatal("a concept should not collect links that an issue can own")
		}
	}
}

func TestGraphLayoutListsOnlyUnfinishedPrerequisitesAsBlockers(t *testing.T) {
	layout := buildGraphLayout(sampleGraph(), "", false)
	if len(layout.blockers) != 1 || layout.blockers[0].needed.ID != "issue:cpu4" || layout.blockers[0].waiting.ID != "issue:arliz3" {
		t.Fatalf("blockers = %+v", layout.blockers)
	}
	if layout.doneBlocks != 1 || layout.blockedIssues != 1 {
		t.Fatalf("doneBlocks=%d blockedIssues=%d", layout.doneBlocks, layout.blockedIssues)
	}
}

func TestGraphLayoutFocusComesFirstAndShowsBothDirections(t *testing.T) {
	layout := buildGraphLayout(sampleGraph(), "issue:CPU#4", false)
	if !layout.hasFocus || len(layout.groups) == 0 || !layout.groups[0].focus || layout.groups[0].node.ID != "issue:cpu4" {
		t.Fatalf("focus group should be first: %+v", layout.groups)
	}
	phrases := map[string]bool{}
	for _, link := range layout.groups[0].links {
		phrases[link.phrase] = true
	}
	if !phrases["is needed by"] || !phrases["has pull request"] {
		t.Fatalf("focus node should show incoming and outgoing links, got %v", phrases)
	}
}

func TestGraphLayoutOrderMatchesSelectableRows(t *testing.T) {
	layout := buildGraphLayout(sampleGraph(), "", true)
	if len(layout.order) != len(layout.groups)+len(layout.others) {
		t.Fatalf("order has %d rows, groups+others = %d", len(layout.order), len(layout.groups)+len(layout.others))
	}
	seen := map[string]bool{}
	for _, n := range layout.order {
		if seen[n.ID] {
			t.Fatalf("%s is selectable twice", n.ID)
		}
		seen[n.ID] = true
	}
	again := buildGraphLayout(sampleGraph(), "", true)
	for i := range layout.order {
		if layout.order[i].ID != again.order[i].ID {
			t.Fatal("layout order must be deterministic")
		}
	}
}

func TestGraphLayoutCapsLongListsAndReportsWhatIsMissing(t *testing.T) {
	unlinked := domain.Graph{}
	total := graphMaxOthers + 5
	for i := 0; i < total; i++ {
		id := "issue:" + strings.Repeat("x", i+1)
		unlinked.Nodes = append(unlinked.Nodes, domain.GraphNode{ID: id, Type: "issue", Label: id, Reference: id})
	}
	layout := buildGraphLayout(unlinked, "", false)
	if len(layout.others) != graphMaxOthers || layout.truncOthers != total-graphMaxOthers || len(layout.order) != graphMaxOthers {
		t.Fatalf("others=%d truncated=%d order=%d", len(layout.others), layout.truncOthers, len(layout.order))
	}

	linked := domain.Graph{Nodes: []domain.GraphNode{{ID: "concept:c", Type: "concept", Label: "C", Reference: "concept:C"}}}
	extra := 7
	for i := 0; i < graphMaxGroups+extra; i++ {
		id := "issue:" + strings.Repeat("y", i+1)
		linked.Nodes = append(linked.Nodes, domain.GraphNode{ID: id, Type: "issue", Label: id, Reference: id})
		linked.Edges = append(linked.Edges, domain.GraphEdge{Source: id, Target: "concept:c", Type: "uses_concept"})
	}
	layout = buildGraphLayout(linked, "", false)
	if len(layout.groups) != graphMaxGroups || layout.truncGroups != extra {
		t.Fatalf("groups=%d truncated=%d", len(layout.groups), layout.truncGroups)
	}
	if len(layout.order) != len(layout.groups)+len(layout.others) {
		t.Fatal("order must list exactly the rows that are drawn")
	}
}

func TestGraphCountSummaryUsesReadablePlurals(t *testing.T) {
	got := graphCountSummary(map[string]int{"pull_request": 2, "issue": 1, "branch": 2, "repository": 3})
	want := "1 issue · 2 pull requests · 3 repositories · 2 branches"
	if got != want {
		t.Fatalf("graphCountSummary() = %q, want %q", got, want)
	}
}

func TestGraphNodeTitleDropsDuplicatedReference(t *testing.T) {
	node := domain.GraphNode{Label: "CPU#4 · Learn electricity", Reference: "issue:CPU#4"}
	if got := graphNodeTitle(node); got != "Learn electricity" {
		t.Fatalf("graphNodeTitle() = %q", got)
	}
	if got := graphNodeTitle(domain.GraphNode{Label: "Electricity", Reference: "concept:Electricity"}); got != "" {
		t.Fatalf("a label equal to its reference should print once, got %q", got)
	}
}

func TestGraphPageExplainsBlockingInPlainWords(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height = 100, 60
	m.page = pageGraph
	m.graph = sampleGraph()
	view := m.View().Content
	for _, want := range []string{
		"HOW TO READ THIS", "What is blocked right now", "waits for", "[BLOCKED]", "[OPEN]",
		"Learn electricity", "Nodes and their links", "has pull request", "Ownership links: hidden",
		"Hidden: 3 ownership link(s)",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("graph page should contain %q", want)
		}
	}
	for _, unwanted := range []string{"belongs to", "has branch", "targets branch"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("ownership wording %q should be hidden by default", unwanted)
		}
	}

	m.graphStructure = true
	view = m.View().Content
	for _, want := range []string{"belongs to", "has branch", "targets branch", "Ownership links: shown"} {
		if !strings.Contains(view, want) {
			t.Errorf("with ownership links shown the page should contain %q", want)
		}
	}
}

func TestGraphPageSelectableRowsMatchKeyboardItems(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height = 100, 200
	m.page = pageGraph
	m.graph = sampleGraph()
	_ = m.View()
	layout := m.currentGraphLayout()
	if len(m.selectLines) != len(layout.order) {
		t.Fatalf("rendered %d selectable rows but keyboard navigation has %d items", len(m.selectLines), len(layout.order))
	}
	if items := m.cursorItems(); len(items) != len(layout.order) {
		t.Fatalf("cursor items = %d, want %d", len(items), len(layout.order))
	}
}

func TestGraphPageSurvivesNarrowTerminalsAndEmptyGraphs(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageGraph
	m.width, m.height = 100, 40
	m.graph = domain.Graph{}
	if view := m.View().Content; !strings.Contains(view, "NOTHING TO SHOW YET") {
		t.Fatal("empty graph should explain what to do next")
	}
	m.graph = sampleGraph()
	for _, width := range []int{48, 56, 64} {
		m.width = width
		if view := m.View().Content; !strings.Contains(view, "Knowledge graph") {
			t.Fatalf("graph page missing at width %d", width)
		}
	}
}
