## Isolated-Context Review

Two roles played by one model: as Author it develops the code; as Reviewer, in a context isolated from the authoring one, it reviews the change before the PR. This is layer R2 above, and `isolated_review.md` defines it.

- The Author completes Self-Review above, then writes the change.
- Before the branch is pushed, the diff goes to an isolated context that did not see the change being written. It receives the diff, the approved spec and the standards, and nothing of the authoring conversation.
- The Reviewer applies the same standards (`code_conventions.md`, `var_method.md`, this file) and reports: correctness defects, invented or unverified symbols, scope creep, security issues, convention violations.
- The Reviewer does not rewrite the code; it reports findings. The Author resolves them or justifies the decision in the PR.
- Record in the PR the model that authored and that it reviewed in an isolated context, as a same-provider review.
- When no isolated context can be opened, the fallback is the one stated in Review Composition above — R1 plus CRURA's adjudication stage — and the absence is noted in the PR.
- A Reviewer finding is advisory, not binding, but an unresolved finding must be addressed or justified, never silently dropped.
