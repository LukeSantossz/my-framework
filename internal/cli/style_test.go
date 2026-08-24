package cli

import (
	"strings"
	"testing"
)

func TestReviewAppliesTerseStyleToAConversationPromptAndNotToAPostedOne(t *testing.T) {
	// The same review, the same chain, the same diff. What differs is what the
	// output becomes: a line in a terminal, or a pull request artifact that
	// token_economy.md §3 requires in full prose.
	root := gitRepo(t, chainProject)
	branchWithChange(t, root, "feat/x")

	conversational, out, _ := reviewEnv(t, root, "review", "--role", "r2", "--dry-run")
	if code := Run(conversational); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "prompt style: terse (conversation)") {
		t.Errorf("terse style was not applied to a conversation prompt: %q", out.String())
	}

	posted, out2, _ := reviewEnv(t, root, "review", "--role", "r3", "--dry-run", "--pr", "7", "--post")
	posted.Getenv = func(name string) string {
		switch name {
		case "GITHUB_REPOSITORY":
			return "owner/repo"
		case "GITHUB_TOKEN":
			return "t"
		}
		return ""
	}
	// The forge call is what would need the network; the style decision is made
	// before it, so a failure there does not hide the assertion below.
	Run(posted)
	if strings.Contains(out2.String(), "prompt style: terse") {
		t.Errorf("terse style reached a pull request artifact: %q", out2.String())
	}
}

func TestDoctorRecordsCavemanCompressAsHavingNoImplementation(t *testing.T) {
	// token_economy.md names a capability the framework does not have. Naming it
	// is honest only while something says plainly that it is absent.
	e, out, _ := env(t, "version = 1\n", "doctor")
	if code := Run(e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "caveman-compress") {
		t.Fatalf("doctor does not mention caveman-compress:\n%s", got)
	}
	line := ""
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "caveman-compress") {
			line = l
		}
	}
	if !strings.Contains(line, "NOT IMPLEMENTED") {
		t.Errorf("caveman-compress is not reported as absent: %q", line)
	}
	if !strings.Contains(got, "terse-prompt-style") {
		t.Errorf("the capability that does exist is not reported either:\n%s", got)
	}
}
