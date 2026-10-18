package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Help is generated from the command table, so this guard fails when a command is added
// that help cannot describe (a missing action word or a flag the usage line hides).
func TestHelpCoversEveryCommand(t *testing.T) {
	var help bytes.Buffer
	printHelp(&help)
	text := help.String()
	for i := range commands {
		c := &commands[i]
		if !strings.Contains(text, "idios "+c.name) {
			t.Errorf("help does not mention %q", c.name)
		}
		var cmd bytes.Buffer
		printCommandHelp(&cmd, c)
		if !strings.HasPrefix(cmd.String(), "Usage: idios "+c.path()) {
			t.Errorf("help %s: bad usage line %q", c.path(), cmd.String())
		}
		for _, f := range c.flags {
			if !strings.Contains(cmd.String(), "--"+f.name) {
				t.Errorf("help %s does not list --%s", c.path(), f.name)
			}
		}
	}
}

func TestUsageLineMarksRequiredAndOptionalFlags(t *testing.T) {
	cmd, _ := resolve([]string{"issue", "create"})
	if cmd == nil {
		t.Fatal("issue create is missing from the registry")
	}
	want := "idios issue create --repo REPO --title TITLE [--description DESCRIPTION] [--parent PARENT] [--due DUE] [--priority PRIORITY]"
	if got := usageLine(cmd); got != want {
		t.Fatalf("usageLine = %q, want %q", got, want)
	}
}

// `idios help <group>` is advertised by misuse errors, so it must describe the group.
func TestGroupHelpListsItsActions(t *testing.T) {
	var buf bytes.Buffer
	printGroupHelp(&buf, "repository")
	out := buf.String()
	if !strings.HasPrefix(out, "Usage: idios repo create|list|show|settings\n") {
		t.Fatalf("group help = %q", out)
	}
	for _, word := range subcommands("repo") {
		if !strings.Contains(out, "idios repo "+word) {
			t.Errorf("group help does not describe action %q:\n%s", word, out)
		}
	}
}

func TestGroupMisuseExplainsAlternatives(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"repo"}, "usage: idios repo create|list|show|settings"},
		{[]string{"repository", "bogus"}, `unknown repo action "bogus"`},
		{[]string{"pr", "bogus"}, `unknown PR action "bogus"`},
	}
	for _, tc := range cases {
		if got := groupMisuse(tc.args[0], tc.args).Error(); got != tc.want {
			t.Errorf("groupMisuse(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}
