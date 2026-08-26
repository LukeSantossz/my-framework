// Package tomlx reads the structure of a TOML file one line at a time.
//
// Two commands edit a TOML file in place rather than re-encoding it: `mf
// statusline apply`, which rewrites two keys of a Developer's Codex
// configuration, and `mf config set`, which writes one key into a repository's
// committed policy. Both must leave every comment, every blank line and every
// key they were not asked about exactly as they found them, which a decode-and-
// re-encode round trip cannot do.
//
// It lives here rather than in either of them because a second implementation
// of the same parsing is how the two came to disagree: the status line's editor
// handles a commented or quoted header and the configuration writer's did not,
// so `mf config set` appended a duplicate table to a file whose header carried
// a trailing comment — and a duplicate table is a file that no longer loads,
// for everyone who clones it. Neither command is the natural home for the
// other's parser.
package tomlx

import "strings"

// Table reports which table a line opens, and whether it opens one at all.
//
// A header is not simply a line wrapped in brackets: TOML allows padding inside
// them and a comment after them, and `[tui] # my terminal settings` names the
// same table as `[tui]`. Reading either as ordinary content costs more than the
// section it missed — an in-place rewrite stays in whichever table it thought
// it was in, and the header it never saw gets a second one appended, which
// leaves a file the decoder refuses while the command reports success.
//
// The line is expected trimmed of leading and trailing space.
func Table(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	header := strings.TrimSpace(StripComment(trimmed))
	if len(header) < 2 || !strings.HasSuffix(header, "]") {
		return "", false
	}
	name := strings.TrimSpace(header[1 : len(header)-1])
	return strings.Trim(name, `"'`), true
}

// StripComment cuts a line at the `#` that starts a comment, leaving one inside
// a quoted key alone: `["a#b"]` names a table rather than commenting one out.
func StripComment(line string) string {
	quote := rune(0)
	for i, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			return line[:i]
		}
	}
	return line
}

// Key returns the name a line assigns to, unquoted, or "" when it assigns
// nothing.
func Key(line string) string {
	name, _, found := strings.Cut(StripComment(line), "=")
	if !found {
		return ""
	}
	return strings.Trim(strings.TrimSpace(name), `"'`)
}
