package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/tomlx"
)

// Target names the file a write lands in. The two are not interchangeable: the
// split by data nature is what keeps a credential-shaped value out of a
// committed file, so a write that would cross it is refused rather than
// redirected.
type Target int

const (
	TargetProject Target = iota
	TargetMachine
)

func (t Target) String() string {
	if t == TargetMachine {
		return "machine"
	}
	return "project"
}

// envVarName is the shape of an environment variable name. A value that fails
// it is almost certainly the key itself rather than the name of the variable
// holding it, and configuration output ends up in bug reports and screenshots.
var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// The two key spaces, as the loader sees them.
//
// This list has to say exactly what config.go's decoders and validateStatic
// say, and for a sharper reason than tidiness: a write that lands in the wrong
// file is not refused by the loader afterwards, it makes the whole file
// unreadable. `mf config set backends.x.api_key_env ... --project` used to
// succeed, and from then on nobody could load the repository — including the
// person who ran it — until someone hand-edited the file. A write-time refusal
// costs one command; the alternative costs everybody who clones.
var (
	// Machine-only: reachability, credential references and local preferences.
	// A committed file naming any of them is refused by the loader.
	machineOnlyPrefixes = []string{"providers.", "explain.", "fingerprints.", "prices."}

	// The same rule where it is a leaf rather than a section: a backend may be
	// declared in either layer, but the two fields that carry a route may not
	// be committed.
	machineOnlySuffixes = []string{".endpoint", ".api_key_env"}

	// Project-only: the repository's own layout, the gates' inputs and the
	// vendor instruction files. The machine file has no place to decode them,
	// so a write there produces an unknown key and a machine that can run no
	// command at all.
	projectOnlyPrefixes = []string{"paths.", "checks.", "agents."}
)

func hasAnyPrefix(key string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

func isMachineOnly(key string) bool {
	if hasAnyPrefix(key, machineOnlyPrefixes) {
		return true
	}
	for _, s := range machineOnlySuffixes {
		if strings.HasSuffix(key, s) {
			return true
		}
	}
	return false
}

// Set writes one key into one layer, preserving everything else in the file.
// It edits text rather than re-encoding the document, because a policy file is
// hand-edited and re-encoding would silently drop its comments and ordering.
func Set(opts Options, key, value string, target Target) error {
	if strings.HasSuffix(key, ".api_key") {
		return fmt.Errorf("refusing to write %s: a credential must never appear in configuration; store the variable *name* in api_key_env instead", key)
	}
	if strings.HasSuffix(key, ".api_key_env") && !envVarName.MatchString(value) {
		return fmt.Errorf("refusing to write %s: %q is not an environment variable name; give the NAME of the variable holding the key (for example DEEPSEEK_API_KEY), never the key itself", key, value)
	}
	if isMachineOnly(key) && target == TargetProject {
		return fmt.Errorf("refusing to write %s into the project file: endpoints, credential references and local preferences are machine state and may not be committed", key)
	}
	if hasAnyPrefix(key, projectOnlyPrefixes) && target == TargetMachine {
		return fmt.Errorf("refusing to write %s into the machine file: where this repository keeps its documents and which paths its gates read are policy, identical on every clone, so they belong in %s", key, ProjectFileName)
	}
	if strings.HasPrefix(key, "paths.") {
		if msg := pathProblem(value); msg != "" {
			return fmt.Errorf("refusing to write %s: %s", key, msg)
		}
	}
	// Only in the project layer, because that is where the rule lives: the
	// loader refuses a committed command that names a path or an interpreter,
	// and refuses it for the whole file, so a write that lands one there costs
	// everybody who clones the repository a hand edit before anything loads.
	// The machine layer carries no such rule and must not gain one here — a
	// user's own file cannot run a repository's code on them.
	if target == TargetProject && strings.HasPrefix(key, "backends.") && strings.HasSuffix(key, ".command") {
		if msg := commandProblem(value); msg != "" {
			return fmt.Errorf("refusing to write %s: %s", key, msg)
		}
	}

	path := opts.MachinePath
	seed := "version = 1\n"
	if target == TargetProject {
		path = filepath.Join(opts.RepoRoot, ProjectFileName)
	}
	if path == "" {
		return fmt.Errorf("no path for the %s layer", target)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		body = []byte(seed)
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
			return mkErr
		}
	}

	updated, err := setInDocument(string(body), key, value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// setInDocument places `key = value` in the TOML text, creating the section
// when it is absent and replacing the value in place when it is present.
func setInDocument(doc, key, value string) (string, error) {
	idx := strings.LastIndex(key, ".")
	if idx < 0 {
		return "", fmt.Errorf("key %q has no section", key)
	}
	section, leaf := key[:idx], key[idx+1:]
	assignment := fmt.Sprintf("%s = %q", leaf, value)
	if isList(key) {
		assignment = leaf + " = " + tomlArray(value)
	}

	lines := strings.Split(doc, "\n")
	header := "[" + section + "]"
	inSection := false
	sectionStart := -1
	sectionEnd := len(lines)

	// A header is read the way the decoder reads it, not as a line that starts
	// and ends with a bracket: `[roles.r2]  # the reviewer chain` opens the
	// same table, and missing it appended a second `[roles.r2]` — a file the
	// decoder then refuses, for everyone who clones it, while the command that
	// wrote it reported success.
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if name, isHeader := tomlx.Table(trimmed); isHeader {
			if inSection {
				sectionEnd = i
				break
			}
			if name == section {
				inSection = true
				sectionStart = i
			}
			continue
		}
		if inSection && tomlx.Key(trimmed) == leaf {
			lines[i] = assignment
			return strings.Join(lines, "\n"), nil
		}
	}

	if sectionStart >= 0 {
		// Append inside the existing section, after its last non-empty line, so
		// a trailing blank line separating sections survives.
		insert := sectionEnd
		for insert > sectionStart+1 && strings.TrimSpace(lines[insert-1]) == "" {
			insert--
		}
		out := append([]string{}, lines[:insert]...)
		out = append(out, assignment)
		out = append(out, lines[insert:]...)
		return strings.Join(out, "\n"), nil
	}

	trimmedDoc := strings.TrimRight(doc, "\n")
	return trimmedDoc + "\n\n" + header + "\n" + assignment + "\n", nil
}

// listKeys are the keys whose value is a sequence rather than a scalar. A chain
// written as one comma-joined string decodes as a type error and the whole file
// is refused, so the shape has to be decided at write time — the reader never
// gets a chance to guess.
var listKeys = []string{".backends", ".args", ".unavailable_patterns", ".exempt_paths", ".design_surfaces"}

func isList(key string) bool {
	for _, suffix := range listKeys {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

// tomlArray renders a comma-separated value as a TOML array. The command line
// takes one string because that is what a shell hands over; the file takes a
// list because that is what the value is.
func tomlArray(value string) string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			items = append(items, fmt.Sprintf("%q", p))
		}
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// Migrate takes over responsibility for the deprecated git-config keys by
// writing their values into the machine file. It is deliberately
// non-destructive: it never removes the source, because deleting a machine's
// configuration as a side effect of running a command is not a migration a user
// can undo. The machine layer outranks the legacy layer, so the migrated value
// wins immediately, and the caller reports how to remove the originals.
func Migrate(opts Options) ([]string, error) {
	if opts.GitConfig == nil {
		return nil, nil
	}
	sources := make([]string, 0, len(legacyKeys))
	for gitKey := range legacyKeys {
		sources = append(sources, gitKey)
	}
	sort.Strings(sources)

	var moved []string
	for _, gitKey := range sources {
		value, ok := opts.GitConfig(gitKey)
		if !ok || value == "" {
			continue
		}
		target := legacyKeys[gitKey]
		if err := Set(opts, target, value, TargetMachine); err != nil {
			return moved, fmt.Errorf("migrating %s: %w", gitKey, err)
		}
		moved = append(moved, gitKey)
	}
	return moved, nil
}

// RemovalCommands renders what a user would run to drop the migrated keys.
// Printed rather than executed, so the destructive half stays a human decision.
func RemovalCommands(moved []string) []string {
	cmds := make([]string, 0, len(moved))
	for _, key := range moved {
		cmds = append(cmds, "git config --global --unset "+key)
	}
	return cmds
}
