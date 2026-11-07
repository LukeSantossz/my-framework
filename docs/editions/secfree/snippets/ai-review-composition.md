- R1, internal review: performed by the Author in the authoring session. It applies `code_conventions.md`, `var_method.md`, and this file, and stands in for the Author's Self-Review.
- R2, isolated-context review: the same model as the Author, in a context that did not see the change being written, reviews the diff before the branch is pushed. It is not a cross-provider review and is never recorded as one.
- R3, isolated-context PR review: the same model, in a second isolated context, reviews the pull request as a whole. It is additional signal and does not substitute for R2.

How the contexts are kept apart, and how the pull request records each layer, is defined in `isolated_review.md`.

When an isolated context cannot be opened, R1 plus the adjudication stage of CRURA (per `crura_method.md`) stand in for R2; note its absence in the PR. It is the adjudication that substitutes, not the line-by-line read, because adjudication is the part that runs unconditionally.

The Author's Self-Review is not part of that substitution. R1 already stands in for it, per R1 above, so naming it again here would offer as a replacement the thing R1 replaced — leaving a change whose only human check is the one CRURA makes triggered.

A layer that did not run is recorded as not having run: it is never the same as a layer that found nothing.
