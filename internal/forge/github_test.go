package forge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGitHub records what it was asked and answers with what the test scripted.
type fakeGitHub struct {
	pull     string
	comments string
	patched  []string
	posted   []string
	server   *httptest.Server
}

func newFake(t *testing.T, pull, comments string) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{pull: pull, comments: comments}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls/"):
			fmt.Fprint(w, f.pull)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments"):
			fmt.Fprint(w, f.comments)
		case r.Method == http.MethodPatch:
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			f.patched = append(f.patched, body["body"])
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodPost:
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			f.posted = append(f.posted, body["body"])
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) client() *Client {
	return &Client{BaseURL: f.server.URL, Owner: "o", Repo: "r", Token: "t"}
}

const samePR = `{"number":7,"title":"feat: a thing","body":"why it exists",
 "base":{"ref":"main","sha":"aaa"},
 "head":{"ref":"feat/x","sha":"bbb","repo":{"fork":false,"full_name":"o/r"}}}`

func TestPullRequestCarriesTheIntentR2CannotSee(t *testing.T) {
	// The title, the body and the linked spec are the reason R3 runs on the pull
	// request rather than on the branch: reviewing intent against implementation
	// is only possible where the intent is written down.
	f := newFake(t, samePR, `[]`)
	pr, err := f.client().PullRequest(7)
	if err != nil {
		t.Fatalf("PullRequest: %v", err)
	}
	if pr.Title != "feat: a thing" || pr.Body != "why it exists" {
		t.Errorf("intent lost: %+v", pr)
	}
	if pr.BaseRef != "main" || pr.HeadSHA != "bbb" {
		t.Errorf("refs lost: %+v", pr)
	}
	if pr.IsFork {
		t.Error("a same-repository pull request must not report as a fork")
	}
}

func TestPullRequestFromAForkIsReportedAsSuch(t *testing.T) {
	// Secrets are unavailable to fork workflows by design, so R3 must say it
	// cannot run rather than appear to pass.
	forkPR := strings.Replace(samePR, `"fork":false,"full_name":"o/r"`, `"fork":true,"full_name":"someone/r"`, 1)
	f := newFake(t, forkPR, `[]`)
	pr, err := f.client().PullRequest(7)
	if err != nil {
		t.Fatalf("PullRequest: %v", err)
	}
	if !pr.IsFork {
		t.Error("a fork pull request must report as a fork")
	}
}

func TestPullRequestFromADifferentRepositoryCountsAsAFork(t *testing.T) {
	// The `fork` flag alone is not enough: a head in another repository has the
	// same consequence for secrets whether or not GitHub labels it a fork.
	other := strings.Replace(samePR, `"fork":false,"full_name":"o/r"`, `"fork":false,"full_name":"elsewhere/r"`, 1)
	f := newFake(t, other, `[]`)
	pr, _ := f.client().PullRequest(7)
	if !pr.IsFork {
		t.Error("a head in another repository must be treated as a fork")
	}
}

func TestUpsertPostsWhenThereIsNoPreviousComment(t *testing.T) {
	f := newFake(t, samePR, `[]`)
	action, err := f.client().UpsertComment(7, "findings here")
	if err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}
	if action != "posted" {
		t.Errorf("action = %q, want posted", action)
	}
	if len(f.posted) != 1 || !strings.Contains(f.posted[0], "findings here") {
		t.Errorf("posted = %v", f.posted)
	}
	if !strings.Contains(f.posted[0], Marker) {
		t.Error("the comment carries no marker, so a re-run could not find it again")
	}
}

func TestUpsertReplacesItsOwnPreviousCommentRatherThanAppending(t *testing.T) {
	// Comment spam is how a review bot becomes invisible.
	existing := fmt.Sprintf(`[{"id":11,"body":"someone else"},{"id":22,"body":%q}]`, Marker+"\nold findings")
	f := newFake(t, samePR, existing)
	action, err := f.client().UpsertComment(7, "new findings")
	if err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}
	if action != "replaced" {
		t.Errorf("action = %q, want replaced", action)
	}
	if len(f.posted) != 0 {
		t.Errorf("a new comment was appended as well: %v", f.posted)
	}
	if len(f.patched) != 1 || !strings.Contains(f.patched[0], "new findings") {
		t.Errorf("patched = %v", f.patched)
	}
}

func TestUpsertIgnoresCommentsThatAreNotItsOwn(t *testing.T) {
	f := newFake(t, samePR, `[{"id":11,"body":"a human review"}]`)
	action, err := f.client().UpsertComment(7, "findings")
	if err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}
	if action != "posted" {
		t.Errorf("action = %q; a human's comment must never be overwritten", action)
	}
	if len(f.patched) != 0 {
		t.Errorf("a comment this tool did not write was edited: %v", f.patched)
	}
}

func TestParseRepoSplitsTheWorkflowValue(t *testing.T) {
	owner, repo, ok := ParseRepo("LukeSantossz/my-framework")
	if !ok || owner != "LukeSantossz" || repo != "my-framework" {
		t.Errorf("ParseRepo = %q, %q, %v", owner, repo, ok)
	}
	if _, _, ok := ParseRepo("nope"); ok {
		t.Error("a value with no slash must not parse")
	}
}

func TestAnHTTPErrorIsReportedRatherThanSwallowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Owner: "o", Repo: "r"}
	if _, err := c.PullRequest(7); err == nil {
		t.Fatal("a 403 must surface; a silent empty pull request would review nothing and say it did")
	}
}
