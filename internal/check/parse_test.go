package check

import (
	"strings"
	"testing"
)

const githubDoc = "# GitHub\n\n" +
	"## Conventional Commits\n\n" +
	"### Type Table\n\n" +
	"Single canonical vocabulary for the whole project.\n\n" +
	"- feat: new feature for the user.\n" +
	"- fix: bug fix.\n" +
	"- docs: documentation-only changes.\n" +
	"- chore: build, tooling, or configuration changes (e.g. eslint).\n\n" +
	"Examples: `feat(auth): add Google integration`.\n\n" +
	"### Branch Naming\n\n" +
	"- notatype: this bullet is in another section and must not be read as a type.\n"

func TestParsesTheTypeTableOutOfTheDocumentRatherThanCarryingACopy(t *testing.T) {
	types, err := ParseTypeTable(githubDoc)
	if err != nil {
		t.Fatalf("ParseTypeTable: %v", err)
	}
	want := []string{"feat", "fix", "docs", "chore"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Errorf("types = %v, want %v", types, want)
	}
}

func TestStopsTheTypeTableAtTheNextHeading(t *testing.T) {
	types, _ := ParseTypeTable(githubDoc)
	for _, ty := range types {
		if ty == "notatype" {
			t.Error("a bullet from the following section leaked into the vocabulary")
		}
	}
}

func TestFailsLoudlyWhenTheTypeTableCannotBeParsed(t *testing.T) {
	// Falling back to a compiled-in list would silently reinstate the parallel
	// vocabulary the standards forbid, and would turn a broken document into a
	// passing build.
	_, err := ParseTypeTable("# GitHub\n\nNo table here.\n")
	if err == nil {
		t.Fatal("want an error when the Type Table is absent")
	}
	if !strings.Contains(err.Error(), "Type Table") {
		t.Errorf("error %q does not name what it could not find", err)
	}
}

func TestFailsWhenTheTypeTableSectionExistsButIsEmpty(t *testing.T) {
	doc := "### Type Table\n\nprose only, no bullets\n\n### Branch Naming\n"
	if _, err := ParseTypeTable(doc); err == nil {
		t.Fatal("an empty vocabulary must be an error, not an empty list that rejects everything")
	}
}

const specMethodDoc = "# SPEC Method\n\n" +
	"## The Artifact\n\n" +
	"```markdown\n" +
	"# SPEC: <title>\n\n" +
	"## Problem\nOne sentence.\n\n" +
	"## Design Decision\nThe approach.\n\n" +
	"## Alternatives Considered\nTwo minimum.\n\n" +
	"## Scope\n- Includes:\n- Does NOT include:\n\n" +
	"## Acceptance Criteria\nVerifiable.\n\n" +
	"## Reproducibility\nCommands.\n\n" +
	"## Risks and Assumptions\nOne line each.\n" +
	"```\n\n" +
	"## Spec-lite\n\n" +
	"A lighter tier.\n\n" +
	"```markdown\n" +
	"# SPEC: <title>\n\n" +
	"## Problem\nOne sentence.\n\n" +
	"## Scope\n- Includes:\n- Does NOT include:\n\n" +
	"## Acceptance Criteria\nVerifiable.\n" +
	"```\n"

func TestParsesTheGateCheckedSectionsFromTheSpecLiteTemplate(t *testing.T) {
	// The document says spec-lite "keeps exactly the three Gate-checked
	// sections", so that block is the definition of what the Gate requires —
	// the full template carries more, and enforcing all of it would reject a
	// tier the standard explicitly allows.
	sections, err := ParseGateSections(specMethodDoc)
	if err != nil {
		t.Fatalf("ParseGateSections: %v", err)
	}
	want := []string{"Problem", "Scope", "Acceptance Criteria"}
	if strings.Join(sections, "|") != strings.Join(want, "|") {
		t.Errorf("sections = %v, want %v", sections, want)
	}
}

func TestFailsLoudlyWhenTheSpecLiteTemplateCannotBeParsed(t *testing.T) {
	_, err := ParseGateSections("# SPEC Method\n\nNo template.\n")
	if err == nil {
		t.Fatal("want an error when the template is absent")
	}
	if !strings.Contains(err.Error(), "Spec-lite") {
		t.Errorf("error %q does not name what it could not find", err)
	}
}
