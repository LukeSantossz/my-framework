package style

import (
	"strings"
	"testing"
)

func TestAppliesTerseStyleToAConversationPrompt(t *testing.T) {
	base := "Review the change and report findings."
	out, err := Compose(base, Conversation)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !strings.HasPrefix(out, base) {
		t.Errorf("the base instruction was rewritten rather than extended:\n%s", out)
	}
	if !strings.Contains(out, TerseInstruction) {
		t.Error("the terse instruction was not applied to a conversation prompt")
	}
}

func TestAppliesTerseStyleToStatusUpdatesAndExplanations(t *testing.T) {
	// token_economy.md §2 permits exactly these three shapes.
	for _, artifact := range []Artifact{Conversation, StatusUpdate, Explanation} {
		if _, err := Compose("base", artifact); err != nil {
			t.Errorf("%s: terse mode is permitted here but was refused: %v", artifact, err)
		}
	}
}

func TestRefusesTerseStyleForSpecPullRequestIssueAndCommitText(t *testing.T) {
	// The hard boundary in token_economy.md §3. Previously a rule a person had
	// to remember; refusing here is what makes it enforced rather than trusted.
	for _, artifact := range []Artifact{Spec, PullRequest, Issue, Commit, CodeComment} {
		out, err := Compose("base", artifact)
		if err == nil {
			t.Errorf("%s: terse style was applied to a versioned artifact", artifact)
			continue
		}
		if out != "base" {
			t.Errorf("%s: the refused call still altered the prompt: %q", artifact, out)
		}
		if !strings.Contains(err.Error(), "token_economy.md") {
			t.Errorf("%s: the refusal does not name the standard it comes from: %v", artifact, err)
		}
	}
}

func TestAnUnknownArtifactIsRefusedRatherThanAssumedConversational(t *testing.T) {
	// A new artifact kind arriving as terse-by-default is how the boundary
	// erodes without anyone deciding to move it.
	if _, err := Compose("base", Artifact("release-notes")); err == nil {
		t.Fatal("an artifact this package does not classify must be refused")
	}
}

func TestNeverLetsTerseStyleShortenASafetyOrCorrectnessInstruction(t *testing.T) {
	// token_economy.md places this norm below Safety and Correctness in the
	// code_conventions.md precedence order. The instruction has to carry that,
	// because the model reading it never reads the standard.
	for _, want := range []string{"Safety", "Correctness", "never"} {
		if !strings.Contains(TerseInstruction, want) {
			t.Errorf("the terse instruction omits %q, so nothing tells the model what outranks it", want)
		}
	}

	safety := "Report every security finding in full, with its rationale."
	out, err := Compose(safety, Conversation)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !strings.Contains(out, safety) {
		t.Error("the safety instruction did not survive composition intact")
	}
}

func TestRecordsCavemanCompressAsHavingNoImplementation(t *testing.T) {
	// Naming a capability the framework does not have is only honest while
	// something says plainly that it does not have it.
	caps := Capabilities()
	found := false
	for _, c := range caps {
		if c.Name != "caveman-compress" {
			continue
		}
		found = true
		if c.Implemented {
			t.Error("caveman-compress is reported as implemented; nothing here rewrites a context file")
		}
		if c.Note == "" {
			t.Error("caveman-compress is recorded as absent with no reason, which reads as an oversight")
		}
	}
	if !found {
		t.Fatal("caveman-compress is not recorded at all")
	}
}

func TestTerseModeIsRecordedAsImplemented(t *testing.T) {
	for _, c := range Capabilities() {
		if c.Name == "terse-prompt-style" && c.Implemented {
			return
		}
	}
	t.Fatal("the terse style this package applies is not recorded as implemented")
}
