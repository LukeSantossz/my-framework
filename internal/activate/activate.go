// Package activate turns the framework's documented activation steps into
// commands: wiring the versioned hooks, recording which framework version a
// repository adopted, and putting in place what an adopting repository cannot
// obtain any other way — the project policy file, the standards this build
// carries, and the source the vendor instruction files are generated from.
//
// Nothing here repairs what it reports, and nothing overwrites a value a person
// set. Activation that silently rewrites local decisions is how a tool loses a
// user. That rule is what decides every close call in this file: a hooks path
// another tool owns is left alone rather than replaced, a standard the adopter
// edited is left alone rather than restored, and an adopted version is left
// alone by a build that has no released version of its own to record.
package activate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	framework "github.com/LukeSantossz/my-framework"
	"github.com/LukeSantossz/my-framework/internal/agents"
	"github.com/LukeSantossz/my-framework/internal/config"
	"github.com/LukeSantossz/my-framework/internal/vcs"
	"github.com/LukeSantossz/my-framework/internal/version"
)

// HooksDir is the versioned directory the hooks path must point at. It lives in
// the repository rather than in .git/ so the hook itself is reviewable.
const HooksDir = ".githooks"

// LockFileName records which framework version a repository adopted.
const LockFileName = ".framework.lock"

// AgentSourcePath is where the marked-up instruction source belongs. It is
// agents.SourcePath rather than a second literal, because a scaffold that wrote
// the file anywhere else would produce a repository `mf agents sync` cannot read.
const AgentSourcePath = agents.SourcePath

// HooksState is what git currently says about the hooks path.
//
// Path and Local are separate facts, and conflating them is what let a single
// global setting make every repository on a machine report itself activated.
// Path is the value in effect, from whichever scope defines it; Local says the
// repository's own configuration is where it came from. Only a local value
// travels with the repository, so only a local value is a decision this
// repository made.
type HooksState struct {
	Path      string
	Local     bool
	Canonical bool
	Present   bool
}

// HooksStatus reports without changing anything.
func HooksStatus(root string) HooksState {
	repo := vcs.Open(root)
	value, err := repo.ConfigGet("core.hooksPath")
	state := HooksState{Path: strings.TrimSpace(value)}
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
	if local, localErr := repo.ConfigGetLocal("core.hooksPath"); localErr == nil && local != "" {
		state.Local = true
	}
	state.Canonical = filepath.ToSlash(state.Path) == HooksDir
	return state
}

// InstallHooks points git at the versioned directory. Idempotent.
//
// A hooks path this repository already set and mf did not is left exactly as it
// is. A repository using Husky, lefthook or a hand-written directory has one
// gate, and git honours one core.hooksPath: replacing it does not add this
// framework's hook beside theirs, it switches theirs off. That is the loss this
// package's own rule forbids, and git reports nothing when it happens.
func InstallHooks(root string) error {
	if _, err := os.Stat(filepath.Join(root, HooksDir)); err != nil {
		return fmt.Errorf("no %s directory in this repository, so there is nothing to wire; "+
			"`mf init` writes the versioned hooks this build carries", HooksDir)
	}
	state := HooksStatus(root)
	if state.Local && !state.Canonical {
		return fmt.Errorf("core.hooksPath is already set in this repository to %q, and mf did not set it; "+
			"git honours one hooks path, so wiring %s would switch that one off. "+
			"Remove it yourself (`git config --local --unset core.hooksPath`) and run `mf hooks install` again",
			state.Path, HooksDir)
	}
	return vcs.Open(root).ConfigSetLocal("core.hooksPath", HooksDir)
}

// UninstallHooks removes the wiring this framework installed, leaving the
// versioned directory alone. Idempotent, and it removes only what mf owns.
//
// Ownership is read off the value rather than a marker file: InstallHooks
// refuses to replace a path someone else set, so the only local value mf can
// have produced is one naming its own versioned directory, and there is
// consequently never a displaced value to restore. A record would document a
// replacement that cannot happen.
//
// The absent case is a success, not an error. `git config --unset` exits 5 for
// a key that is not there, which surfaced as raw plumbing naming no remedy and
// broke every teardown script on its second run.
func UninstallHooks(root string) error {
	state := HooksStatus(root)
	if !state.Local {
		return nil
	}
	if !state.Canonical {
		return fmt.Errorf("core.hooksPath here is %q, which mf did not set; leaving it alone. "+
			"To remove it yourself: `git config --local --unset core.hooksPath`", state.Path)
	}
	return vcs.Open(root).ConfigUnsetLocal("core.hooksPath")
}

// ShadowedLocalHooks lists the hooks in this repository's git directory that a
// core.hooksPath makes git ignore.
//
// Pointing core.hooksPath anywhere replaces .git/hooks wholesale rather than
// adding to it, so a hook a person installed there — by hand, or by a tool that
// writes into .git/hooks — stops firing the moment this framework is wired, and
// git says nothing. The `.sample` files git ships are excluded: they never ran,
// so naming them would bury the ones that did.
func ShadowedLocalHooks(root string) []string {
	gitDir, err := vcs.Open(root).GitDir()
	if err != nil {
		return nil
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	entries, err := os.ReadDir(filepath.Join(gitDir, "hooks"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".sample") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// PinnedModel records which model id a backend resolved to, and when. It turns
// a vendor retiring or silently re-pointing an id from a mystery into a
// reported difference.
type PinnedModel struct {
	Model    string `toml:"model"`
	PinnedOn string `toml:"pinned_on"`
}

// Lock is the adopted-version record.
type Lock struct {
	Version          int                    `toml:"version"`
	FrameworkVersion string                 `toml:"framework_version"`
	Models           map[string]PinnedModel `toml:"models"`
}

func LockPath(root string) string { return filepath.Join(root, LockFileName) }

func ReadLock(root string) (Lock, bool) {
	data, err := os.ReadFile(LockPath(root))
	if err != nil {
		return Lock{}, false
	}
	lock := Lock{Version: 1}
	if _, err := toml.Decode(string(data), &lock); err != nil {
		return Lock{}, false
	}
	// Existence is keyed on the file parsing, not on any one field. `mf models
	// pin` before `mf init` writes a lock that legitimately carries pins and no
	// adopted version, and reporting that as "no lock" loses the pins.
	return lock, true
}

const lockHeader = "# What this repository adopted, and what its reviewers resolved to.\n" +
	"# Written by `mf init` and `mf models pin`; compared by `mf upgrade` and `mf doctor`.\n" +
	"# A pinned model that no longer matches the configuration is reported, never\n" +
	"# silently corrected: which side is right is a decision, not a default.\n"

// WriteLock persists the record, preserving whatever it already held, and
// reports whether the adopted version it now carries is the one passed in. A
// command that writes one field must not drop the others.
//
// A build with no released identity of its own does not replace an adoption
// somebody recorded. `go build` from a source tree, and `go run`, report the
// unreleased default; running `mf init` from one used to rewrite "this
// repository adopted v0.4.0" into "0.0.0-dev", which is a value nobody chose
// and nothing can be compared against. The rule is this package's own: an
// unreleased build has nothing to record, so it records nothing and says so.
func WriteLock(root, frameworkVersion string) (recorded bool, err error) {
	lock, _ := ReadLock(root)
	lock.Version = 1
	recorded = frameworkVersion != "" &&
		(lock.FrameworkVersion == "" || frameworkVersion != version.Dev)
	if recorded {
		lock.FrameworkVersion = frameworkVersion
	}
	return recorded, writeLockFile(root, lock)
}

// PinModels records the model ids the configuration currently resolves to.
func PinModels(root string, resolved map[string]string, today string) (Lock, error) {
	lock, _ := ReadLock(root)
	lock.Version = 1
	if lock.Models == nil {
		lock.Models = map[string]PinnedModel{}
	}
	for backend, model := range resolved {
		if model == "" {
			continue
		}
		lock.Models[backend] = PinnedModel{Model: model, PinnedOn: today}
	}
	return lock, writeLockFile(root, lock)
}

// NotConfigured is what Configured carries for a pin whose backend no
// configuration layer resolves any more.
//
// The sentinel is in the value rather than beside it as a flag because every
// caller renders Configured verbatim into a sentence, and a pin that cannot be
// checked must not be able to render as a blank where a model id belongs.
const NotConfigured = "(not configured)"

// ModelDrift reports where the configuration and the pin disagree.
type ModelDrift struct {
	Backend    string
	Pinned     string
	Configured string
	PinnedOn   string
}

// ComparePins reports every pin the configuration does not confirm.
//
// A pin for a backend nothing resolves is reported rather than skipped. It is
// the case a deleted or renamed backend produces, and skipping it counted the
// pin among those "all matching the configuration" — a pin nothing checked,
// reported as a pin that checks out, which is the one answer a comparison must
// never give.
func ComparePins(lock Lock, resolved map[string]string) []ModelDrift {
	var drift []ModelDrift
	names := make([]string, 0, len(lock.Models))
	for name := range lock.Models {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pin := lock.Models[name]
		current, present := resolved[name]
		if present && current == pin.Model {
			continue
		}
		if !present {
			current = NotConfigured
		}
		drift = append(drift, ModelDrift{
			Backend: name, Pinned: pin.Model, Configured: current, PinnedOn: pin.PinnedOn,
		})
	}
	return drift
}

func writeLockFile(root string, lock Lock) error {
	var buf bytes.Buffer
	buf.WriteString(lockHeader)
	if err := toml.NewEncoder(&buf).Encode(lock); err != nil {
		return err
	}
	return os.WriteFile(LockPath(root), buf.Bytes(), 0o644)
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

	// StandardsDir is where this repository keeps the documents the gates read,
	// as configured. Empty takes the layout this framework ships with. It is a
	// parameter because a repository that vendors these standards as a submodule
	// keeps them somewhere else, and materialising a second copy under
	// `docs/standards` there would give that repository two corpora to drift.
	StandardsDir string

	// R2Backend is the reviewer the adopter named at activation, if any. The
	// scaffold declares it in the R2 chain, because a chain is policy while the
	// route to it is machine state, and writing only the route would leave a
	// backend nothing reaches for. Empty leaves the chain empty, which is the
	// honest state for a repository that has not chosen one.
	R2Backend string

	// StandardsSubmodule is the submodule that supplies the corpus, when one
	// does. Empty leaves the scaffold as shipped.
	//
	// It is separate from StandardsDir because the two answer different
	// questions: StandardsDir says where to read, and this says that the
	// layout has to be written down. The commented `[paths]` block the
	// scaffold ships changes nothing until an adopter edits it, and a
	// generated instruction file naming `docs/standards/` resolves to nothing
	// in this layout — so a repository init can see is vendoring is one it
	// must configure rather than describe.
	StandardsSubmodule string

	// AgentsSourceDir is the directory holding the document the vendor
	// instruction files are generated from, as configured. Empty takes the
	// layout this framework ships with. Materialising it anywhere else would
	// write a source the configuration does not name, which init then cannot
	// generate from — it wrote one file and read another.
	AgentsSourceDir string
}

// Init applies the local activation state.
//
// It is idempotent, and it never overwrites a file a person already wrote:
// scaffolding over someone's policy — or over a standard they adopted and then
// edited, which is the point of adopting them — is a data loss disguised as a
// convenience. Every step therefore reports what it left alone as plainly as
// what it wrote.
//
// What it writes is what an adopting repository cannot obtain any other way:
// the policy file, the standards corpus this build carries, and the source the
// vendor instruction files are generated from. Reporting success while leaving
// a repository with no standards and no instruction source made the documented
// adoption path produce a repository whose gates read empty directories.
func Init(o InitOptions) ([]Step, error) {
	var steps []Step

	projectPath := filepath.Join(o.RepoRoot, config.ProjectFileName)
	if _, err := os.Stat(projectPath); err == nil {
		steps = append(steps, Step{Name: "project file", Message: config.ProjectFileName + " already exists; left untouched"})
	} else {
		body := scaffold
		if o.StandardsSubmodule != "" {
			body = vendoredScaffold(body, o.StandardsSubmodule)
		}
		if o.R2Backend != "" {
			body = strings.Replace(body, "[roles.r2]\nbackends = []",
				fmt.Sprintf("[roles.r2]\nbackends = [%q]", o.R2Backend), 1)
		}
		if err := os.WriteFile(projectPath, []byte(body), 0o644); err != nil {
			return steps, err
		}
		steps = append(steps, Step{Name: "project file", Changed: true, Message: "wrote " + config.ProjectFileName})
	}

	standardsDir := o.StandardsDir
	if standardsDir == "" {
		standardsDir = framework.StandardsPrefix
	}
	standardsStep, err := writeStandards(o.RepoRoot, standardsDir)
	if err != nil {
		return steps, err
	}
	steps = append(steps, standardsStep)

	// The agent tree is not relocatable the way the standards are: `mf agents
	// sync` reads its source from one path, so a copy anywhere else is one
	// nothing generates from.
	agentsDir := o.AgentsSourceDir
	if agentsDir == "" {
		agentsDir = framework.AgentDocsPrefix
	}
	sourceStep, err := writeAgentSource(o.RepoRoot, agentsDir)
	if err != nil {
		return steps, err
	}
	steps = append(steps, sourceStep)

	// The hooks are written before the wiring step reads the directory, because
	// pointing core.hooksPath at an empty directory is the state that let a
	// repository report an activated gate and have none.
	hookStep, err := materialiseHooks(o.RepoRoot)
	if err != nil {
		return steps, err
	}
	steps = append(steps, hookStep)

	state := HooksStatus(o.RepoRoot)
	switch {
	case !state.Present:
		// Said as a state of the repository rather than as a note about a
		// setting: with no versioned directory there is no pre-push gate here at
		// all, and the previous wording read as an aside.
		steps = append(steps, Step{Name: "hooks", Message: "no " + HooksDir + " directory here, so this repository has no push gate; " +
			"add one and run `mf hooks install`"})
	case state.Canonical && state.Local:
		steps = append(steps, Step{Name: "hooks", Message: "core.hooksPath already points at " + HooksDir})
	case state.Local:
		// Another tool owns the gate. Replacing it would switch theirs off, so
		// the decision is handed back rather than made here.
		steps = append(steps, Step{Name: "hooks", Message: "core.hooksPath is set to " + state.Path + " here and mf did not set it; " +
			"left untouched — git honours one hooks path, so removing that one is your decision"})
	case state.Path != "" && !state.Canonical:
		// The value came from outside this repository, so it is not a decision
		// this repository made and wiring here is right. But git honours one
		// hooks path, and the local value about to be written switches the
		// inherited one off in this repository — silently, unless it is said
		// here. This is the one replacement the package's rule allows, and
		// naming it is what keeps it from being a loss.
		//
		// An inherited value that already names the versioned directory falls
		// through to the wiring below instead: nothing is switched off, and
		// saying otherwise would report a loss that did not happen.
		if err := InstallHooks(o.RepoRoot); err != nil {
			return steps, err
		}
		steps = append(steps, Step{Name: "hooks", Changed: true, Message: "core.hooksPath -> " + HooksDir +
			"; a setting outside this repository pointed at " + state.Path + ", which no longer applies here"})
	default:
		if err := InstallHooks(o.RepoRoot); err != nil {
			return steps, err
		}
		steps = append(steps, Step{Name: "hooks", Changed: true, Message: "core.hooksPath -> " + HooksDir})
	}
	if shadowed := ShadowedLocalHooks(o.RepoRoot); len(shadowed) > 0 && HooksStatus(o.RepoRoot).Path != "" {
		steps = append(steps, Step{Name: "local hooks", Message: "core.hooksPath replaces .git/hooks rather than adding to it, so " +
			strings.Join(shadowed, ", ") + " no longer runs"})
	}

	before, _ := ReadLock(o.RepoRoot)
	recorded, err := WriteLock(o.RepoRoot, o.FrameworkVersion)
	if err != nil {
		return steps, err
	}
	switch {
	case before.FrameworkVersion == o.FrameworkVersion:
		steps = append(steps, Step{Name: "lock", Message: LockFileName + " already records " + o.FrameworkVersion})
	case recorded:
		steps = append(steps, Step{Name: "lock", Changed: true, Message: "recorded framework version " + o.FrameworkVersion})
	default:
		steps = append(steps, Step{Name: "lock", Message: LockFileName + " records " + before.FrameworkVersion +
			"; this build is " + o.FrameworkVersion + " and has no released version to record over it"})
	}

	return steps, nil
}

// writeAgentSource materialises the instruction source, unless the configured
// location is inside a submodule.
//
// Same rule as the standards, for the same reason and one level down: a
// submodule that supplies the corpus supplies the source beside it, so writing
// here puts untracked files in somebody else's checkout — and it does it in a
// directory `mf agents sync` then reads, so the generated instruction files
// would be built from a copy the adopter never updates.
func writeAgentSource(root, dir string) (Step, error) {
	if sub, ok := insideSubmodule(root, dir); ok {
		return Step{Name: "agent source", Message: dir + " is inside the " + sub + " submodule, which supplies it; nothing written"}, nil
	}
	return materialise(root, framework.AgentDocs, framework.AgentDocsPrefix, dir, "agent source", "file", "")
}

// writeStandards materialises the corpus this build carries, unless the
// configured location is inside a submodule.
//
// A repository that vendors these standards as a submodule already has the
// corpus, supplied by the submodule and owned by the repository it points at.
// Writing files there would put untracked copies inside somebody else's
// checkout, and it would do it precisely when the submodule has not been
// initialised yet — the state in which the directory looks empty and the copies
// look needed. The one known downstream consumer is exactly this case.
func writeStandards(root, dir string) (Step, error) {
	if sub, ok := insideSubmodule(root, dir); ok {
		return Step{Name: "standards", Message: dir + " is inside the " + sub + " submodule, which supplies them; nothing written"}, nil
	}
	return materialise(root, framework.Standards, framework.StandardsPrefix, dir, "standards", "document",
		" (`mf upgrade` reports how they differ from this build)")
}

// submodulePath matches the `path = ...` entries in a .gitmodules file. The
// file is parsed rather than queried through git because the question is what
// the repository declares, which is committed text, and it has to be answerable
// in a clone whose submodules were never initialised.
var submodulePath = regexp.MustCompile(`(?m)^\s*path\s*=\s*(.+?)\s*$`)

func insideSubmodule(root, dir string) (string, bool) {
	body, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return "", false
	}
	target := path.Clean(filepath.ToSlash(dir))
	for _, m := range submodulePath.FindAllStringSubmatch(string(body), -1) {
		sub := path.Clean(filepath.ToSlash(m[1]))
		if target == sub || strings.HasPrefix(target, sub+"/") {
			return sub, true
		}
	}
	return "", false
}

// Vendored is a submodule this repository declares that may supply the corpus
// instead of `mf init` writing one.
type Vendored struct {
	// Dir is the submodule's path, slash-separated and relative to the root.
	Dir string
	// Populated says the checkout is there and carries a standards corpus.
	// False means the submodule is declared and not checked out, which is not
	// evidence of anything: the directory of a corpus and the directory of an
	// unrelated dependency look identical while both are empty.
	Populated bool
}

// declaredSubmodules lists what .gitmodules declares, in file order.
func declaredSubmodules(root string) []string {
	body, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return nil
	}
	var subs []string
	for _, m := range submodulePath.FindAllStringSubmatch(string(body), -1) {
		if sub := path.Clean(filepath.ToSlash(m[1])); sub != "" && sub != "." {
			subs = append(subs, sub)
		}
	}
	return subs
}

// VendoredStandards reports the submodule that already supplies this
// repository's standards, so activation does not have to write a second copy.
//
// The evidence is the corpus itself — a checkout carrying its INDEX — rather
// than the submodule's name or its remote. A name is a convention nothing
// enforces, and matching the remote against this framework's own repository
// would classify a fork, a mirror or an SSH clone as unrelated, which is
// precisely the adopter this reads for.
//
// A declared submodule that is not checked out is reported with Populated
// false rather than dropped. It cannot be classified, and the caller has to
// know that it could not: writing the shipped layout there is how an adopter
// ends up with two corpora, one of which nothing generates from.
func VendoredStandards(root string) (Vendored, bool) {
	var empty []string
	for _, sub := range declaredSubmodules(root) {
		index := filepath.Join(root, filepath.FromSlash(sub), filepath.FromSlash(framework.StandardsPrefix), "INDEX.md")
		if _, err := os.Stat(index); err == nil {
			return Vendored{Dir: sub, Populated: true}, true
		}
		if entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(sub))); err != nil || len(entries) == 0 {
			empty = append(empty, sub)
		}
	}
	if len(empty) > 0 {
		return Vendored{Dir: empty[0]}, true
	}
	return Vendored{}, false
}

// materialiseHooks writes the versioned hooks this build carries.
//
// They go in with the executable bit set, which an embedded filesystem does not
// carry: git runs a hook by executing it, so a hook copied in without the bit
// is one that never fires on any platform that has one — and it fails as an
// unrunnable file rather than as a missing gate, which is harder to read.
//
// The mode is applied on every activation, not only on the run that wrote the
// files. A hook can arrive without the bit long after it was first written — a
// checkout from an archive that carries no modes, a copy across filesystems —
// and setting it only when something was written left that hook unrunnable
// through every later `mf init`, with nothing reporting it.
//
// Only the names this build ships are touched. The versioned directory belongs
// to the repository, not to this framework: an adopter keeps their own scripts
// and notes there, and a blanket chmod over the directory would decide the mode
// of files nobody here wrote.
func materialiseHooks(root string) (Step, error) {
	step, err := materialise(root, framework.Hooks, framework.HooksPrefix, HooksDir, "hook files", "hook", "")
	if err != nil {
		return step, err
	}
	return step, fs.WalkDir(framework.Hooks, framework.HooksPrefix, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel := strings.TrimPrefix(p, framework.HooksPrefix+"/")
		dest := filepath.Join(root, HooksDir, filepath.FromSlash(rel))
		if _, statErr := os.Stat(dest); statErr != nil {
			return nil
		}
		return os.Chmod(dest, 0o755)
	})
}

// materialise copies one embedded tree into the repository, file by file,
// skipping every destination that already exists.
//
// Skipping rather than comparing is deliberate. Adopting these documents means
// editing them — that is what adopting them is for — so a second `mf init` that
// restored the shipped text would delete the adopter's work. `mf upgrade` is
// the command that reports how a local document differs from this build, and it
// applies nothing for the same reason.
//
// unchangedHint is appended when nothing was written, and only there: it names
// what a reader can do about the copies that were left alone. It is a parameter
// because only the standards have such a command — `mf upgrade` compares that
// tree and no other, so offering it for any other would name a comparison
// nothing performs.
func materialise(root string, src fs.FS, prefix, dir, stepName, noun, unchangedHint string) (Step, error) {
	target := filepath.Join(root, filepath.FromSlash(dir))

	var written, kept int
	err := fs.WalkDir(src, prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := fs.ReadFile(src, path)
		if readErr != nil {
			return readErr
		}
		rel := strings.TrimPrefix(path, prefix+"/")
		dest := filepath.Join(target, filepath.FromSlash(rel))
		if _, statErr := os.Stat(dest); statErr == nil {
			kept++
			return nil
		}
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return mkErr
		}
		if writeErr := os.WriteFile(dest, body, 0o644); writeErr != nil {
			return writeErr
		}
		written++
		return nil
	})
	if err != nil {
		return Step{Name: stepName}, err
	}

	switch {
	case written == 0:
		return Step{Name: stepName, Message: fmt.Sprintf("%d %s(s) already in %s; left untouched%s", kept, noun, dir, unchangedHint)}, nil
	case kept == 0:
		return Step{Name: stepName, Changed: true, Message: fmt.Sprintf("wrote %d %s(s) to %s", written, noun, dir)}, nil
	default:
		return Step{Name: stepName, Changed: true, Message: fmt.Sprintf("wrote %d %s(s) to %s; %d already there, left untouched", written, noun, dir, kept)}, nil
	}
}

const scaffold = `# Development-standards policy for this repository.
#
# Committed on purpose: this carries policy, so it travels with the repository.
# It must never carry an endpoint, an API key, or the name of the variable
# holding one — those are machine state and the loader refuses them here.
#
# A project names providers; only a machine defines how to reach them. Which
# provider reviews your code is your choice, so this framework names none: the
# quickest way to record one is to have said so at activation,
#
#   mf init --provider <name> --endpoint <url> --api-key-env <VAR> --model <id>
#
# which writes the route on this machine and the chain below in one step. The
# same thing by hand, for a provider added later:
#
#   mf config set providers.<name>.endpoint <url> --machine
#   mf config set providers.<name>.api_key_env <VAR> --machine
#   mf config set providers.<name>.kind openai-compatible --machine
#   mf config set backends.<name>.kind api --machine
#   mf config set backends.<name>.provider <name> --machine
#   mf config set backends.<name>.model <id> --machine
#
# <VAR> is the NAME of the environment variable holding the key, never the key.
#
# Then decide who puts it in a chain. This file does, for everyone who clones
# the repository: add the name to a role below and commit it. A machine chain
# (` + "`mf config set roles.r2.backends ... --machine`" + `) applies only to a role this
# file leaves undeclared, because a machine may not review a repository with a
# chain that repository did not choose. For one run, and only for the person
# running it: ` + "`MF_ROLES_R2_BACKENDS=<name> mf review --role r2`" + `.
#
# ` + "`mf doctor`" + ` reports which of these steps is still missing.

version = 1

# --- paths --------------------------------------------------------------------

# Where the gates look for the documents they read. The values below are the
# built-in defaults, so leaving this commented out changes nothing. Uncomment
# and edit it if this repository keeps the corpus somewhere else — consuming
# these standards as a submodule is the case it exists for. Every value is
# resolved against the repository root and may not leave it, and there is no
# machine layer for them: where a repository keeps its documents is the same
# fact on every clone, so a machine able to redirect a gate could make one
# commit pass here and fail in CI.
#
# [paths]
# standards = ".standards/docs/standards"
# specs = "docs/specs"
# adr = "docs/adr"
# agents_source = ".standards/docs/agents/instructions.md"
# agents_file = "AGENTS.md"

[review]
base = "main"
effort = "high"

# --- roles -------------------------------------------------------------------

# Every chain ships empty: this framework will not name a reviewer you have not
# configured. A role with no chain reports that it did not run, which is a
# weaker claim than a review and reads as one.

[roles.r1]
backends = []

[roles.r2]
backends = []
require_cross_provider = true

[roles.r3]
backends = []

# The CRUX explainer is a role like every other, so which model explains a
# change is configuration rather than a setting nobody remembers exists.
[roles.explain]
backends = []

# --- checks ------------------------------------------------------------------

# What counts as trivial for the Spec Gate, as an explicit path list rather than
# a heuristic or a model. It is crude on purpose: a gate nobody can predict is a
# gate people route around, and widening this list is visible in review.
[checks]
exempt_paths = ["README.md", "LICENSE", ".gitignore"]

# --- agent instruction files --------------------------------------------------

# Generated from docs/agents/instructions.md by ` + "`mf agents sync`" + `, and compared
# against it by ` + "`mf check agents`" + `. Declared here rather than left out, because a
# repository that declares no targets has an agents gate that passes forever
# while its CLAUDE.md says the gate fails when they drift apart.
#
# Roles are declared, not derived from the backend chains: the Author is a
# per-branch declaration rather than a chain, so there is nothing to derive it
# from. Delete a target whose vendor you do not use.

[agents.claude]
file = "CLAUDE.md"
roles = ["shared", "author"]

[agents.codex]
file = "AGENTS.md"
roles = ["shared", "reviewer"]
`

// vendoredVerb is the commented `[paths]` block the scaffold ships. It is
// matched whole rather than line by line: a partial rewrite would leave half a
// commented block above a live one, and the whole point of the substitution is
// that an adopter reads one answer.
const shippedPaths = `# [paths]
# standards = ".standards/docs/standards"
# specs = "docs/specs"
# adr = "docs/adr"
# agents_source = ".standards/docs/agents/instructions.md"
# agents_file = "AGENTS.md"
`

// vendoredScaffold rewrites the shipped scaffold for a repository whose
// standards come from a submodule.
//
// Two edits, both required together: the `[paths]` block stops being a comment
// and names the submodule, and every `[agents.*]` target gains the matching
// `path_prefix`. Without the first the gates read an empty `docs/standards`;
// without the second the instruction files those gates compare against point
// at that same empty directory. Writing one and not the other produces a
// repository that passes `mf check agents` while telling every agent to read
// documents that are not there.
func vendoredScaffold(body, sub string) string {
	standards := sub + "/" + framework.StandardsPrefix
	source := sub + "/" + framework.AgentDocsPrefix + "/" + path.Base(AgentSourcePath)
	live := fmt.Sprintf(`[paths]
standards = %q
specs = %q
adr = %q
agents_source = %q
agents_file = %q
`, standards, config.DefaultSpecsDir, config.DefaultADRDir, source, config.DefaultAgentsFile)
	body = strings.Replace(body, shippedPaths, live, 1)

	prefix := fmt.Sprintf(`path_prefix = %q
`, standards)
	for _, target := range []string{
		`file = "CLAUDE.md"
roles = ["shared", "author"]
`,
		`file = "AGENTS.md"
roles = ["shared", "reviewer"]
`,
	} {
		body = strings.Replace(body, target, target+prefix, 1)
	}
	return body
}
