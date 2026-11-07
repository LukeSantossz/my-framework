package edition

import (
	"errors"
	"strings"
	"testing"
)

func noSnippets(name string) (string, error) {
	return "", errors.New("no snippet named " + name)
}

func snippets(m map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		body, ok := m[name]
		if !ok {
			return "", errors.New("no snippet named " + name)
		}
		return body, nil
	}
}

const marked = "intro\n" +
	"<!-- mf:edition full -->\n" +
	"run `mf check` before pushing\n" +
	"<!-- mf:edition end -->\n" +
	"shared rule\n" +
	"<!-- mf:edition full secfree=review -->\n" +
	"R2 is cross-provider\n" +
	"<!-- mf:edition end -->\n" +
	"outro\n"

func TestStripRemovesEveryFullOnlyRegion(t *testing.T) {
	got, err := Strip(marked, snippets(map[string]string{"review": "R2 is an isolated context\n"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"mf check", "cross-provider", "mf:edition"} {
		if strings.Contains(got, gone) {
			t.Errorf("stripped text still carries %q:\n%s", gone, got)
		}
	}
	for _, kept := range []string{"intro\n", "shared rule\n", "outro\n"} {
		if !strings.Contains(got, kept) {
			t.Errorf("stripped text lost %q:\n%s", kept, got)
		}
	}
}

func TestStripSubstitutesTheNamedSnippet(t *testing.T) {
	got, err := Strip(marked, snippets(map[string]string{"review": "R2 is an isolated context\n\n"}))
	if err != nil {
		t.Fatal(err)
	}
	want := "intro\nshared rule\nR2 is an isolated context\noutro\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestStripFailsOnASnippetNameThatDoesNotExist(t *testing.T) {
	if _, err := Strip(marked, noSnippets); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("expected an error naming the missing snippet, got %v", err)
	}
}

func TestMalformedMarkersFailInBothEditions(t *testing.T) {
	cases := map[string]string{
		"unclosed":         "a\n<!-- mf:edition full -->\nb\n",
		"nested":           "<!-- mf:edition full -->\n<!-- mf:edition full -->\nx\n<!-- mf:edition end -->\n<!-- mf:edition end -->\n",
		"end without open": "a\n<!-- mf:edition end -->\n",
		"unknown edition":  "<!-- mf:edition corporate -->\nx\n<!-- mf:edition end -->\n",
		"bad snippet name": "<!-- mf:edition full secfree=Bad_Name -->\nx\n<!-- mf:edition end -->\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Strip(body, noSnippets); err == nil {
				t.Error("Strip accepted a malformed marker")
			}
			if _, err := Unmark(body); err == nil {
				t.Error("Unmark accepted a malformed marker")
			}
		})
	}
}

func TestUnmarkKeepsTheContentAndDropsOnlyTheMarkerLines(t *testing.T) {
	got, err := Unmark(marked)
	if err != nil {
		t.Fatal(err)
	}
	want := "intro\nrun `mf check` before pushing\nshared rule\nR2 is cross-provider\noutro\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestUnmarkLeavesUnmarkedTextByteIdentical(t *testing.T) {
	for _, body := range []string{"", "a\nb\n", "no trailing newline", "a\r\nb\r\n", "<!-- mf:role author -->\nx\n"} {
		got, err := Unmark(body)
		if err != nil {
			t.Fatal(err)
		}
		if got != body {
			t.Errorf("Unmark(%q) = %q", body, got)
		}
	}
}

func TestMarkersAreRecognisedOnCRLFLines(t *testing.T) {
	body := "a\r\n<!-- mf:edition full -->\r\nb\r\n<!-- mf:edition end -->\r\nc\r\n"
	got, err := Strip(body, noSnippets)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\r\nc\r\n" {
		t.Errorf("got %q", got)
	}
}
