package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// printHelp lists the real command table, so help cannot drift away from what dispatch
// actually accepts. Per-command flags live in `idios help <command>`.
func printHelp(w io.Writer) {
	fmt.Fprint(w, `IDIOS — local learning and development workspace

Usage:
  idios [--db PATH] [--json] [--verbose] COMMAND [FLAGS]
  idios help COMMAND...        Show the flags of one command

Global flags, accepted before or after COMMAND:
  --db PATH   Engine database (default: ~/.idios/idios-engine.sqlite3; IDIOS_DB overrides)
  --json      Machine-readable JSON for scripts and GUI clients
  --verbose   Write monitoring logs to stderr (IDIOS_LOG_LEVEL also enables them)

Commands:
`)
	for _, g := range helpGroups() {
		fmt.Fprintf(w, "  %s\n", g.usage())
		if note := g.note(); note != "" {
			fmt.Fprintf(w, "      %s\n", note)
		}
	}
	fmt.Fprint(w, `  idios help [COMMAND...]  Show help for one command
  idios protocol           Print the machine-readable contract version
  idios version            Print the Engine version
`)
	fmt.Fprint(w, `
References:
  Issues: Repository#12        Pull Requests: Repository!3

Workflow:
  1. idios issue create --repo CPU --title 'Learn CPU'
  2. idios issue start --id CPU#1
  3. idios pr create --issue CPU#1 --description 'What I have done so far'
  4. idios pr edit --id CPU!1 --description 'Updated progress'   (repeat while you work)
  5. idios pr accept --id CPU!1 --summary 'Core concepts reviewed'   (issue becomes done)

An Issue can become done only through an accepted PR. No real Git commit, file merge,
or non-main branch operations are performed by IDIOS. See docs/guides/workspace.md.
`)
}

// printCommandHelp shows one command's usage line and flag list.
func printCommandHelp(w io.Writer, c *command) {
	fmt.Fprintf(w, "Usage: %s\n", usageLine(c))
	if len(c.flags) == 0 {
		fmt.Fprintln(w, "\nThis command takes no flags.")
		return
	}
	fmt.Fprintln(w, "\nFlags:")
	tw := tabwriter.NewWriter(w, 2, 2, 2, ' ', 0)
	for _, f := range c.flags {
		note := f.usage
		if f.required {
			note += " (required)"
		} else if f.def != "" {
			note += " (default " + f.def + ")"
		}
		fmt.Fprintf(tw, "  %s\t%s\n", f.label(), note)
	}
	tw.Flush()
}

// printUsage writes the shortest useful hint after a misuse or a --help request.
func printUsage(w io.Writer, args []string, err error) {
	cmd, _ := resolve(args)
	fmt.Fprintln(w, "error:", err)
	if cmd != nil {
		fmt.Fprintf(w, "\nUsage: %s\n", usageLine(cmd))
		fmt.Fprintf(w, "Run `idios help %s` for flags.\n", cmd.path())
		return
	}
	if len(args) > 0 && hasGroup(args[0]) {
		fmt.Fprintf(w, "\nRun `idios help %s` for flags.\n", args[0])
		return
	}
	fmt.Fprintln(w, "\nRun `idios help` for the command list.")
}

// printGroupHelp describes a top-level group and lists each of its actions.
func printGroupHelp(w io.Writer, word string) {
	var cmds []*command
	for i := range commands {
		c := &commands[i]
		if c.name == word || (c.alias != "" && c.alias == word) {
			cmds = append(cmds, c)
		}
	}
	if len(cmds) == 0 {
		return
	}
	name := cmds[0].name
	fmt.Fprintf(w, "Usage: idios %s %s\n", name, strings.Join(subcommands(name), "|"))
	for _, c := range cmds {
		fmt.Fprintf(w, "\n  %s\n", usageLine(c))
	}
	fmt.Fprintf(w, "\nRun `idios help %s <action>` for one action's flags.\n", name)
}

// isHelpRequest reports the `--help`/`-h` result the flag package returns.
func isHelpRequest(err error) bool { return errors.Is(err, flag.ErrHelp) }

func usageLine(c *command) string {
	parts := []string{"idios", c.name}
	parts = append(parts, c.action...)
	for _, f := range c.flags {
		if f.required {
			parts = append(parts, f.label())
			continue
		}
		parts = append(parts, "["+f.label()+"]")
	}
	return strings.Join(parts, " ")
}

type helpGroup struct {
	name       string
	alias      string
	children   []string
	standalone []*command
}

func helpGroups() []helpGroup {
	var groups []helpGroup
	index := map[string]int{}
	for i := range commands {
		c := &commands[i]
		at, ok := index[c.name]
		if !ok {
			groups = append(groups, helpGroup{name: c.name, alias: c.alias})
			at = len(groups) - 1
			index[c.name] = at
		}
		if len(c.action) == 0 {
			groups[at].standalone = append(groups[at].standalone, c)
		}
	}
	for i := range groups {
		groups[i].children = subcommands(groups[i].name)
	}
	return groups
}

func (g helpGroup) usage() string {
	name := g.name
	if g.alias != "" {
		name += "|" + g.alias
	}
	if len(g.children) > 0 {
		return "idios " + name + " " + strings.Join(g.children, "|")
	}
	text := "idios " + name
	if len(g.standalone) == 1 {
		for _, f := range g.standalone[0].flags {
			if f.required {
				text += " " + f.label()
			}
		}
	}
	return text
}

func (g helpGroup) note() string {
	if len(g.children) > 0 {
		return g.groupNote()
	}
	if len(g.standalone) > 0 && len(g.standalone[0].flags) > 0 {
		return "options: " + flagsSummary(g.standalone[0])
	}
	return ""
}

func (g helpGroup) groupNote() string {
	switch g.name {
	case "repo":
		return "Repositories and their settings (marks the main branch)"
	case "project":
		return "Projects group Issues across Repositories"
	case "issue":
		return "Issues: the unit of work; only an accepted PR completes one"
	case "pr":
		return "Pull Requests: work review and acceptance records"
	case "concept":
		return "Concepts: shared knowledge nodes"
	case "relation":
		return "Semantic graph edges (workflow edges use issue depends)"
	case "branch":
		return "Branches; v1 exposes main only"
	case "completion":
		return "History of accepted work on main"
	case "label":
		return "Global and Repository-scoped labels"
	case "milestone":
		return "Repository- or Project-scoped milestones"
	case "activity":
		return "Recent workflow activity events"
	}
	return ""
}

func flagsSummary(c *command) string {
	names := make([]string, 0, len(c.flags))
	for _, f := range c.flags {
		names = append(names, "--"+f.name)
	}
	return strings.Join(names, " ")
}
