// Package activate turns the framework's documented activation steps into
// commands: wiring the versioned hooks, recording which framework version a
// repository adopted, and scaffolding the project policy file.
//
// Nothing here repairs what it reports, and nothing overwrites a value a person
// set. Activation that silently rewrites local decisions is how a tool loses a
// user.
package activate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/vcs"
)

// HooksDir is the versioned directory the hooks path must point at. It lives in
// the repository rather than in .git/ so the hook itself is reviewable.
const HooksDir = ".githooks"

// LockFileName records which framework version a repository adopted.
const LockFileName = ".framework.lock"

type Repo interface {
	Config(args ...string) (string, error)
}

// HooksState is what git currently says about the hooks path.
type HooksState struct {
	Path      string
	Canonical bool
	Present   bool
}

// HooksStatus reports without changing anything.
func HooksStatus(root string) HooksState {
	repo := vcs.Open(root)
	path, err := repo.ConfigGet("core.hooksPath")
	state := HooksState{Path: strings.TrimSpace(path)}
	// Whether the versioned directory exists is a fact about the repository,
	// not about the setting, so it is answered even when nothing is wired —
	// that combination ("directory present, nothing pointing at it") is exactly
	// the unactivated repository this reports on.
	if _, statErr := os.Stat(filepath.Join(root, HooksDir)); statErr == nil {
		state.Present = true
	}
	if err != nil || state.Path == "" {
		return state
	}
	state.Canonical = filepath.ToSlash(state.Path) == HooksDir
	return state
}

// InstallHooks points git at the versioned directory. Idempotent.
func InstallHooks(root string) error {
	if _, err := os.Stat(filepath.Join(root, HooksDir)); err != nil {
		return fmt.Errorf("no %s directory in this repository; nothing to wire", HooksDir)
	}
	return vcs.Open(root).ConfigSetLocal("core.hooksPath", HooksDir)
}

// UninstallHooks removes the setting, leaving the versioned directory alone.
func UninstallHooks(root string) error {
	return vcs.Open(root).ConfigUnsetLocal("core.hooksPath")
}

// Lock is the adopted-version record.
type Lock struct {
	Version          int    `toml:"version"`
	FrameworkVersion string `toml:"framework_version"`
}

func LockPath(root string) string { return filepath.Join(root, LockFileName) }

func ReadLock(root string) (Lock, bool) {
	data, err := os.ReadFile(LockPath(root))
	if err != nil {
		return Lock{}, false
	}
	lock := Lock{Version: 1}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if strings.TrimSpace(key) == "framework_version" {
			lock.FrameworkVersion = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return lock, lock.FrameworkVersion != ""
}

func WriteLock(root, frameworkVersion string) error {
	body := fmt.Sprintf("# Which framework version this repository adopted.\n"+
		"# Written by `mf init`; compared by `mf upgrade`.\nversion = 1\nframework_version = %q\n", frameworkVersion)
	return os.WriteFile(LockPath(root), []byte(body), 0o644)
}

// Step is one thing init did or declined to do.
type Step struct {
	Name    string
	Changed bool
	Message string
}

type InitOptions struct {
	RepoRoot         string
	FrameworkVersion string
}

// Init applies the local activation state. It is idempotent, and it never
// overwrites a project file a person already wrote: scaffolding over someone's
// policy is a data loss disguised as a convenience.
func Init(o InitOptions) ([]Step, error) {
	var steps []Step

	projectPath := filepath.Join(o.RepoRoot, config.ProjectFileName)
	if _, err := os.Stat(projectPath); err == nil {
		steps = append(steps, Step{Name: "project file", Message: config.ProjectFileName + " already exists; left untouched"})
	} else {
		if err := os.WriteFile(projectPath, []byte(scaffold), 0o644); err != nil {
			return steps, err
		}
		steps = append(steps, Step{Name: "project file", Changed: true, Message: "wrote " + config.ProjectFileName})
	}

	state := HooksStatus(o.RepoRoot)
	switch {
	case state.Canonical:
		steps = append(steps, Step{Name: "hooks", Message: "core.hooksPath already points at " + HooksDir})
	case !state.Present:
		steps = append(steps, Step{Name: "hooks", Message: "no " + HooksDir + " directory here; nothing to wire"})
	default:
		if err := InstallHooks(o.RepoRoot); err != nil {
			return steps, err
		}
		steps = append(steps, Step{Name: "hooks", Changed: true, Message: "core.hooksPath -> " + HooksDir})
	}

	if lock, ok := ReadLock(o.RepoRoot); ok && lock.FrameworkVersion == o.FrameworkVersion {
		steps = append(steps, Step{Name: "lock", Message: LockFileName + " already records " + o.FrameworkVersion})
	} else {
		if err := WriteLock(o.RepoRoot, o.FrameworkVersion); err != nil {
			return steps, err
		}
		steps = append(steps, Step{Name: "lock", Changed: true, Message: "recorded framework version " + o.FrameworkVersion})
	}

	return steps, nil
}

const scaffold = `# Development-standards policy for this repository.
#
# Committed on purpose: this carries policy, so it travels with the repository.
# It must never carry an endpoint, an API key, or the name of the variable
# holding one — those are machine state and the loader refuses them here.
#
# A project names providers; only a machine defines how to reach them.

version = 1

[review]
base = "main"
effort = "high"

[roles.r1]
backends = []

[roles.r2]
backends = []
require_cross_provider = true

[roles.r3]
backends = []

[checks]
exempt_paths = ["README.md", "LICENSE", ".gitignore"]
`
