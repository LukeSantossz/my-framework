package main

// Guards the job that publishes the sec-free edition on each release (spec
// 0052). Like the publish guard beside it, it reads the workflow rather than
// running it: the behaviour lives in GitHub Actions, and what can be asserted
// here is that the properties the edition depends on are still in the file.

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func releaseWorkflow(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading %s: %v", p, err)
	}
	return strings.ReplaceAll(string(body), "\r\n", "\n")
}

// editionJob is the text of the `edition:` job, up to the next top-level job.
func editionJob(t *testing.T) string {
	t.Helper()
	text := releaseWorkflow(t)
	i := strings.Index(text, "\n  edition:\n")
	if i < 0 {
		t.Fatal("release.yml has no `edition` job; the sec-free edition is never published")
	}
	rest := text[i+1:]
	if j := regexp.MustCompile(`\n  [a-z-]+:\n`).FindStringIndex(rest[len("  edition:\n"):]); j != nil {
		rest = rest[:len("  edition:\n")+j[0]]
	}
	return rest
}

func TestTheEditionTagNeverMatchesTheReleaseTrigger(t *testing.T) {
	// A pushed edition tag that matched the trigger would start a release from
	// the edition branch, which holds no Go code to build.
	text := releaseWorkflow(t)
	trigger := regexp.MustCompile(`tags: \["([^"]+)"\]`).FindStringSubmatch(text)
	if trigger == nil {
		t.Fatal("release.yml no longer declares its tag trigger in the form this guard reads")
	}
	job := editionJob(t)
	if !strings.Contains(job, `"secfree-${TAG}"`) {
		t.Fatal("the edition job no longer names its tag secfree-${TAG}")
	}
	if ok, _ := path.Match(trigger[1], "secfree-v1.2.3"); ok {
		t.Errorf("the release trigger %q matches the edition tag secfree-v1.2.3", trigger[1])
	}
}

func TestTheEditionJobCommitsTheTaggedBuildOntoTheOrphanSecfreeBranch(t *testing.T) {
	job := editionJob(t)
	for _, want := range []string{
		"needs: build",
		"edition build secfree --out",
		"commit-tree",
		"refs/heads/secfree",
		"refs/tags/${edition_tag}",
		"contents: write",
	} {
		if !strings.Contains(job, want) {
			t.Errorf("the edition job lacks %q", want)
		}
	}
}

func TestTheEditionJobIsSafeToRerun(t *testing.T) {
	// A re-run after a published edition must stop rather than push a second
	// commit for the same release.
	job := editionJob(t)
	if !strings.Contains(job, "ls-remote --exit-code --tags") {
		t.Error("the edition job does not check whether this release's edition tag already exists")
	}
}
