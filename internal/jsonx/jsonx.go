// Package jsonx recovers a JSON object from an answer that also contains prose.
//
// Models asked for JSON routinely wrap it in a sentence, a fenced code block,
// or both. Trimming the fences is not enough — the object has to be found by
// its own structure — and every caller that asks a model for a structured
// answer has the same problem for a different key. It lives here rather than in
// one of them because neither review reporting nor change explanation is the
// natural home for the other's parser.
package jsonx

import "encoding/json"

// Object returns the first well-formed JSON object that has the anchor as a
// top-level key.
//
// Top-level is what makes it correct on a nested envelope: searching backwards
// from the anchor for an opening brace finds the innermost object containing
// it, which for `{"background":{...},"intuition":"..."}` is the wrong one —
// and being the wrong one is invisible, because the fields simply come back
// empty. Candidates are tried from the start of the answer, so prose that
// happens to contain a brace costs a failed parse rather than a wrong result.
func Object(body, anchor string) (string, bool) {
	for start := 0; start < len(body); start++ {
		if body[start] != '{' {
			continue
		}
		raw, ok := balanced(body, start)
		if !ok {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &fields); err != nil {
			continue
		}
		if _, has := fields[anchor]; has {
			return raw, true
		}
	}
	return "", false
}

// balanced returns the object beginning at start. An unterminated one is not a
// partial answer to be salvaged: half an envelope parsed as a whole one reports
// fields nobody wrote.
func balanced(body string, start int) (string, bool) {
	depth, inString, escaped := 0, false, false
	for i := start; i < len(body); i++ {
		c := body[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return body[start : i+1], true
			}
		}
	}
	return "", false
}
