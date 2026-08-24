// Package check enforces the framework's process rules.
//
// The vocabularies are read out of the standards rather than restated here. The
// standards forbid a parallel list — "The canonical type vocabulary lives only
// in the Type Table in github.md" — so a checker carrying its own copy would
// violate the rule it enforces, and would drift the first time the document
// changed. Reading the document makes the drift impossible instead of unlikely.
//
// Every parse failure is a hard error. Falling back to a compiled-in list would
// silently reinstate the forbidden vocabulary and turn a broken document into a
// passing build, so a document whose shape no longer matches stops the check
// rather than quietly running on stale data.
//
// See docs/adr/0009-checks-derive-vocabularies-from-standards.md.
package check

import (
	"fmt"
	"strings"
)

// ParseTypeTable reads the canonical Conventional Commits vocabulary out of
// github.md.
func ParseTypeTable(doc string) ([]string, error) {
	body, ok := sectionBody(doc, "### Type Table")
	if !ok {
		return nil, fmt.Errorf("could not find the Type Table heading in github.md; the checker reads the vocabulary from the document and will not guess it")
	}
	var types []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		entry := strings.TrimPrefix(line, "- ")
		name, _, found := strings.Cut(entry, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if name != "" && !strings.Contains(name, " ") {
			types = append(types, name)
		}
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("the Type Table in github.md carries no entries; an empty vocabulary would reject every commit")
	}
	return types, nil
}

// ParseGateSections reads the sections the Spec Gate requires out of the
// spec-lite template in spec_method.md.
//
// The spec-lite block is used rather than the full template because the
// document defines it as keeping "exactly the three Gate-checked sections".
// Enforcing the full template would reject a tier the standard allows.
func ParseGateSections(doc string) ([]string, error) {
	body, ok := sectionBody(doc, "## Spec-lite")
	if !ok {
		return nil, fmt.Errorf("could not find the Spec-lite section in spec_method.md; the checker reads the Gate's required sections from it and will not guess them")
	}
	block, ok := firstFencedBlock(body)
	if !ok {
		return nil, fmt.Errorf("the Spec-lite section in spec_method.md carries no template block to read the required sections from")
	}
	var sections []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			sections = append(sections, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("the Spec-lite template in spec_method.md declares no sections")
	}
	return sections, nil
}

// sectionBody returns the text between a heading and the next heading of the
// same or shallower depth, so a bullet in a following section cannot leak into
// the vocabulary read from this one.
func sectionBody(doc, heading string) (string, bool) {
	lines := strings.Split(doc, "\n")
	depth := len(heading) - len(strings.TrimLeft(heading, "#"))
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	inFence := false
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		// A template block is full of `#` lines that are content, not
		// structure. Reading them as headings would end the section at its own
		// example, which is exactly where the thing being parsed lives.
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		d := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		if d <= depth {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

func firstFencedBlock(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			return strings.Join(lines[start:i], "\n"), true
		}
	}
	return "", false
}
