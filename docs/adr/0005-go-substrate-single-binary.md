# Go as the substrate, shipped as a single binary

The framework's activation layer is Bash plus a Node renderer, and the README states the
stack as "Bash + Markdown". Turning that layer into a harness — a role runner, a
configuration schema, structured findings, usage accounting — is work the substrate has
to carry, and the current one does not: the `openai` adapter already had to shell out to
Node for JSON and HTTP, which is the shape of every provider that follows it. We rebuild
the activation layer in Go and ship it as a single binary named `mf`. The standards stay
Markdown, consumable as a git submodule with no runtime at all; the binary is what
activates them.

## Status

Accepted.

## Considered Options

- **Go, single binary (chosen)**: no runtime prerequisite for an adopter, which is the
  property that turns "copy these directories" into an installation. Subprocess
  handling, HTTP and cross-platform behaviour are first-class, and Windows under Git
  Bash — the primary development environment here — is where the current substrate is
  weakest.
- **Keep Bash**: rejected — subcommand parsing, configuration validation, JSON and HTTP
  are all outside what shell does well, and the `openai` adapter already demonstrated it
  by delegating to Node. Every provider added afterwards repeats that cost.
- **Node/TypeScript on npm**: rejected — the smallest step from today's soft dependency
  on Node and the best library situation of the four, but it makes a runtime a hard
  prerequisite of a development-process tool that must run in repositories of any
  language.
- **Python via uv or pipx**: rejected — it aligns with `sb100_agents`, the only known
  adopter, and would supply configuration schema and validation for free, but it imposes
  a Python environment for the same reason Node was rejected.

## Consequences

- Roughly 1,900 lines of tested shell and Node are replaced across the slices of
  `docs/specs/0014-rebuild-the-framework-as-a-harness.md`. Their suites stay green until
  each subject is ported, and `scripts/r2-review.sh` becomes a shim that delegates to the
  binary when it is present and runs the previous path when it is not.
  _Amended by `docs/specs/0027-close-the-audit-pendings.md`_: the shim is gone, and with
  it `scripts/reviewers/`, `scripts/setup.sh` and the four shell suites. Keeping both
  paths is what let the documentation describe one implementation while the binary ran
  another, and a guard comparing them would have had to encode every backend field a
  third time. The binary is the only way any gate runs; an adopter without a Go toolchain
  installs a release binary rather than falling back to shell.
- The repository gains a Go toolchain, a `go.mod` and a per-platform release pipeline it
  did not have. No Go exists in its history, so the idiom and the pipeline are both new
  at once; the slicing is what keeps that from landing in one step.
- The framework becomes two artifacts from one repository: the standards corpus, which
  stays a runtime-free submodule — `sb100_agents` consumes it that way and is unaffected
  — and the binary, which is installed.
- The README's stated stack changes from Bash to Go; Markdown is unchanged.
- Node stops being required for the status line once that slice lands, closing the
  limitation the README records about the two agents diverging on a machine without it.
