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

type Change struct {
	File   string
	Status string // "added upstream", "removed locally", "differs", "local only"
}

type Report struct {
	LockedVersion  string
	RunningVersion string
	Changes        []Change
	Note           string
}

// Compare reads the local standards tree and the embedded one.
func Compare(root, runningVersion string) (Report, error) {
	rep := Report{RunningVersion: runningVersion}
	if lock, ok := activate.ReadLock(root); ok {
		rep.LockedVersion = lock.FrameworkVersion
	} else {
		rep.Note = "no " + activate.LockFileName + "; this repository has not recorded an adopted version"
	}

	embedded := map[string]string{}
	err := fs.WalkDir(framework.Standards, framework.StandardsPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := framework.Standards.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		embedded[filepath.Base(path)] = normalize(string(body))
		return nil
	})
	if err != nil {
		return rep, err
	}

	localDir := filepath.Join(root, filepath.FromSlash(framework.StandardsPrefix))
	local := map[string]string{}
	entries, err := os.ReadDir(localDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return rep, err
		}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(localDir, e.Name()))
		if readErr != nil {
			continue
		}
		local[e.Name()] = normalize(string(body))
	}

	for name, want := range embedded {
		got, present := local[name]
		switch {
		case !present:
			rep.Changes = append(rep.Changes, Change{File: name, Status: "missing locally"})
		case got != want:
			rep.Changes = append(rep.Changes, Change{File: name, Status: "differs from this build"})
		}
	}
	for name := range local {
		if _, present := embedded[name]; !present {
			rep.Changes = append(rep.Changes, Change{File: name, Status: "local only"})
		}
	}
	sort.Slice(rep.Changes, func(i, j int) bool { return rep.Changes[i].File < rep.Changes[j].File })
	return rep, nil
}

// normalize removes the line-ending difference a Windows checkout introduces,
// so a clone is not reported as diverging from the build it matches.
func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func (r Report) Summary() string {
	if len(r.Changes) == 0 {
		return fmt.Sprintf("standards match this build (%s)", r.RunningVersion)
	}
	return fmt.Sprintf("%d standards file(s) differ from this build (%s)", len(r.Changes), r.RunningVersion)
}
