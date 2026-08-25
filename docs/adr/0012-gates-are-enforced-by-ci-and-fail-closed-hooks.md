# The deterministic gates are enforced by CI and by fail-closed hooks

Every rule this repository calls binding — the Spec Gate, the commit and branch
vocabularies of `github.md`, the documentation invariants of `docs/adr/0009`, the durable
numbering of `spec_method.md`, the generated instruction files, and the design gate of
`docs/adr/0011` — was implemented in `mf check` and then invoked by no workflow and no
hook. A framework whose premise is that a standard nothing performs is not a standard did
not activate its own. The pre-push hook was worse than absent: both of its failure paths
ended in `|| exit 0` and printed nothing, so an adopter who wired `core.hooksPath`, saw
`mf doctor` report the hooks as wired, and pushed, had no gate at all and was never told.

So the gates are wired to two places and both fail closed. `mf check` runs in CI as
`gate.yml`, which `ci.yml` calls on every commit and pull request and `release.yml` calls
on every version tag — one definition of "does this tree pass", so a tag cannot publish
what `main` would have been rejected for. It also runs from the versioned hooks:
`pre-push` for the whole set before the R2 review, `commit-msg` for the commit vocabulary
against the message still under the author's cursor. A hook that cannot find the binary
stops the action and says how to get one, because a gate that cannot find its runner has
not passed — it has not run, and those are only the same thing to a hook that lies.

## Status

Accepted.

## Considered Options

- **CI and fail-closed hooks, from a single gate definition (chosen)**: the two answer
  different questions and neither replaces the other. The hook is the fast local answer
  that stops a defect before it is shared and can be bypassed by the person it stops; CI
  is the answer nobody can bypass, and is the only one that can gate a tag. Deriving both
  from one workflow file is what stops the tag bar and the `main` bar from drifting apart.
- **CI only, leaving the hooks unwired**: rejected. It is enforcement without a bypass,
  which is the stronger property, but it moves every violation to a push that has already
  happened — the commit vocabulary in particular is only cheap to fix while the commit is
  still the one being written, and a commit-msg gate reporting a violation after the push
  reports the right violation one commit too late to be the answer to it.
- **Hooks only, leaving CI as it was**: rejected. `--no-verify` is git's own bypass and
  always works, so hooks alone make every gate optional at the discretion of whoever is
  pushing. They are a convenience for the author, not a guarantee for the repository.
- **Leave the hooks failing open and only add CI**: rejected, and it is the option that
  produced the incident. A hook that exits zero when it could not run reports a pass for
  a gate that did not exist, which is strictly worse than having no hook: the absence is
  visible and the false pass is not. The cost of closing it is deliberate and is the
  behaviour change — pushes that used to sail through a silent no-op now stop.
- **Restate the check list in each workflow**: rejected. Two hand-maintained lists agree
  until someone edits one of them, and the one that would have been edited last is the
  release, where the disagreement is published rather than merely wrong.
- **Enforce by convention, in the standards and the pull request checklist**: rejected.
  That is the state the audit found, and it is what this framework exists to remove: the
  checklist asked for gates that ran nowhere, so every pull request could truthfully tick
  a box for a gate no one had executed.

## Consequences

- Pushes and commits that previously succeeded now stop. A missing `mf` binary, an
  undiscoverable repository, or a failing check ends the action; `git push --no-verify`
  and `git commit --no-verify` remain, and `github.md`'s checklist requires a bypass to be
  recorded in the pull request. A bypass is a stated fact rather than a silent one.
- Wiring the gates to CI made this repository's own history fail them until every rule
  they enforce was satisfied. That is the intended direction: when a gate proves to encode
  a rule the project does not actually want, the standard is what changes, not the gate.
- The gates run from the binary built out of the tree under test, never from a released
  one, so a change that breaks a gate can fail its own pull request.
- A pull request is checked out on a detached HEAD at a merge commit GitHub built, where
  three of these gates — spec, commit, branch — have no branch and no base to ask about.
  The job restores both refs before running them, so it reports on the change rather than
  on the checkout. A tag and a push to `main` have no branch under review at all, and
  there the job runs the four gates that are properties of the tree.
- The R2 review stays advisory and stays separate. It calls a model and can take minutes,
  so `SKIP_R2_REVIEW=1` skips it alone and never the deterministic gates, which are free.
  A chain where no backend was available still does not block a push: a reviewer that
  never ran is not a finding, and an expired quota must not be able to lock a repository.
- `mf check` becomes load-bearing for everyday work rather than a command someone
  remembers to run, which raises the cost of a gate that is wrong. A false positive now
  blocks a push instead of printing a line, and that is the risk this decision accepts.
