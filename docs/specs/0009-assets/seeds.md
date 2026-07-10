# SPEC 0009 seeded diffs (immutable fixtures)

The exact content of the three seeded defect files used in the benchmark, preserved
so the scored cells can be reproduced. Each was committed on a throwaway branch as a
single new file under `scripts/bench/`, then reviewed with
`codex review --commit <SHA> -c model="<model>" -c model_reasoning_effort="high"`.

> These are **intentional negative-test fixtures**, not framework code. Each contains
> exactly one planted defect and must not be copied into real scripts or executed.

## seed1 — correctness (inverted base-branch guard)

The `!=` inverts the stated guard: it skips on every feature branch and runs only on
the base branch.

```bash
#!/usr/bin/env bash
# Guard: skip the review when we are on the base branch (nothing to review
# against itself); otherwise run the review for the current feature branch.
set -u
base="${REVIEW_BASE:-main}"
branch="$(git rev-parse --abbrev-ref HEAD)"

if [ "$branch" != "$base" ]; then
  echo "On base branch '$base'; nothing to review. Skipping."
  exit 0
fi

echo "Running review for $branch vs $base"
codex review --base "$base"
```

## seed2 — invented symbol (non-existent CLI flags)

`git rev-parse --branch-name` and `codex review --format=json` are both invented; the
CLI has neither.

```bash
#!/usr/bin/env bash
# Emit the review as JSON, tagged with the current branch name.
set -u
branch="$(git rev-parse --branch-name)"
codex review --base main --format=json > "review-${branch}.json"
echo "wrote review-${branch}.json"
```

## seed3 — security (command injection via eval)

`eval` on the unsanitized positional argument executes attacker-controlled shell.

```bash
#!/usr/bin/env bash
# Dispatch a named review hook supplied by the caller.
set -u
hook="${1:-}"
eval "review_hook_${hook}"
```

## Real diffs

The two clean diffs are existing commits in this repository, reviewed with
`codex review --commit <SHA>`: `c3a891b` (exec-bit guard hardening) and `5ff245c`
(`sort -u` dedupe). No planted defect; a disciplined reviewer returns no finding.
