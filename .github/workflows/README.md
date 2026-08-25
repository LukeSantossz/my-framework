# Workflows

Four files, and only two entry points. `ci.yml` runs on every commit and pull
request, `release.yml` on every version tag, and both call `gate.yml` and
nothing else — so the bar a tag has to clear and the bar `main` has to clear
are the same object rather than two lists that agree until someone edits one of
them. `review.yml` is separate because it is a review rather than a gate: it
runs on the pull request, where the intent is written down, and it never fails
on findings.

| File | Trigger | What it does |
| --- | --- | --- |
| `gate.yml` | `workflow_call` | Go build, vet, tests and `gofmt` on Linux, macOS and Windows; `mf check` against the change |
| `ci.yml` | push to `main`, pull request | calls `gate.yml` |
| `release.yml` | tag `v*` | calls `gate.yml`, then builds five platforms, verifies the version stamp, publishes with checksums |
| `review.yml` | pull request | R3: reviews the change against the pull request's stated intent and posts a comment |

## Configuring R3

R3 reviews nothing until this repository is told how to reach a reviewer.
`.framework.toml` declares the chain as `coderabbit`, which runs as a GitHub app
rather than as anything a workflow starts, so a runner has no reviewer to call.
Supplying one is a settings change, not a commit: `docs/adr/0006` puts the
reviewers a project wants in the committed file and how any of them is reached
in the machine layer, and a CI runner is a machine like any other.

Set these in **Settings → Secrets and variables → Actions**:

| Kind | Name | Value |
| --- | --- | --- |
| Variable | `MF_R3_ENDPOINT` | Base URL of the API, without `/chat/completions`. |
| Variable | `MF_R3_MODEL` | The model id to ask for. |
| Variable | `MF_R3_PROVIDER_KIND` | `openai-compatible` (the default), `anthropic`, or `google`. |
| Secret | `MF_R3_API_KEY` | The credential. |

`MF_R3_ENDPOINT` is the switch. With it set, `review.yml` writes the machine
configuration on the runner, prepends that backend to R3's chain for the run,
reviews the pull request and posts the findings as a comment. With it unset, a
job says so in the run summary and nothing else runs — no checkout, no
toolchain, no diff.

The key is a **secret** and the rest are **variables** on purpose: a variable is
readable by anyone who can open the settings page, and only one of these four
values is a credential. The workflow passes the key under the name the provider
definition points at, so there is one secret rather than one per vendor, and
changing vendors is three variables rather than a workflow edit.

Forks get neither. A fork's workflow is given no repository secrets and a
read-only token by design, so R3 cannot run there; a third job says that on the
pull request rather than leaving a missing check that reads as approval.
