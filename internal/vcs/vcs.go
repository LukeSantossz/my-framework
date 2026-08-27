// Package vcs is the git plumbing the runner needs: resolving refs, producing a
// bounded diff, and reading the per-branch Author Declaration.
package vcs

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type Repo struct {
	Root string
}

func Open(root string) *Repo { return &Repo{Root: root} }

func (r *Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// IndexIsExecutable reports whether the index records a path as executable.
//
// The index rather than the filesystem, because that is what a clone gets. On a
// checkout with core.fileMode false — the Windows default — the mode on disk is
// not read at all, so a file written 0755 is staged 0644 and every other
// platform receives it non-executable.
func (r *Repo) IndexIsExecutable(path string) (bool, bool) {
	out, err := r.git("ls-files", "--stage", "--", path)
	if err != nil {
		return false, false
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return false, false
	}
	return fields[0] == "100755", true
}

// MarkIndexExecutable records a path as executable in the index, adding it if
// the index does not carry it yet.
//
// git will not run a hook the checkout leaves non-executable, so a repository
// adopted on Windows shipped both gates switched off to everyone else — the
// "reports a wired gate and has none" failure these hooks exist to end, reached
// through the one mechanism the hooks themselves cannot see.
func (r *Repo) MarkIndexExecutable(path string) error {
	_, err := r.git("update-index", "--add", "--chmod=+x", "--", path)
	return err
}

// Resolves reports whether a ref exists in this repository.
func (r *Repo) Resolves(ref string) bool {
	_, err := r.git("rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func (r *Repo) CurrentBranch() (string, error) {
	out, err := r.git("rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

// Diff is a bounded view of a change.
type Diff struct {
	Text      string
	Truncated bool
	Empty     bool
}

// Diff returns the change of head against base, capped at maxBytes.
//
// A ref that does not resolve is an error rather than an empty diff. git prints
// nothing for an unknown ref, and reading that silence as "nothing to review"
// would let the chain report a backend as having reviewed a change it never saw.
func (r *Repo) Diff(base, head string, maxBytes int) (Diff, error) {
	for _, ref := range []string{base, head} {
		if !r.Resolves(ref) {
			return Diff{}, fmt.Errorf("ref %q does not resolve in this repository; cannot build a diff to review", ref)
		}
	}
	out, err := r.git("diff", base+"..."+head)
	if err != nil {
		return Diff{}, err
	}
	if strings.TrimSpace(out) == "" {
		return Diff{Empty: true}, nil
	}
	if maxBytes > 0 && len(out) > maxBytes {
		return Diff{Text: out[:maxBytes], Truncated: true}, nil
	}
	return Diff{Text: out}, nil
}

func (r *Repo) ChangedFiles(base, head string) ([]string, error) {
	for _, ref := range []string{base, head} {
		if !r.Resolves(ref) {
			return nil, fmt.Errorf("ref %q does not resolve in this repository", ref)
		}
	}
	out, err := r.git("diff", "--name-only", base+"..."+head)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// Declaration is who authored a change: the provider and model, as recorded
// while the change was being made.
type Declaration struct {
	Provider string
	Model    string
}

// The declaration lives in the repository-local git config, keyed by branch. It
// is deliberately not committed: it records what happened on this clone, and a
// value that travelled with the branch would assert something about sessions it
// never saw.
func declKey(branch, field string) string {
	return fmt.Sprintf("branch.%s.mfAuthor%s", branch, field)
}

// AuthorDeclaration reads the declaration for a branch. Absence is a normal
// answer, not an error: it is what makes the cross-provider state `unknown`.
func (r *Repo) AuthorDeclaration(branch string) (Declaration, bool) {
	provider, err := r.git("config", "--local", "--get", declKey(branch, "Provider"))
	if err != nil {
		return Declaration{}, false
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return Declaration{}, false
	}
	model, _ := r.git("config", "--local", "--get", declKey(branch, "Model"))
	return Declaration{Provider: provider, Model: strings.TrimSpace(model)}, true
}

func (r *Repo) SetAuthorDeclaration(branch string, d Declaration) error {
	if _, err := r.git("config", "--local", declKey(branch, "Provider"), d.Provider); err != nil {
		return err
	}
	if d.Model == "" {
		// A declaration that names no model must not inherit the one before
		// it. Re-declaring a different provider kept the old model, so
		// `mf doctor` and R2's cross-provider note reported a model that
		// provider never ran — a record that reads as fact and is not one.
		// An unset key that was never set is not an error here.
		_, _ = r.git("config", "--local", "--unset", declKey(branch, "Model"))
		return nil
	}
	if _, err := r.git("config", "--local", declKey(branch, "Model"), d.Model); err != nil {
		return err
	}
	return nil
}

// Commit is one commit's message, split the way Conventional Commits reads it.
type Commit struct {
	SHA     string
	Subject string
	Body    string

	// Parents is how many commits this one sits on top of. It is carried
	// because a caller checking message conventions has to be able to tell a
	// subject an author wrote from one git or a forge generated: a merge has
	// two or more parents, and nothing in its text reliably says so.
	Parents int
}

// Merge reports whether this commit joins two histories rather than adding to
// one.
func (c Commit) Merge() bool { return c.Parents > 1 }

// Commits lists what head adds over base, newest last.
func (r *Repo) Commits(base, head string) ([]Commit, error) {
	for _, ref := range []string{base, head} {
		if !r.Resolves(ref) {
			return nil, fmt.Errorf("ref %q does not resolve in this repository", ref)
		}
	}
	// A record separator no commit message will contain, so a body with blank
	// lines cannot be mistaken for the end of an entry.
	const sep = "\x1e"
	out, err := r.git("log", "--reverse", "--format=%H%x1f%P%x1f%s%x1f%b"+sep, base+".."+head)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, entry := range strings.Split(out, sep) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "\x1f", 4)
		if len(parts) < 3 {
			continue
		}
		c := Commit{SHA: parts[0], Parents: len(strings.Fields(parts[1])), Subject: parts[2]}
		if len(parts) == 4 {
			c.Body = parts[3]
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// PathsEverAdded lists every path git history records as having been added
// under the given directories, whether or not it is still there. Paths come
// back as git names them — slash-separated and relative to the repository root
// — sorted and without duplicates.
//
// It exists because absence is invisible to anything that reads the working
// tree: a durable record deleted rather than retired in place leaves nothing
// behind to notice, and the archive it belonged to still looks complete. Only
// history remembers that it was ever there.
//
// It answers about the history this clone has. A shallow clone reports only
// what its window reaches, so a caller that treats the answer as complete
// wants a full checkout — which is why the workflow running these gates fetches
// with depth 0.
func (r *Repo) PathsEverAdded(dirs ...string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}
	args := append([]string{"log", "--diff-filter=A", "--name-only", "--format=", "--"}, dirs...)
	out, err := r.git(args...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		paths = append(paths, line)
	}
	sort.Strings(paths)
	return paths, nil
}

// RenamedPaths maps each path git history records as renamed, under the given
// directories, to the name that commit gave it. Paths come back as git names
// them, slash-separated and relative to the repository root.
//
// It is the companion PathsEverAdded needs to be read correctly. git detects
// renames by default, so the commit that renames a file reports it as R and the
// new name never appears as an addition: a caller comparing added paths against
// the working tree sees the old name missing, with nothing saying where it went,
// and reports a file as deleted that is sitting there under another name.
//
// One step per entry, and the caller walks the chain. A file renamed twice has
// two records here, and only the caller knows whether it wants the end of the
// chain or each stage of it. A chain can also loop — a name given back to an
// earlier file — so a caller that walks it needs to remember where it has been.
// RecordFilesOnOtherRefs maps each filename that some other ref's tree holds
// directly under dir to the name of a ref holding it. dir is
// repository-relative and slash-separated.
//
// It answers the one question contiguity cannot: whether a record number
// missing from this branch is a hole or a claim somebody still has open.
// Numbers are taken when a record is written, so two changes open at once means
// one branch holds NNNN while another writes NNNN+1, and the second has a gap
// it did not make.
//
// Three things it deliberately does not count. The branch at HEAD: its tree is
// what this working tree came from, so counting it would let a record deleted
// and not yet committed excuse the gap it just made. History: it would excuse a
// gap forever on the strength of a branch that was abandoned and deleted, while
// a tree is a claim someone still has. And anything below dir: the caller
// decides what a record filename looks like, and a draft in a subdirectory is
// not a record.
//
// Local refs only — a gate that reached the network would fail differently on a
// machine that happens to be offline.
func (r *Repo) RecordFilesOnOtherRefs(dir string) (map[string]string, error) {
	if dir == "" {
		return nil, nil
	}
	out, err := r.git("for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	current := ""
	if head, err := r.git("symbolic-ref", "--quiet", "HEAD"); err == nil {
		current = strings.TrimSpace(head)
	}
	held := map[string]string{}
	for _, ref := range strings.Split(out, "\n") {
		if ref = strings.TrimSpace(ref); ref == "" || ref == current {
			continue
		}
		// Not recursive: only what sits directly under dir. A ref whose tree
		// has no such directory is not an error worth stopping for — it is a
		// branch that predates the archive.
		names, err := r.git("ls-tree", "--name-only", ref, "--", dir+"/")
		if err != nil {
			continue
		}
		for _, path := range strings.Split(names, "\n") {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			base := path
			if i := strings.LastIndex(base, "/"); i >= 0 {
				base = base[i+1:]
			}
			if _, seen := held[base]; !seen {
				held[base] = ref
			}
		}
	}
	return held, nil
}

func (r *Repo) RenamedPaths(dirs ...string) (map[string]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}
	args := append([]string{"log", "--diff-filter=R", "--name-status", "--format=", "--"}, dirs...)
	out, err := r.git(args...)
	if err != nil {
		return nil, err
	}
	renames := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) != 3 || !strings.HasPrefix(fields[0], "R") {
			continue
		}
		from, to := strings.TrimSpace(fields[1]), strings.TrimSpace(fields[2])
		if from == "" || to == "" {
			continue
		}
		// git logs newest first, so the first record of a name is the most
		// recent thing that happened to it. A name reused by a later file would
		// otherwise be overwritten by the older rename and send a caller
		// walking the chain into history that no longer applies.
		if _, seen := renames[from]; seen {
			continue
		}
		renames[from] = to
	}
	return renames, nil
}

// ObjectID resolves an object name — `HEAD:docs/specs/0001-a.md`, a tag, a
// commit — to the id of the object it names.
//
// Comparing two ids is how a caller asks whether two files are byte-identical
// without reading either: git stores content addressed by hash, and the hash is
// taken after the index's line-ending normalisation, so a Windows checkout and
// a Linux one agree about a file they both hold. A name that does not resolve
// is an error rather than an empty id, because an empty id would compare equal
// to another empty one and report two absent files as a match.
func (r *Repo) ObjectID(object string) (string, error) {
	out, err := r.git("rev-parse", "--verify", "--quiet", object)
	if err != nil {
		return "", fmt.Errorf("object %q does not resolve in this repository", object)
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", fmt.Errorf("object %q does not resolve in this repository", object)
	}
	return id, nil
}

// ConfigGet reads the value in effect, from whichever scope defines it.
// Absence is not an error.
func (r *Repo) ConfigGet(key string) (string, error) {
	return r.configValue("config", "--get", key)
}

// ConfigGetLocal reads only what this repository's own configuration says.
//
// It is a separate call rather than a flag on ConfigGet because the difference
// between the two is a difference in meaning, not in scope: a value inherited
// from a user's global configuration applies to every repository on the machine
// and travels with none of them, so reporting it as this repository's own
// decision makes every clone look activated. Absence is not an error.
func (r *Repo) ConfigGetLocal(key string) (string, error) {
	return r.configValue("config", "--local", "--get", key)
}

// configValue is what makes both readers' absence contract true.
//
// `git config --get` exits 1 for a key nobody set, which reaches here as a
// command failure indistinguishable from a broken configuration file. Both
// readers documented absence as a normal answer and returned that failure
// anyway, so every caller had to know the documentation was wrong and treat a
// non-nil error as "unset" — which is also how a genuine failure would have
// been read as an unset key, silently.
//
// Exit 1 is the only status git uses for a key it could not read as set, so
// every other status still surfaces. A malformed key shares that status; the
// keys here are literals in this package's callers, so the cost is a typo
// resolving as absent rather than as an error, and the alternative is matching
// git's message text.
func (r *Repo) configValue(args ...string) (string, error) {
	out, err := r.git(args...)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	return strings.TrimSpace(out), err
}

// GitDir resolves the repository's git directory, which may be relative to the
// root. It asks git rather than joining onto `.git`, because a worktree and a
// submodule both keep theirs somewhere else and a hardcoded join reports on a
// directory that is not the one in use.
//
// Deliberately not `rev-parse --git-path hooks`: git special-cases that name
// and answers with core.hooksPath, so asking it where the hooks are returns the
// directory that replaced them rather than the one that was replaced.
func (r *Repo) GitDir() (string, error) {
	out, err := r.git("rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (r *Repo) ConfigSetLocal(key, value string) error {
	_, err := r.git("config", "--local", key, value)
	return err
}

func (r *Repo) ConfigUnsetLocal(key string) error {
	_, err := r.git("config", "--local", "--unset", key)
	return err
}
