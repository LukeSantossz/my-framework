// Package framework embeds into the binary the documents an adopting
// repository has to be given.
//
// The embed directive can only reach files at or below its own package
// directory, which is why this one file sits at the repository root rather than
// under internal/. It is what lets `mf upgrade` compare an adopter's standards
// against the ones this build shipped with, and what lets `mf init` put a
// corpus there in the first place, instead of asking them to copy two trees
// between checkouts by hand.
package framework

import "embed"

//go:embed docs/standards/*.md
var Standards embed.FS

// StandardsPrefix is the path the embedded files carry.
const StandardsPrefix = "docs/standards"

// AgentDocs is the agent-facing tree: the marked-up source every vendor
// instruction file is generated from, and the skill documents that source and
// the standards both reference.
//
// It ships with the binary for the same reason the standards do. A repository
// with no source has nothing for `mf agents sync` to read, so the generated
// CLAUDE.md an agent finds at its root would be a file nobody could regenerate
// or check for drift; and a repository given the standards without these would
// have a corpus whose cross-references point at documents it was never handed.
//
//go:embed docs/agents/*.md
var AgentDocs embed.FS

// AgentDocsPrefix is the path the embedded agent documents carry.
const AgentDocsPrefix = "docs/agents"

// Hooks is the versioned hook directory `core.hooksPath` is pointed at.
//
// It ships in the binary because a repository with no hooks has no gate, and
// `mf init` reporting that it wired one while the directory was empty is the
// state that made the documented adoption path produce an ungated repository.
// The `all:` prefix is required: without it embed skips every path element
// beginning with a dot, which is every element of this one.
//
//go:embed all:.githooks
var Hooks embed.FS

// HooksPrefix is the path the embedded hooks carry, and the directory they
// belong in. The two are the same string on purpose: git finds hooks by name
// under core.hooksPath, so a copy anywhere else is one git never runs.
const HooksPrefix = ".githooks"
