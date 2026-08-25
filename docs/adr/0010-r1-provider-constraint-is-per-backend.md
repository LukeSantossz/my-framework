# R1's provider constraint is a per-backend property, not a layer requirement

`CONTEXT.md` defines R1 as "the automated same-Provider review — the Superpowers
two-stage subagent pass (Claude)". Two things are wrong with that as a requirement.
Superpowers is not installed, so R1 is a declared layer with no executor — the same
failure `docs/specs/0013-detach-r2-from-codex.md` fixed for R2, where one hard-wired
executor meant the layer silently stopped existing while still reporting success. And
"same provider" is not a quality R1 has; it is a description of what one tool happens to
do. Held as a requirement it is a liability, because a reviewer drawn from the Author's
own provider is the definition of self-bias — the documented tendency of a model to
prefer its own output.

So R1 becomes a backend chain like R2, with Superpowers retained as one backend among
several, and the provider constraint stops being a property of the layer. Each backend
declares its own provider identity, and the runner reports which backend ran and what its
provider was. R2 keeps its constraint, because there the cross-provider requirement is the
entire point. R1 keeps none, because there it buys nothing and costs self-bias.

What separates the layers is then not the provider but when they run, how much of the
change they see, and what they cost: R1 pre-commit over the staged diff and cheap; R2
pre-push over the branch against its base and cross-provider; R3 in CI over the Pull
Request.

## Status

Accepted.

## Considered Options

- **Provider constraint declared per backend — R1 unconstrained, R2 cross-provider
  (chosen)**: it keeps the constraint where it does work and removes it where it only
  imports self-bias, and it lets R1 run on a cheap model from any provider, which is what
  makes running it on every commit affordable.
- **Keep R1 as a same-provider layer with the harness enforcing the match**: rejected at
  the Developer's decision — it preserves the glossary unchanged, but it keeps self-bias
  as a deliberate rule and requires the harness to know the Author's provider for a layer
  that gains nothing from knowing it.
- **Drop R1 entirely, since R2 reviews the same change**: rejected — R1 is the cheap,
  early, always-available pass, and removing it moves every defect to the pre-push gate,
  where acting on it costs more.
- **Keep Superpowers as R1's sole executor**: rejected at the Developer's decision to keep
  it configured rather than mandatory. A layer with exactly one possible executor is the
  failure `0013` already diagnosed.

## Consequences

- `CONTEXT.md`'s R1 entry and the Review Composition section of
  `docs/standards/ai_guidelines.md` change: R1 is no longer described as same-provider.
- Superpowers stays configurable and stays first-class, but it is an `in-session` backend.
  It runs inside a Claude Code session and cannot be invoked as a subprocess, so its
  participation is an instruction plus an attestation rather than an execution. A run
  satisfied that way says so, and an absent session counts as unavailability, so the chain
  advances instead of stalling.
- R1 can run with a cheap model from any provider, on every commit.
- The Review Composition hierarchy keeps its shape: three layers meeting at the Pull
  Request, composing rather than replacing one another.
- _Amended by `docs/specs/0027-close-the-audit-pendings.md`_: R1's half of the separation
  stated above is decided and not implemented. R2 runs from `.githooks/pre-push` and R3
  from `.github/workflows/review.yml`, but there is no pre-commit hook anywhere: the
  versioned directory carries `pre-push` and `commit-msg` and nothing else, and
  `mf hooks install` only points `core.hooksPath` at that directory, so git has no R1
  hook to discover. `docs/specs/0019` deferred "a commit-msg or pre-commit hook" to
  `0020`, and `0020` scoped its wiring to pre-push and commit-msg; the pre-commit trigger
  fell out there and was never picked up. Until it is, R1 runs when a person runs
  `mf review --role r1`, and "over the staged diff" is wrong in both halves — nothing
  invokes it at commit time, and the runner diffs the branch against its base rather than
  the index. The sentence above is the record of what was decided, not a description of
  what the harness does.
