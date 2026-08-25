package forge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// fakeGitHub records what it was asked and answers with what the test scripted.
type fakeGitHub struct {
	pull string
	// commentPages holds one JSON array per page, in order, the way the forge
	// serves them. A single-page thread is just a one-element slice.
	commentPages []string
	// pagesRequested records the page numbers asked for, so a test can assert
	// the walk stopped rather than paging on forever.
	pagesRequested []int
	patched        []string
	posted         []string
	server         *httptest.Server
}

func newFake(t *testing.T, pull, comments string) *fakeGitHub {
	return newFakePaged(t, pull, comments)
}

func newFakePaged(t *testing.T, pull string, commentPages ...string) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{pull: pull, commentPages: commentPages}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls/"):
			fmt.Fprint(w, f.pull)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments"):
			page, err := strconv.Atoi(r.URL.Query().Get("page"))
			if err != nil || page < 1 {
				// The client must ask for an explicit page; without one the
				// forge's default would silently hide everything past page 1.
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.pagesRequested = append(f.pagesRequested, page)
			if page > len(f.commentPages) {
				fmt.Fprint(w, `[]`)
				return
			}
			fmt.Fprint(w, f.commentPages[page-1])
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

// fullPageOfOtherPeoplesComments builds a page the walk cannot stop on: it is
// exactly the page size, so the only way to learn what follows is to ask.
func fullPageOfOtherPeoplesComments(firstID int) string {
	bodies := make([]string, 0, commentPageSize)
	for i := 0; i < commentPageSize; i++ {
		bodies = append(bodies, fmt.Sprintf(`{"id":%d,"body":"a human comment"}`, firstID+i))
	}
	return "[" + strings.Join(bodies, ",") + "]"
}

func TestUpsertFindsItsCommentPastTheFirstPage(t *testing.T) {
	// On a thread with more than a page of comments the marker is not on page
	// one. Reading only that page would find nothing and post again on every
	// run, turning the gate into the comment spam Marker exists to prevent.
	third := fmt.Sprintf(`[{"id":3001,"body":%q}]`, Marker+"\nold findings")
	f := newFakePaged(t, samePR,
		fullPageOfOtherPeoplesComments(1000),
		fullPageOfOtherPeoplesComments(2000),
		third)

	action, err := f.client().UpsertComment(7, "new findings")
	if err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}
	if action != "replaced" {
		t.Fatalf("action = %q, want replaced; the marker on page 3 was not found", action)
	}
	if len(f.posted) != 0 {
		t.Errorf("a duplicate comment was appended: %v", f.posted)
	}
	if len(f.patched) != 1 || !strings.Contains(f.patched[0], "new findings") {
		t.Errorf("patched = %v", f.patched)
	}
	if got := fmt.Sprint(f.pagesRequested); got != "[1 2 3]" {
		t.Errorf("pages requested = %s, want [1 2 3]", got)
	}
}

func TestUpsertStopsPagingAtTheFirstShortPage(t *testing.T) {
	// A page shorter than the page size is the last one. Asking for another
	// spends a round trip to be told what the short page already said.
	f := newFakePaged(t, samePR,
		fullPageOfOtherPeoplesComments(1000),
		`[{"id":2001,"body":"a human comment"}]`)

	action, err := f.client().UpsertComment(7, "findings")
	if err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}
	if action != "posted" {
		t.Errorf("action = %q, want posted", action)
	}
	if got := fmt.Sprint(f.pagesRequested); got != "[1 2]" {
		t.Errorf("pages requested = %s, want [1 2]", got)
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
