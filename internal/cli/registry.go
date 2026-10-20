package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"idios/internal/application"
)

// command is one CLI action, described as data: words to match, flags to accept and the
// Application use case to call. Everything else (parsing, required flags, help, usage
// errors) derives from this table, so no action repeats the same boilerplate.
type command struct {
	name   string   // top-level word
	alias  string   // accepted second spelling of the top-level word
	noun   string   // name used in error text such as `unknown repo action "x"`
	action []string // sub-command words; empty for a stand-alone command
	flags  []flagSpec
	run    func(context.Context, *application.Service, values) (any, error)
}

var commands = []command{
	{
		name: "seed", noun: "seed", action: []string{"demo"},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.SeedDemo(ctx)
		},
	},
	{
		name: "repo", alias: "repository", noun: "repo", action: []string{"create"},
		flags: []flagSpec{{name: "name", usage: "repository name", required: true}, {name: "description", usage: "description"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateRepository(ctx, v.Str("name"), v.Str("description"))
		},
	},
	{
		name: "repo", alias: "repository", noun: "repo", action: []string{"list"},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListRepositories(ctx)
		},
	},
	{
		name: "repo", alias: "repository", noun: "repo", action: []string{"show"},
		flags: []flagSpec{{name: "repo", usage: "repository name or ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.GetRepository(ctx, v.Str("repo"))
		},
	},
	{
		name: "repo", alias: "repository", noun: "repo", action: []string{"settings", "get"},
		flags: []flagSpec{{name: "repo", usage: "repository", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.GetRepositorySettings(ctx, v.Str("repo"))
		},
	},
	{
		name: "repo", alias: "repository", noun: "repo", action: []string{"settings", "set"},
		flags: []flagSpec{
			{name: "repo", usage: "repository", required: true},
			{name: "key", usage: "setting key", required: true},
			{name: "value", usage: "setting value"},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.SetRepositorySetting(ctx, v.Str("repo"), v.Str("key"), v.Str("value")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "saved", "repository": v.Str("repo"), "key": v.Str("key")}, nil
		},
	},

	{
		name: "project", noun: "project", action: []string{"create"},
		flags: []flagSpec{{name: "name", usage: "project name", required: true}, {name: "description", usage: "description"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateProject(ctx, v.Str("name"), v.Str("description"))
		},
	},
	{
		name: "project", noun: "project", action: []string{"list"},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListProjects(ctx)
		},
	},
	{
		name: "project", noun: "project", action: []string{"add-issue"},
		flags: []flagSpec{{name: "project", usage: "project name or ID", required: true}, {name: "issue", usage: "Repository#number or issue ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.AddIssueToProject(ctx, v.Str("project"), v.Str("issue")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "added", "project": v.Str("project"), "issue": v.Str("issue")}, nil
		},
	},
	{
		name: "project", noun: "project", action: []string{"issues"},
		flags: []flagSpec{{name: "project", usage: "project name or ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListProjectIssues(ctx, v.Str("project"))
		},
	},

	{
		name: "issue", noun: "issue", action: []string{"create"},
		flags: []flagSpec{
			{name: "repo", usage: "repository name or ID", required: true},
			{name: "title", usage: "issue title", required: true},
			{name: "description", usage: "description"},
			{name: "parent", usage: "parent issue reference"},
			{name: "due", usage: "due date YYYY-MM-DD"},
			{name: "priority", usage: "priority label such as p1/p2/p3"},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateIssue(ctx, v.Str("repo"), v.Str("title"), v.Str("description"), v.Str("parent"), v.Str("due"), v.Str("priority"))
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"list"},
		flags: []flagSpec{{name: "repo", usage: "optional repository"}, {name: "status", usage: "open|in_progress|done|cancelled"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := oneOf("status", v.Str("status"), "open", "in_progress", "done", "cancelled"); err != nil {
				return nil, err
			}
			return s.ListIssues(ctx, v.Str("repo"), v.Str("status"))
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"show"},
		flags: []flagSpec{{name: "id", usage: "Repository#number or stable ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.GetIssue(ctx, v.Str("id"))
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"start"},
		flags: []flagSpec{{name: "id", usage: "Repository#number or stable ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.StartIssue(ctx, v.Str("id"))
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"cancel"},
		flags: []flagSpec{{name: "id", usage: "Repository#number or stable ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CancelIssue(ctx, v.Str("id"))
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"depends"},
		flags: []flagSpec{{name: "issue", usage: "dependent issue reference", required: true}, {name: "on", usage: "prerequisite issue reference", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.AddDependency(ctx, v.Str("issue"), v.Str("on")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "dependency_added", "issue": v.Str("issue"), "depends_on": v.Str("on")}, nil
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"undepends"},
		flags: []flagSpec{{name: "issue", usage: "dependent issue reference", required: true}, {name: "on", usage: "prerequisite issue reference to remove", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.RemoveDependency(ctx, v.Str("issue"), v.Str("on")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "dependency_removed", "issue": v.Str("issue"), "no_longer_depends_on": v.Str("on")}, nil
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"deps"},
		flags: []flagSpec{{name: "id", usage: "issue reference", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListDependencies(ctx, v.Str("id"))
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"concept"}, flags: refPair("issue", "issue reference", "concept", "concept name or ID"),
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.LinkIssueConcept(ctx, v.Str("issue"), v.Str("concept")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "linked", "issue": v.Str("issue"), "concept": v.Str("concept")}, nil
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"label"}, flags: refPair("issue", "issue reference", "label", "label name or ID"),
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.AssignLabel(ctx, v.Str("issue"), v.Str("label")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "assigned", "issue": v.Str("issue"), "label": v.Str("label")}, nil
		},
	},
	{
		name: "issue", noun: "issue", action: []string{"milestone"}, flags: refPair("issue", "issue reference", "milestone", "milestone ID, title, or scoped ref"),
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.AssignMilestone(ctx, v.Str("issue"), v.Str("milestone")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "assigned", "issue": v.Str("issue"), "milestone": v.Str("milestone")}, nil
		},
	},

	{
		name: "pr", alias: "pull-request", noun: "PR", action: []string{"create"},
		flags: []flagSpec{
			{name: "issue", usage: "issue reference", required: true},
			{name: "title", usage: "PR title; defaults from Issue"},
			{name: "description", usage: "required work explanation and result", required: true},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreatePullRequest(ctx, v.Str("issue"), v.Str("title"), v.Str("description"))
		},
	},
	{
		name: "pr", alias: "pull-request", noun: "PR", action: []string{"list"},
		flags: []flagSpec{{name: "repo", usage: "optional repository"}, {name: "status", usage: "open|accepted|closed"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := oneOf("status", v.Str("status"), "open", "accepted", "closed"); err != nil {
				return nil, err
			}
			return s.ListPullRequests(ctx, v.Str("repo"), v.Str("status"))
		},
	},
	{
		name: "pr", alias: "pull-request", noun: "PR", action: []string{"show"},
		flags: []flagSpec{{name: "id", usage: "Repository!number or PR ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.GetPullRequest(ctx, v.Str("id"))
		},
	},
	{
		name: "pr", alias: "pull-request", noun: "PR", action: []string{"edit"},
		flags: []flagSpec{{name: "id", usage: "Repository!number or PR ID", required: true}, {name: "title", usage: "new title (optional)"}, {name: "description", usage: "new description (optional)"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.UpdatePullRequest(ctx, v.Str("id"), v.Str("title"), v.Str("description"))
		},
	},
	{
		name: "pr", alias: "pull-request", noun: "PR", action: []string{"accept"},
		flags: []flagSpec{{name: "id", usage: "Repository!number or PR ID", required: true}, {name: "summary", usage: "short completion result"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			pr, completion, issue, err := s.AcceptPullRequest(ctx, v.Str("id"), v.Str("summary"))
			if err != nil {
				return nil, err
			}
			return map[string]any{"pull_request": pr, "completion": completion, "issue": issue}, nil
		},
	},
	{
		name: "pr", alias: "pull-request", noun: "PR", action: []string{"close"},
		flags: []flagSpec{{name: "id", usage: "Repository!number or PR ID", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ClosePullRequest(ctx, v.Str("id"))
		},
	},

	{
		name: "concept", noun: "concept", action: []string{"create"},
		flags: []flagSpec{{name: "name", usage: "concept name", required: true}, {name: "description", usage: "description"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateConcept(ctx, v.Str("name"), v.Str("description"))
		},
	},
	{
		name: "concept", noun: "concept", action: []string{"list"},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListConcepts(ctx)
		},
	},

	{
		name: "relation", noun: "relation", action: []string{"add"},
		flags: []flagSpec{
			{name: "from-type", usage: "source entity type", required: true},
			{name: "from", usage: "source reference", required: true},
			{name: "to-type", usage: "target entity type", required: true},
			{name: "to", usage: "target reference", required: true},
			{name: "type", usage: "relation type", def: "related_to"},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateRelation(ctx, v.Str("from-type"), v.Str("from"), v.Str("to-type"), v.Str("to"), v.Str("type"))
		},
	},

	{
		name:  "graph",
		flags: []flagSpec{{name: "from", usage: "optional node ref e.g. concept:CPU"}, {name: "depth", usage: "neighbourhood radius when --from is set", def: "1", integer: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.Graph(ctx, v.Str("from"), v.Int("depth"))
		},
	},

	{
		name: "branch", noun: "branch", action: []string{"list"},
		flags: []flagSpec{{name: "repo", usage: "repository", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListBranches(ctx, v.Str("repo"))
		},
	},
	{
		name: "completion", noun: "completion", action: []string{"list"},
		flags: []flagSpec{{name: "repo", usage: "optional repository"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListCompletions(ctx, v.Str("repo"))
		},
	},

	{
		name: "label", noun: "label", action: []string{"create"},
		flags: []flagSpec{
			{name: "name", usage: "label name", required: true},
			{name: "scope", usage: "global|repository", def: "global"},
			{name: "repo", usage: "repository if scoped locally"},
			{name: "description", usage: "description"},
			{name: "color", usage: "optional display color"},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateLabel(ctx, v.Str("name"), v.Str("scope"), v.Str("repo"), v.Str("description"), v.Str("color"))
		},
	},
	{
		name: "label", noun: "label", action: []string{"list"},
		flags: []flagSpec{{name: "scope", usage: "global or repo name to show global+local labels"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListLabels(ctx, v.Str("scope"))
		},
	},
	{
		name: "label", noun: "label", action: []string{"assign"}, flags: refPair("issue", "issue ref", "label", "label name or ID"),
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.AssignLabel(ctx, v.Str("issue"), v.Str("label")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "assigned"}, nil
		},
	},

	{
		name: "milestone", noun: "milestone", action: []string{"create"},
		flags: []flagSpec{
			{name: "scope", usage: "repository|project", def: "repository"},
			{name: "owner", usage: "repository or project reference", required: true},
			{name: "title", usage: "milestone title", required: true},
			{name: "description", usage: "description"},
			{name: "due", usage: "YYYY-MM-DD"},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.CreateMilestone(ctx, v.Str("scope"), v.Str("owner"), v.Str("title"), v.Str("description"), v.Str("due"))
		},
	},
	{
		name: "milestone", noun: "milestone", action: []string{"list"},
		flags: []flagSpec{{name: "scope", usage: "repository|project"}, {name: "owner", usage: "optional scope owner"}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListMilestones(ctx, v.Str("scope"), v.Str("owner"))
		},
	},
	{
		name: "milestone", noun: "milestone", action: []string{"assign"}, flags: refPair("issue", "issue ref", "milestone", "milestone ID, title, or scoped ref"),
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			if err := s.AssignMilestone(ctx, v.Str("issue"), v.Str("milestone")); err != nil {
				return nil, err
			}
			return map[string]string{"status": "assigned"}, nil
		},
	},

	{
		name: "activity", noun: "activity", action: []string{"list"},
		flags: []flagSpec{
			{name: "type", usage: "entity type"},
			{name: "id", usage: "entity ID"},
			{name: "limit", usage: "maximum events", def: "50", integer: true},
		},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ListActivity(ctx, v.Str("type"), v.Str("id"), v.Int("limit"))
		},
	},

	{
		name:  "legacy-import",
		flags: []flagSpec{{name: "from", usage: "path to old IDIOS SQLite file (read-only)", required: true}},
		run: func(ctx context.Context, s *application.Service, v values) (any, error) {
			return s.ImportLegacy(ctx, v.Str("from"))
		},
	},
}

// refPair is the `--<first> ... --<second> ...` flag pair several link commands share.
func refPair(first, firstUsage, second, secondUsage string) []flagSpec {
	return []flagSpec{
		{name: first, usage: firstUsage, required: true},
		{name: second, usage: secondUsage, required: true},
	}
}

// resolve finds the command whose words prefix args and reports how many words it consumed.
func resolve(args []string) (*command, int) {
	var best *command
	for i := range commands {
		c := &commands[i]
		if !matches(c, args) {
			continue
		}
		if best == nil || len(c.action) > len(best.action) {
			best = c
		}
	}
	if best == nil {
		return nil, 0
	}
	return best, 1 + len(best.action)
}

func matches(c *command, args []string) bool {
	if len(args) < 1+len(c.action) {
		return false
	}
	if args[0] != c.name && (c.alias == "" || args[0] != c.alias) {
		return false
	}
	for i, word := range c.action {
		if args[1+i] != word {
			return false
		}
	}
	return true
}

// subcommands lists the first word of every action under a top-level command, in table
// order. It is what produces `usage: idios repo create|list|show|settings`.
func subcommands(name string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range commands {
		if c.name != name || len(c.action) == 0 || seen[c.action[0]] {
			continue
		}
		seen[c.action[0]] = true
		out = append(out, c.action[0])
	}
	return out
}

// dispatch runs one resolved command and returns the value the adapter should print.
func dispatch(ctx context.Context, svc *application.Service, args []string, flagOut io.Writer) (any, error) {
	cmd, consumed := resolve(args)
	if cmd == nil {
		if word := args[0]; hasGroup(word) {
			return nil, groupMisuse(word, args)
		}
		return nil, fmt.Errorf("unknown command %q; run `idios help`", args[0])
	}
	v, err := parseCommand(cmd.path(), cmd.flags, args[consumed:], flagOut)
	if err != nil {
		return nil, err
	}
	return cmd.run(ctx, svc, v)
}

func hasGroup(word string) bool {
	for _, c := range commands {
		if c.name == word || (c.alias != "" && c.alias == word) {
			return true
		}
	}
	return false
}

// groupMisuse explains what a top-level group accepts when it is called with too few or
// unknown sub-command words.
func groupMisuse(word string, args []string) error {
	name := word
	if child := firstMatchingName(word); child != "" {
		name = child
	}
	noun := name
	for _, c := range commands {
		if c.name == name {
			noun = c.noun
			break
		}
	}
	if len(args) > 1 {
		return fmt.Errorf("unknown %s action %q", noun, args[1])
	}
	return errors.New("usage: idios " + name + " " + strings.Join(subcommands(name), "|"))
}

// firstMatchingName maps an alias to its canonical command name.
func firstMatchingName(word string) string {
	for _, c := range commands {
		if c.name == word || (c.alias != "" && c.alias == word) {
			return c.name
		}
	}
	return ""
}

func (c *command) path() string {
	parts := append([]string{c.name}, c.action...)
	return strings.Join(parts, " ")
}
