// Package edition reads the markers that separate the full edition's text from
// the sec-free edition's, in the standards and the agent instruction source.
//
// There is one source for both editions. A region that belongs only to the
// full edition — a sentence naming the binary, a hook, a vendor backend — is
// wrapped in `<!-- mf:edition full -->` … `<!-- mf:edition end -->`, and may
// name a snippet the sec-free edition reads in its place:
// `<!-- mf:edition full secfree=<name> -->`. The full edition keeps the region
// and loses only the marker lines; the sec-free edition loses the region.
//
// The marker form follows `<!-- mf:role … -->` on purpose: one comment syntax
// for every instruction the generators read, each on a line of its own.
//
// A malformed marker is an error in both directions, never a line passed
// through. A typo that left a full-only region in the sec-free edition would
// publish instructions to run a binary its readers are forbidden to run, and
// nothing in that edition could notice.
package edition

import (
	"fmt"
	"regexp"
	"strings"
)

// candidate is any line that is trying to be an edition marker. Every one of
// them must then parse as a valid marker, so a misspelt one fails loudly.
var candidate = regexp.MustCompile(`^\s*<!--\s*mf:edition\b`)

var valid = regexp.MustCompile(`^<!--\s*mf:edition\s+(?:(full)(?:\s+secfree=([a-z0-9-]+))?|(end))\s*-->$`)

// region is one full-only region, by line index into the split body.
type region struct {
	open, end int
	snippet   string
}

// scan splits body into lines, keeping each line's ending, and locates every
// full-only region.
func scan(body string) ([]string, []region, error) {
	lines := strings.SplitAfter(body, "\n")
	var regions []region
	current := -1
	snippet := ""
	for i, line := range lines {
		text := strings.TrimRight(line, "\r\n")
		if !candidate.MatchString(text) {
			continue
		}
		m := valid.FindStringSubmatch(strings.TrimSpace(text))
		if m == nil {
			return nil, nil, fmt.Errorf("line %d: %q is not an edition marker; expected `<!-- mf:edition full -->`, `<!-- mf:edition full secfree=<name> -->` or `<!-- mf:edition end -->`", i+1, text)
		}
		if m[1] == "full" {
			if current >= 0 {
				return nil, nil, fmt.Errorf("line %d: an edition region opens inside the one opened on line %d; regions do not nest", i+1, current+1)
			}
			current, snippet = i, m[2]
			continue
		}
		if current < 0 {
			return nil, nil, fmt.Errorf("line %d: `<!-- mf:edition end -->` closes no open region", i+1)
		}
		regions = append(regions, region{open: current, end: i, snippet: snippet})
		current = -1
	}
	if current >= 0 {
		return nil, nil, fmt.Errorf("line %d: the edition region opened here is never closed with `<!-- mf:edition end -->`", current+1)
	}
	return lines, regions, nil
}

// Unmark returns the full edition's text: every marker line removed and every
// region's content kept. Text with no markers comes back byte for byte.
func Unmark(body string) (string, error) {
	lines, regions, err := scan(body)
	if err != nil {
		return "", err
	}
	drop := map[int]bool{}
	for _, r := range regions {
		drop[r.open] = true
		drop[r.end] = true
	}
	var b strings.Builder
	for i, line := range lines {
		if !drop[i] {
			b.WriteString(line)
		}
	}
	return b.String(), nil
}

// Strip returns the sec-free edition's text: every full-only region removed,
// and replaced by its named snippet where it names one. snippet resolves a
// name to its text; an error from it fails the strip, because a region whose
// replacement is missing would silently lose what the edition needed in its
// place.
func Strip(body string, snippet func(name string) (string, error)) (string, error) {
	lines, regions, err := scan(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	next := 0
	for _, r := range regions {
		for _, line := range lines[next:r.open] {
			b.WriteString(line)
		}
		if r.snippet != "" {
			text, err := snippet(r.snippet)
			if err != nil {
				return "", fmt.Errorf("line %d: snippet %q: %w", r.open+1, r.snippet, err)
			}
			b.WriteString(strings.TrimRight(text, "\r\n"))
			b.WriteString(lineEnding(lines[r.open]))
		}
		next = r.end + 1
	}
	for _, line := range lines[next:] {
		b.WriteString(line)
	}
	return b.String(), nil
}

// lineEnding is the ending a marker line carried, so a snippet takes the
// convention of the document it is put into.
func lineEnding(line string) string {
	if strings.HasSuffix(line, "\r\n") {
		return "\r\n"
	}
	return "\n"
}
