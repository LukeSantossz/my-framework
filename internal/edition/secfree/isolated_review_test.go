package secfree

// Guards the isolated-review standard the edition ships. It is prose, so these
// tests read the document rather than run anything: what they assert is that
// the two obligations spec 0052 rests on are still written in it, so an edit
// that drops one fails here rather than in a pull request that recorded a
// same-provider review as a cross-provider one.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func isolatedReview(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", filepath.FromSlash(EditionDir), "isolated_review.md")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading %s: %v", p, err)
	}
	// Prose wraps at any word, so the phrases are matched with the line
	// breaks folded back into spaces.
	return strings.Join(strings.Fields(string(body)), " ")
}

func TestIsolatedReviewRequiresAContextThatSawNeitherAuthoringNorTheOtherReview(t *testing.T) {
	doc := isolatedReview(t)
	for _, want := range []string{
		"did not inherit the conversation in which the change was authored",
		"never receives the authoring conversation",
		"or another review's findings",
		"R3 does not see R2's findings",
		"A review run inside the authoring conversation is R1",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("isolated_review.md no longer says %q", want)
		}
	}
}

func TestIsolatedReviewRecordNamesSameProviderAndNeverACrossProviderState(t *testing.T) {
	doc := isolatedReview(t)
	for _, line := range []string{
		"R2: same-provider, isolated context, <model>.",
		"R3: same-provider, isolated context, <model>.",
	} {
		if !strings.Contains(doc, line) {
			t.Errorf("the PR record form lacks %q", line)
		}
	}
	if !strings.Contains(doc, "never use `verified`, `declared` or \"cross-provider\"") {
		t.Error("isolated_review.md no longer forbids recording a cross-provider state")
	}
	// The record form itself must not carry a state the review cannot have.
	block := regexp.MustCompile("(?s)```text(.*?)```").FindStringSubmatch(doc)
	if block == nil {
		t.Fatal("isolated_review.md has no record form in a text block")
	}
	for _, state := range []string{"verified", "declared", "cross-provider"} {
		if strings.Contains(block[1], state) {
			t.Errorf("the record form carries %q", state)
		}
	}
}
