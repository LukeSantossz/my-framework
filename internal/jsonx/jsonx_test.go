package jsonx

import "testing"

func TestFindsTheObjectInsideAChattyAnswer(t *testing.T) {
	body := "Sure, here it is:\n```json\n{\"findings\":[]}\n```\nHope that helps."
	got, ok := Object(body, "findings")
	if !ok || got != `{"findings":[]}` {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestFindsTheEnvelopeAndNotANestedObjectThatPrecedesTheAnchor(t *testing.T) {
	// Searching backwards from the anchor finds the innermost enclosing object,
	// which here is `background`. The failure is silent — every field decodes
	// as empty — so it is worth a test of its own.
	body := `{"background":{"deep":"d"},"intuition":"i"}`
	got, ok := Object(body, "intuition")
	if !ok || got != body {
		t.Errorf("got %q, %v; want the whole envelope", got, ok)
	}
}

func TestSkipsAnObjectThatDoesNotCarryTheAnchor(t *testing.T) {
	body := `Consider {"example": 1}. The answer: {"findings": [{"summary":"x"}]}`
	got, ok := Object(body, "findings")
	if !ok || got != `{"findings": [{"summary":"x"}]}` {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestBracesInsideStringsDoNotEndTheObject(t *testing.T) {
	body := `{"findings":[{"summary":"a } brace and a \" quote"}]}`
	got, ok := Object(body, "findings")
	if !ok || got != body {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestAnUnterminatedObjectIsNotSalvaged(t *testing.T) {
	if _, ok := Object(`{"findings":[{"summary":"cut off"`, "findings"); ok {
		t.Fatal("half an envelope was accepted as a whole one")
	}
}

func TestProseWithNoObjectIsRefused(t *testing.T) {
	if _, ok := Object("I could not review this.", "findings"); ok {
		t.Fatal("prose was read as an envelope")
	}
}
