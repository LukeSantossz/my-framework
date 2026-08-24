// Package vcs is the git plumbing the runner needs: resolving refs, producing a
// bounded diff, and reading the per-branch Author Declaration.
package vcs

import (
	"bytes"
	"fmt"
	"os/exec"
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
}

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
	out, err := r.git("log", "--reverse", "--format=%H%x1f%s%x1f%b"+sep, base+".."+head)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, entry := range strings.Split(out, sep) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "\x1f", 3)
		if len(parts) < 2 {
			continue
		}
		c := Commit{SHA: parts[0], Subject: parts[1]}
		if len(parts) == 3 {
			c.Body = parts[2]
		}
		commits = append(commits, c)
	}
	return commits, nil
}
