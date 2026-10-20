package tui

import (
	"sort"
	"strconv"
	"strings"

	"idios/internal/domain"
)

// This file turns the flat node/edge list returned by the Application into the reading order
// used by the Graph page. It never invents relations and never applies blocker policy: it only
// groups, orders and words what the Engine already returned.

// Link kinds, from most to least important when reading the graph.
const (
	linkBlocking  = "blocking"  // depends_on: the work cannot start until the other item is done
	linkWorkflow  = "workflow"  // sub-issues, pull requests, completions
	linkContext   = "context"   // concepts, notes, labels, milestones, projects: never blocks
	linkStructure = "structure" // ownership plumbing (repository owns issue, branch, ...)

	graphMaxGroups = 30
	graphMaxOthers = 60
)

// graphPhrase words one stored edge type from both ends: "CPU#4 has pull request CPU!1" when read
// from the source, and "CPU!1 is the pull request of CPU#4" when read from the target.
type graphPhrase struct{ forward, reverse, kind string }

var graphPhrases = map[string]graphPhrase{
	"depends_on":          {"waits for", "is needed by", linkBlocking},
	"has_sub_issue":       {"has sub-issue", "is a sub-issue of", linkWorkflow},
	"has_pull_request":    {"has pull request", "is the pull request of", linkWorkflow},
	"produces_completion": {"produced completion", "was produced by", linkWorkflow},
	"completes_issue":     {"completes", "was completed by", linkWorkflow},
	"owns_issue":          {"owns", "belongs to", linkStructure},
	"has_branch":          {"has branch", "is a branch of", linkStructure},
	"targets_branch":      {"targets branch", "is targeted by", linkStructure},
	"recorded_on":         {"recorded on", "has completion", linkStructure},
	"defines_label":       {"defines label", "is a label of", linkStructure},
	"has_milestone":       {"has milestone", "is a milestone of", linkStructure},
	"contains_issue":      {"contains", "is part of project", linkContext},
	"labelled_with":       {"is labelled", "labels", linkContext},
	"in_milestone":        {"is in milestone", "includes", linkContext},
	"note_about":          {"is a note about", "has note", linkContext},
	"related_to":          {"relates to", "relates to", linkContext},
	"uses":                {"uses", "is used by", linkContext},
	"uses_concept":        {"uses concept", "is used by", linkContext},
	"contains":            {"contains", "is part of", linkContext},
	"part_of":             {"is part of", "contains", linkContext},
	"explains":            {"explains", "is explained by", linkContext},
	"contrasts_with":      {"contrasts with", "contrasts with", linkContext},
	"references":          {"references", "is referenced by", linkContext},
	"supports":            {"supports", "is supported by", linkContext},
	"contradicts":         {"contradicts", "is contradicted by", linkContext},
	"implements":          {"implements", "is implemented by", linkContext},
	"implemented_by":      {"is implemented by", "implements", linkContext},
}

// graphLinkInfo returns the sentence fragment and the kind for an edge type. Unknown types fall
// back to the stored name with underscores removed, and count as context (never blocking).
func graphLinkInfo(edgeType string, forward bool) (phrase, kind string) {
	if known, ok := graphPhrases[edgeType]; ok {
		if forward {
			return known.forward, known.kind
		}
		return known.reverse, known.kind
	}
	phrase = strings.ReplaceAll(edgeType, "_", " ")
	if !forward {
		phrase += " (incoming)"
	}
	return phrase, linkContext
}

func graphKindOrder(kind string) int {
	switch kind {
	case linkBlocking:
		return 0
	case linkWorkflow:
		return 1
	case linkContext:
		return 2
	}
	return 3
}

func graphKindMarker(kind string) string {
	switch kind {
	case linkBlocking:
		return "⛔"
	case linkWorkflow:
		return "→"
	case linkContext:
		return "◇"
	}
	return "▪"
}

// graphTypeRank decides which end of an edge owns the line that describes it: the more central
// entity (an Issue before a Label) collects its links, so each edge is printed exactly once.
func graphTypeRank(nodeType string) int {
	switch nodeType {
	case "issue":
		return 0
	case "pull_request":
		return 1
	case "concept":
		return 2
	case "project":
		return 3
	case "note":
		return 4
	case "repository":
		return 5
	case "milestone":
		return 6
	case "label":
		return 7
	case "branch":
		return 8
	case "completion":
		return 9
	}
	return 50
}

func graphTypeIcon(nodeType string) string {
	switch nodeType {
	case "issue":
		return "○"
	case "pull_request":
		return "⑂"
	case "concept":
		return "◇"
	case "project":
		return "▤"
	case "repository":
		return "▣"
	case "note":
		return "✎"
	case "milestone":
		return "⚑"
	case "label":
		return "#"
	case "completion":
		return "✓"
	}
	return "▪"
}

func graphTypeName(nodeType string) string {
	return strings.ReplaceAll(nodeType, "_", " ")
}

// graphNodeTitle drops the "CPU#4 · " prefix that Issue and Pull Request labels already carry,
// because the reference is printed separately as the clickable part of the row.
func graphNodeTitle(node domain.GraphNode) string {
	title := graphLabel(node)
	ref := graphNodeRef(node)
	if rest, ok := strings.CutPrefix(title, ref+" · "); ok {
		return rest
	}
	if title == ref {
		return ""
	}
	return title
}

func graphFinished(status string) bool { return strings.EqualFold(strings.TrimSpace(status), "done") }

type graphLink struct {
	phrase string
	kind   string
	other  domain.GraphNode
	known  bool   // false when the other end is not part of this (possibly focused) graph
	rawID  string // shown instead of a reference when !known
}

type graphGroup struct {
	node  domain.GraphNode
	focus bool
	links []graphLink
}

type graphBlocker struct {
	waiting, needed domain.GraphNode
}

type graphLayout struct {
	focus      domain.GraphNode
	hasFocus   bool
	blockers   []graphBlocker // prerequisites that are not done yet
	doneBlocks int            // prerequisites already satisfied
	groups     []graphGroup
	others     []domain.GraphNode // nodes that anchor no link: linked only as a target, or unlinked
	order      []domain.GraphNode // selectable rows in on-screen order: groups first, then others

	counts         map[string]int
	linkCount      int
	hiddenLinks    int // structural links not drawn
	hiddenNodes    int // unlinked branches, labels and milestones not drawn
	truncGroups    int
	truncOthers    int
	blockedIssues  int
	totalNodeCount int
}

// buildGraphLayout is pure and deterministic. focus is the Application start reference (such as
// "issue:CPU#4") or empty; showStructure includes ownership links such as "repository owns issue".
func buildGraphLayout(g domain.Graph, focus string, showStructure bool) graphLayout {
	layout := graphLayout{counts: map[string]int{}}
	byID := make(map[string]domain.GraphNode, len(g.Nodes))
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	focusID := ""
	if want := strings.TrimSpace(focus); want != "" {
		for _, n := range g.Nodes {
			if strings.EqualFold(n.Reference, want) || strings.EqualFold(n.ID, want) {
				focusID, layout.focus, layout.hasFocus = n.ID, n, true
				break
			}
		}
	}

	linked := map[string]int{}
	anchored := map[string][]graphLink{}
	waiting := map[string]bool{}
	for _, e := range g.Edges {
		src, srcKnown := byID[e.Source]
		dst, dstKnown := byID[e.Target]
		if !srcKnown && !dstKnown {
			continue
		}
		_, kind := graphLinkInfo(e.Type, true)
		if kind == linkBlocking && srcKnown && dstKnown {
			if graphFinished(dst.Status) {
				layout.doneBlocks++
			} else {
				layout.blockers = append(layout.blockers, graphBlocker{waiting: src, needed: dst})
				waiting[src.ID] = true
			}
		}
		if kind == linkStructure && !showStructure {
			layout.hiddenLinks++
			continue
		}
		layout.linkCount++
		linked[e.Source]++
		linked[e.Target]++

		anchorIsSource := true
		switch {
		case !srcKnown:
			anchorIsSource = false
		case !dstKnown:
			anchorIsSource = true
		case e.Source == focusID:
			anchorIsSource = true
		case e.Target == focusID:
			anchorIsSource = false
		default:
			anchorIsSource = graphTypeRank(src.Type) <= graphTypeRank(dst.Type)
		}
		if anchorIsSource {
			phrase, _ := graphLinkInfo(e.Type, true)
			anchored[e.Source] = append(anchored[e.Source], graphLink{phrase: phrase, kind: kind, other: dst, known: dstKnown, rawID: e.Target})
		} else {
			phrase, _ := graphLinkInfo(e.Type, false)
			anchored[e.Target] = append(anchored[e.Target], graphLink{phrase: phrase, kind: kind, other: src, known: srcKnown, rawID: e.Source})
		}
	}
	layout.blockedIssues = len(waiting)

	sort.SliceStable(layout.blockers, func(i, j int) bool {
		a, b := layout.blockers[i], layout.blockers[j]
		if ra, rb := graphNodeRef(a.waiting), graphNodeRef(b.waiting); ra != rb {
			return ra < rb
		}
		return graphNodeRef(a.needed) < graphNodeRef(b.needed)
	})

	nodes := append([]domain.GraphNode(nil), g.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if fa, fb := a.ID == focusID, b.ID == focusID; fa != fb {
			return fa
		}
		if ra, rb := graphTypeRank(a.Type), graphTypeRank(b.Type); ra != rb {
			return ra < rb
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if ra, rb := graphNodeRef(a), graphNodeRef(b); ra != rb {
			return ra < rb
		}
		return a.ID < b.ID
	})

	for _, n := range nodes {
		quietType := n.Type == "branch" || n.Type == "label" || n.Type == "milestone"
		if !showStructure && quietType && linked[n.ID] == 0 {
			layout.hiddenNodes++
			continue
		}
		layout.counts[n.Type]++
		layout.totalNodeCount++
		links := anchored[n.ID]
		if len(links) == 0 {
			if len(layout.others) < graphMaxOthers {
				layout.others = append(layout.others, n)
			} else {
				layout.truncOthers++
			}
			continue
		}
		if len(layout.groups) >= graphMaxGroups {
			layout.truncGroups++
			continue
		}
		sort.SliceStable(links, func(i, j int) bool {
			a, b := links[i], links[j]
			if ka, kb := graphKindOrder(a.kind), graphKindOrder(b.kind); ka != kb {
				return ka < kb
			}
			if a.phrase != b.phrase {
				return a.phrase < b.phrase
			}
			return graphNodeRef(a.other)+a.rawID < graphNodeRef(b.other)+b.rawID
		})
		layout.groups = append(layout.groups, graphGroup{node: n, focus: n.ID == focusID, links: links})
	}

	for _, group := range layout.groups {
		layout.order = append(layout.order, group.node)
	}
	layout.order = append(layout.order, layout.others...)
	return layout
}

// graphCountSummary words the per-type totals, for example "6 issues · 2 pull requests".
func graphCountSummary(counts map[string]int) string {
	types := make([]string, 0, len(counts))
	for t := range counts {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		if ri, rj := graphTypeRank(types[i]), graphTypeRank(types[j]); ri != rj {
			return ri < rj
		}
		return types[i] < types[j]
	})
	parts := make([]string, 0, len(types))
	for _, t := range types {
		name := graphTypeName(t)
		if counts[t] != 1 {
			name = pluralize(name)
		}
		parts = append(parts, strconv.Itoa(counts[t])+" "+name)
	}
	return strings.Join(parts, " · ")
}

func pluralize(word string) string {
	if strings.HasSuffix(word, "ch") || strings.HasSuffix(word, "sh") || strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") {
		return word + "es"
	}
	if strings.HasSuffix(word, "y") && !strings.HasSuffix(word, "ay") && !strings.HasSuffix(word, "ey") {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	return word + "s"
}
