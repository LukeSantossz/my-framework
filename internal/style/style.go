// Package style applies the Token Economy's terse mode to the prompts this
// harness composes, and refuses it everywhere the standard forbids it.
//
// docs/standards/token_economy.md permits terse output for conversation,
// status updates and explanations, and forbids it for anything that becomes a
// spec, pull request, issue, commit message or code comment. That boundary used
// to be a rule a person had to remember while writing. Here it is a function
// that returns an error, so the only way past it is to change this file.
//
// The prompts the harness composes are the only place the framework can apply
// anything at all: a person writing a commit message by hand is outside it, and
// this package does not pretend otherwise.
package style

import "fmt"

// Artifact is what the text being asked for will become. It is the shape of the
// output, not the shape of the request: the same review is conversational when
// it is printed to a terminal and a pull request artifact when it is posted.
type Artifact string

const (
	// Permitted by token_economy.md §2.
	Conversation Artifact = "conversation"
	StatusUpdate Artifact = "status-update"
	Explanation  Artifact = "explanation"

	// Forbidden by token_economy.md §3.
	Spec        Artifact = "spec"
	PullRequest Artifact = "pull-request"
	Issue       Artifact = "issue"
	Commit      Artifact = "commit"
	CodeComment Artifact = "code-comment"
)

// TerseInstruction is what gets appended. It carries its own precedence,
// because the model reading it never reads token_economy.md: an instruction to
// be brief, with nothing saying what brevity may not cost, is how a security
// finding turns into half a sentence.
const TerseInstruction = `Style: terse. Drop filler, hedging and preamble; keep every technical fact,
identifier, path and quoted error exact. Terseness is a style, never a reason to
do less work, skip a required step, or omit a finding. Safety and Correctness
outrank it: if being brief would drop a security concern, an error path, or the
reason a finding matters, write it out in full.`

// ForbiddenError says terse style was asked for somewhere the standard forbids
// it. It names the standard so the refusal can be checked rather than believed.
type ForbiddenError struct {
	Artifact Artifact
	Reason   string
}

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("terse style does not apply to %s: %s (docs/standards/token_economy.md §3)",
		e.Artifact, e.Reason)
}

// permitted lists the artifacts terse mode may reach. Membership is explicit in
// both directions: an artifact this package has never classified is refused,
// not assumed conversational, because a boundary that widens by default widens
// without anyone deciding to move it.
var permitted = map[Artifact]bool{
	Conversation: true,
	StatusUpdate: true,
	Explanation:  true,
}

var forbidden = map[Artifact]string{
	Spec:        "a spec fills every section with real content and passes the Spec Gate",
	PullRequest: "the pull request template is filled, not abbreviated",
	Issue:       "an issue body carries structured sections, not one-liners",
	Commit:      "a commit message follows Conventional Commits with an imperative subject",
	CodeComment: "the why-not-what rule stands; brevity does not license dropping intent",
}

// Compose returns the prompt with the terse instruction appended, or the prompt
// unchanged and an error. The base is never rewritten, only extended: a style
// layer that edits the instruction it decorates can silently drop the sentence
// that mattered.
func Compose(base string, a Artifact) (string, error) {
	if reason, ok := forbidden[a]; ok {
		return base, &ForbiddenError{Artifact: a, Reason: reason}
	}
	if !permitted[a] {
		return base, &ForbiddenError{Artifact: a,
			Reason: "this artifact is not classified here, and an unclassified artifact is not assumed to be conversational"}
	}
	if base == "" {
		return TerseInstruction, nil
	}
	return base + "\n\n" + TerseInstruction, nil
}

// Capability is one thing the Token Economy names, and whether this framework
// performs it.
type Capability struct {
	Name        string
	Implemented bool
	Note        string
}

// Capabilities reports what the Token Economy describes against what exists
// here. `caveman-compress` is listed precisely because it does not exist:
// naming a capability the framework lacks is honest only while something says
// plainly that it is absent.
func Capabilities() []Capability {
	return []Capability{
		{
			Name:        "terse-prompt-style",
			Implemented: true,
			Note:        "applied to the prompts this harness composes; refused for spec, pull request, issue, commit and code comment text",
		},
		{
			Name:        "caveman-compress",
			Implemented: false,
			Note:        "no implementation here: rewriting the loaded context file risks the activation the framework depends on, so the context file stays in full prose (token_economy.md §1)",
		},
	}
}
