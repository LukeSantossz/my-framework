package tomlx

import "testing"

func TestTableNormalisesSpaceAroundADottedName(t *testing.T) {
	// TOML allows space around the separators, so these name the same table.
	// Comparing the text as written missed the second when the file wrote the
	// first, and `mf config set` then appended a table that already existed.
	for _, line := range []string{"[roles.r2]", "[roles . r2]", "[ roles.r2 ]", "[roles.r2] # the chain"} {
		got, ok := Table(line)
		if !ok || got != "roles.r2" {
			t.Errorf("Table(%q) = %q, %v; want roles.r2", line, got, ok)
		}
	}
}

func TestTableKeepsADotInsideQuotes(t *testing.T) {
	// A dot between quotes is part of a key, not a separator.
	if got, ok := Table(`["a.b"]`); !ok || got != "a.b" {
		t.Errorf(`Table("[\"a.b\"]") = %q, %v`, got, ok)
	}
	if got, ok := Table(`[a."b.c"]`); !ok || got != "a.b.c" {
		t.Errorf(`Table("[a.\"b.c\"]") = %q, %v`, got, ok)
	}
}

func TestTableRejectsWhatDoesNotOpenATable(t *testing.T) {
	for _, line := range []string{"", "backends = []", "# [roles.r2]", "[unterminated"} {
		if got, ok := Table(line); ok {
			t.Errorf("Table(%q) reported table %q", line, got)
		}
	}
}

func TestKeyIgnoresACommentAfterTheValue(t *testing.T) {
	if got := Key(`base = "main" # the branch`); got != "base" {
		t.Errorf("Key = %q, want base", got)
	}
	if got := Key(`"base" = "main"`); got != "base" {
		t.Errorf("Key = %q, want base", got)
	}
	if got := Key("no assignment here"); got != "" {
		t.Errorf("Key = %q, want empty", got)
	}
}
