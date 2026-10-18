package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// flagSpec declares one flag of one command. Declaring flags as data is what lets
// parsing, per-command help and required-value checks all come from one place.
type flagSpec struct {
	name     string
	usage    string
	def      string
	required bool
	integer  bool
}

// label is how the flag appears in a generated usage line.
func (f flagSpec) label() string {
	text := "--" + f.name
	if f.integer {
		return text + " N"
	}
	return text + " " + strings.ToUpper(f.name)
}

// values holds the parsed flags of one invocation, addressed by flag name.
type values struct {
	byName map[string]*string
	ints   map[string]*int
}

func (v values) Str(name string) string {
	if p, ok := v.byName[name]; ok && p != nil {
		return *p
	}
	return ""
}

func (v values) Int(name string) int {
	if p, ok := v.ints[name]; ok && p != nil {
		return *p
	}
	return 0
}

// parseCommand binds args to specs, rejects positional leftovers and enforces required
// flags in declaration order. flag.ErrHelp is returned untouched so the caller can print
// this command's usage and exit successfully.
func parseCommand(name string, specs []flagSpec, args []string, out io.Writer) (values, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(out)
	set.Usage = func() {} // help and misuse are rendered by this package, not by flag
	v := values{byName: map[string]*string{}, ints: map[string]*int{}}
	for _, spec := range specs {
		if spec.integer {
			v.ints[spec.name] = set.Int(spec.name, 0, spec.usage)
			continue
		}
		def := spec.def
		v.byName[spec.name] = set.String(spec.name, def, spec.usage)
	}
	if err := set.Parse(args); err != nil {
		return v, err
	}
	if set.NArg() > 0 {
		return v, fmt.Errorf("unexpected positional arguments: %s", strings.Join(set.Args(), " "))
	}
	for _, spec := range specs {
		if spec.required && strings.TrimSpace(v.Str(spec.name)) == "" {
			return v, fmt.Errorf("--%s is required", spec.name)
		}
	}
	return v, nil
}

// oneOf rejects a flag value outside the allowed set (empty means "no filter").
func oneOf(flagName, value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("invalid --%s %q (want %s)", flagName, value, strings.Join(allowed, "|"))
}
