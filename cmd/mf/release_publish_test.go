package main

// Guards the publish step of the workflow that ships this binary.
//
// It reads the workflow file rather than running it, which is a weaker test
// than any other in this repository and is here because the alternative is
// none: the behaviour lives in GitHub Actions, and the failure it guards
// against — one flaky asset upload discarding a release that passed every
// gate — has already happened once, on v0.7.1. What can be asserted locally is
// that the properties which make the step recoverable are still in it, so a
// later edit that drops one is caught by a test rather than by a tag published
// with nothing behind it.
//
// It lives beside the binary because the workflow's whole job is building and
// publishing that binary, and because a test asserting facts about the release
// belongs somewhere a reader of the release looks.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// publishStep is the `run:` body of the Publish step, as the workflow holds it.
func publishStep(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	i := strings.Index(text, "- name: Publish")
	if i < 0 {
		t.Fatalf("release.yml has no Publish step; this guard names a step that no longer exists")
	}
	return text[i:]
}

func TestPublishCreatesTheReleaseOnlyWhenItDoesNotAlreadyExist(t *testing.T) {
	// `gh release create` with the assets inline deletes the release it just
	// made if any upload fails, so the step has to be re-runnable: a second run
	// must fill the release the first one left rather than collide with it.
	step := publishStep(t)
	if !strings.Contains(step, "gh release view") {
		t.Error("the step does not check whether the release already exists, so a re-run cannot recover a half-published one")
	}
	if strings.Contains(step, `gh release create "$TAG" dist/*`) {
		t.Error("the step still creates the release with its assets inline, which deletes the release if one upload fails")
	}
}

func TestPublishVerifiesTheTagBeforeCreatingAnything(t *testing.T) {
	// A tag that does not resolve must fail here rather than produce a release
	// for a version nobody can check out.
	if !strings.Contains(publishStep(t), "--verify-tag") {
		t.Error("the step creates a release without verifying the tag resolves")
	}
}

func TestPublishUploadsEachAssetSoARerunReplacesRatherThanConflicts(t *testing.T) {
	step := publishStep(t)
	if !strings.Contains(step, "gh release upload") {
		t.Error("the step does not upload assets separately from creating the release")
	}
	if !strings.Contains(step, "--clobber") {
		t.Error("uploads do not use --clobber, so a re-run fails on an asset name that already exists")
	}
}

func TestPublishRetriesAFailedUploadAndThenFails(t *testing.T) {
	// Both halves matter. Retrying without a terminal failure would report a
	// release as published when it is not; failing without retrying puts the
	// common case in front of a person.
	step := publishStep(t)
	if !strings.Contains(step, "for attempt in") {
		t.Error("a failed upload is not retried")
	}
	if !strings.Contains(step, "::error::") || !strings.Contains(step, "exit 1") {
		t.Error("the step does not fail when an asset still cannot be uploaded; a release missing an asset would report success")
	}
}
