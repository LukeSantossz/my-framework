// Package vcs is the git plumbing the runner needs: resolving refs, producing a
// bounded diff, and reading the per-branch Author Declaration.
package vcs

import (
	"bytes"
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
	if d.Model != "" {
		if _, err := r.git("config", "--local", declKey(branch, "Model"), d.Model); err != nil {
			return err
		}
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
	out, err := r.git("config", "--get", key)
	return strings.TrimSpace(out), err
}

// ConfigGetLocal reads only what this repository's own configuration says.
//
// It is a separate call rather than a flag on ConfigGet because the difference
// between the two is a difference in meaning, not in scope: a value inherited
// from a user's global configuration applies to every repository on the machine
// and travels with none of them, so reporting it as this repository's own
// decision makes every clone look activated. Absence is not an error.
func (r *Repo) ConfigGetLocal(key string) (string, error) {
	out, err := r.git("config", "--local", "--get", key)
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
