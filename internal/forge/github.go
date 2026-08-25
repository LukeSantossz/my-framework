// Package forge talks to the code host. Only GitHub is implemented, and the
// package exists so that fact is visible in one place rather than spread
// through the review path.
package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Marker identifies this tool's own comment so a re-run replaces it rather than
// appending another. A gate that runs on every push and comments every time
// trains people to stop reading it.
const Marker = "<!-- mf:review:r3 -->"

type Client struct {
	BaseURL string
	Token   string
	Owner   string
	Repo    string
	HTTP    *http.Client
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) do(method, path string, body any) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.BaseURL, "/")+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, nil
}

// PullRequest is the context R3 sees that R2 does not: the intent, written down.
type PullRequest struct {
	Number  int
	Title   string
	Body    string
	BaseRef string
	HeadRef string
	BaseSHA string
	HeadSHA string
	IsFork  bool
}

func (c *Client) PullRequest(number int) (PullRequest, error) {
	raw, status, err := c.do(http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls/%d", c.Owner, c.Repo, number), nil)
	if err != nil {
		return PullRequest{}, err
	}
	if status != http.StatusOK {
		return PullRequest{}, fmt.Errorf("GET pull %d returned HTTP %d", number, status)
	}
	var parsed struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Base   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			Ref  string `json:"ref"`
			SHA  string `json:"sha"`
			Repo struct {
				Fork     bool   `json:"fork"`
				FullName string `json:"full_name"`
			} `json:"repo"`
		} `json:"head"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return PullRequest{}, fmt.Errorf("pull %d response was not JSON: %w", number, err)
	}
	return PullRequest{
		Number: parsed.Number, Title: parsed.Title, Body: parsed.Body,
		BaseRef: parsed.Base.Ref, HeadRef: parsed.Head.Ref,
		BaseSHA: parsed.Base.SHA, HeadSHA: parsed.Head.SHA,
		IsFork: parsed.Head.Repo.Fork ||
			(parsed.Head.Repo.FullName != "" && parsed.Head.Repo.FullName != c.Owner+"/"+c.Repo),
	}, nil
}

// commentPageSize is the maximum the comments endpoint accepts. A smaller page
// would only mean more round trips for the same answer.
const commentPageSize = 100

// commentPageLimit bounds the walk so a forge that ignores the page parameter
// and keeps returning a full page cannot spin here forever. At the page size
// above this covers ten thousand comments, which is far past any thread a
// person would still be reading.
const commentPageLimit = 100

// findMarkedComment returns the id of this tool's own comment on the thread, or
// found=false when it has never commented there.
//
// Every page is walked rather than only the first. A busy pull request pushes
// the marker off page one, and a marker that cannot be found is a marker that
// does nothing: each run would post another comment, which is exactly the
// behaviour Marker exists to prevent.
func (c *Client) findMarkedComment(number int) (id int64, found bool, err error) {
	for page := 1; page <= commentPageLimit; page++ {
		raw, status, err := c.do(http.MethodGet, fmt.Sprintf(
			"/repos/%s/%s/issues/%d/comments?per_page=%d&page=%d",
			c.Owner, c.Repo, number, commentPageSize, page), nil)
		if err != nil {
			return 0, false, err
		}
		if status != http.StatusOK {
			return 0, false, fmt.Errorf("listing comments page %d returned HTTP %d", page, status)
		}
		var comments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := json.Unmarshal(raw, &comments); err != nil {
			return 0, false, fmt.Errorf("comment list page %d was not JSON: %w", page, err)
		}
		for _, existing := range comments {
			if strings.Contains(existing.Body, Marker) {
				return existing.ID, true, nil
			}
		}
		// A short page is the last one. Asking for the next would cost a round
		// trip to learn what this page already said.
		if len(comments) < commentPageSize {
			return 0, false, nil
		}
	}
	return 0, false, fmt.Errorf("gave up looking for the review comment after %d pages", commentPageLimit)
}

// UpsertComment replaces this tool's previous comment when one exists, and
// posts a new one otherwise. It reports which happened.
func (c *Client) UpsertComment(number int, body string) (string, error) {
	if !strings.Contains(body, Marker) {
		body = Marker + "\n" + body
	}
	existingID, found, err := c.findMarkedComment(number)
	if err != nil {
		return "", err
	}
	if found {
		_, patchStatus, patchErr := c.do(http.MethodPatch,
			fmt.Sprintf("/repos/%s/%s/issues/comments/%d", c.Owner, c.Repo, existingID),
			map[string]string{"body": body})
		if patchErr != nil {
			return "", patchErr
		}
		if patchStatus != http.StatusOK {
			return "", fmt.Errorf("updating comment returned HTTP %d", patchStatus)
		}
		return "replaced", nil
	}
	_, postStatus, postErr := c.do(http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", c.Owner, c.Repo, number),
		map[string]string{"body": body})
	if postErr != nil {
		return "", postErr
	}
	if postStatus != http.StatusCreated && postStatus != http.StatusOK {
		return "", fmt.Errorf("posting comment returned HTTP %d", postStatus)
	}
	return "posted", nil
}

// ParseRepo splits an "owner/repo" pair, which is the shape GITHUB_REPOSITORY
// carries in a workflow.
func ParseRepo(full string) (owner, repo string, ok bool) {
	owner, repo, ok = strings.Cut(full, "/")
	return owner, repo, ok && owner != "" && repo != ""
}
