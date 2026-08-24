// Package framework embeds the canonical standards tree into the binary.
//
// The embed directive can only reach files at or below its own package
// directory, which is why this one file sits at the repository root rather than
// under internal/. It is what lets `mf upgrade` compare an adopter's standards
// against the ones this build shipped with, instead of asking them to diff two
// checkouts by hand.
package framework

import "embed"

//go:embed docs/standards/*.md
var Standards embed.FS

// StandardsPrefix is the path the embedded files carry.
const StandardsPrefix = "docs/standards"
