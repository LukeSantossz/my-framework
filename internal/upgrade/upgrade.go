// Package upgrade compares an adopter's standards against the ones the running
// binary shipped with, and reports the difference. It never applies anything:
// an adopter edits their standards — that is the point of adopting them — so
// writing a release over that destroys local intent. Reporting leaves the merge
// with the person who knows which side is right.
package upgrade

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	framework "github.com/LukeSantossz/my-framework"
	"github.com/LukeSantossz/my-framework/internal/activate"
)

// The three answers a comparison can give. They are constants rather than
// literals at the call sites because `mf doctor` reports by counting them: a
// status spelled two ways would be counted as two things.
const (
	StatusMissing   = "missing locally"
	StatusDiffers   = "differs from this build"
	StatusLocalOnly = "local only"
)

type Change struct {
	File string
	// Status is one of StatusMissing, StatusDiffers or StatusLocalOnly.
	Status string
}

type Report struct {
	LockedVersion  string
	RunningVersion string
	Changes        []Change
	Note           string
}

// Compare reads the local standards tree and the embedded one.
//
// standardsDir is where this repository keeps its documents, absolute or
// relative to root, and empty means the layout this framework ships with. It is
// a parameter rather than a constant because the one downstream consumer
// vendors this framework as a `.standards` submodule and keeps its documents
// under it; comparing that repository against `docs/standards` reports its
// entire corpus as missing while it sits complete a directory away.
func Compare(root, standardsDir, runningVersion string) (Report, error) {
	rep := Report{RunningVersion: runningVersion}
	if lock, ok := activate.ReadLock(root); ok {
		rep.LockedVersion = lock.FrameworkVersion
	} else {
		rep.Note = "no " + activate.LockFileName + "; this repository has not recorded an adopted version"
	}

	embedded, err := embeddedStandards()
	if err != nil {
		return rep, err
	}
	local, err := localStandards(localDir(root, standardsDir))
	if err != nil {
		return rep, err
	}

	for name, want := range embedded {
		got, present := local[name]
		switch {
		case !present:
			rep.Changes = append(rep.Changes, Change{File: name, Status: StatusMissing})
		case got != want:
			rep.Changes = append(rep.Changes, Change{File: name, Status: StatusDiffers})
		}
	}
	for name := range local {
		if _, present := embedded[name]; !present {
			rep.Changes = append(rep.Changes, Change{File: name, Status: StatusLocalOnly})
		}
	}
	sort.Slice(rep.Changes, func(i, j int) bool { return rep.Changes[i].File < rep.Changes[j].File })
	return rep, nil
}

// localDir resolves where to read the local tree from.
func localDir(root, standardsDir string) string {
	if standardsDir == "" {
		standardsDir = framework.StandardsPrefix
	}
	if filepath.IsAbs(standardsDir) {
		return filepath.Clean(standardsDir)
	}
	return filepath.Join(root, filepath.FromSlash(standardsDir))
}

// embeddedStandards reads the tree this build carries, keyed by the path each
// document has inside it. Both sides are keyed the same way, and by the whole
// relative path rather than the file name: keying on the name alone collapses
// two documents that differ only by directory into one entry, so one of them is
// compared against the other's content and the tree reports a difference nobody
// made.
func embeddedStandards() (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(framework.Standards, framework.StandardsPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := framework.Standards.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel := strings.TrimPrefix(path, framework.StandardsPrefix+"/")
		out[rel] = normalize(string(body))
		return nil
	})
	return out, err
}

// localStandards reads the adopter's tree, keyed to match the embedded one. It
// walks rather than listing one level, because a document in a subdirectory is
// still a document: read shallowly it is neither compared nor reported, which
// is the one answer a comparison must never give. A tree that is not there at
// all is not an error — that is exactly the state `mf upgrade` exists to
// report.
func localStandards(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		out[filepath.ToSlash(rel)] = normalize(string(body))
		return nil
	})
	return out, err
}

// normalize removes the line-ending difference a Windows checkout introduces,
// so a clone is not reported as diverging from the build it matches.
func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// VersionMismatch reports whether this repository adopted a version that is not
// the one running. Both facts were already recorded and printed side by side,
// which left the reader to notice they disagree — and whether they disagree is
// the only question the two of them together answer.
func (r Report) VersionMismatch() bool {
	return r.LockedVersion != "" && r.RunningVersion != "" && r.LockedVersion != r.RunningVersion
}

// Summary is the whole of what `mf doctor` prints about the standards, so it
// has to distinguish what `mf upgrade` distinguishes. Counting every change as
// a difference told a repository with no standards at all that its thirteen
// files "differ from this build", which is both false and, worse, hides a real
// difference among phantom ones.
func (r Report) Summary() string {
	counts := map[string]int{}
	for _, c := range r.Changes {
		counts[c.Status]++
	}
	var parts []string
	for _, s := range []struct{ status, label string }{
		{StatusMissing, "missing locally"},
		{StatusDiffers, "differing from this build"},
		{StatusLocalOnly, "local only"},
	} {
		if n := counts[s.status]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s.label))
		}
	}

	summary := fmt.Sprintf("standards match this build (%s)", r.RunningVersion)
	if len(parts) > 0 {
		summary = fmt.Sprintf("%d standards file(s), %s (%s)",
			len(r.Changes), strings.Join(parts, ", "), r.RunningVersion)
	}
	if r.VersionMismatch() {
		summary += fmt.Sprintf("; this repository adopted %s, which is not the build running", r.LockedVersion)
	}
	return summary
}
