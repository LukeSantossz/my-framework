# SPEC: chore(standards): align global and repo standards into one coherent rule set

## Problem
The user's global `CLAUDE.md` and the repo standards contradict each other in
four places (ambiguity policy, an unqualified "Gate", a duplicated VAR table,
and the deprecated "Self-Review" section name), and three smaller gaps blur
the review machinery (CRURA predates the R1/R2/R3 composition, the badges
rule reads as conflicting with the honesty ethos, and no standard records
that CodeRabbit is the R3 actually wired on this repo).

## Design Decision
One coherence pass in two parts. Repo side (this PR, three parallel doc
clusters): `code_conventions.md` gains an explicit authority rule — a repo's
standards override user-global defaults, with Safety and Correctness never
overridden — echoed in `INDEX.md`; `ai_guidelines.md`'s Declare Assumptions
becomes the agreed hybrid-by-cost policy verbatim (assume-and-declare when
cheap to reverse, one focused question when costly); `crura_method.md` is
rewritten to compose with R1/R2/R3 (human review as final arbiter fed by the
three layers, stages tied to today's artifacts); `github.md`'s Badges section
gains the resolving rationale (honesty is discharged by the mandatory Known
Issues section, not by the badge strip); `codex_review.md` records CodeRabbit
as the R3 wired on this repository. User side (executed at cycle end, outside
the PR, with a backup kept): the global `CLAUDE.md` is rewritten thin —
hybrid ambiguity wording, "Spec Gate" qualified, VAR table replaced by a
pointer to the repo's `var_method.md`, "Self-Review" renamed "PR Review
Checklist", and the repo-over-global precedence line added.

## Alternatives Considered
- Ask-first ambiguity policy everywhere (the current global text): rejected
  by the user — it blocks AFK and parallel work; the hybrid-by-cost rule is
  what the working sessions already practice.
- Keeping the global file rich but aligned (VAR table corrected in place):
  rejected by the user — duplication is standing drift risk; thin-with-
  pointer removes the class of bug.
- Making badges honest (show red CI): rejected in favor of documenting the
  separation of duties — badges communicate health for the shop window, the
  mandatory Known Issues section carries the honesty duty; hiding is only
  forbidden where the reader expects truth.
- Pinning the global `CLAUDE.md` content with a repo test: rejected — CI has
  no access to `~/.claude/`; the global file's criteria are verified by
  inspection at execution and recorded in the PR Evidence.

## Scope
- Includes (cluster B — authority and ambiguity):
  - `docs/standards/code_conventions.md` Precedence section: an explicit rule
    that a repository's standards override user-global defaults (for example
    a global `CLAUDE.md`), and that Safety and Correctness are never
    overridden by either side.
  - `docs/standards/ai_guidelines.md` Declare Assumptions: the hybrid policy
    stated as the single rule — state the assumption in one line and proceed
    when it is cheap to reverse; ask one focused question first when a wrong
    assumption is costly to reverse.
  - `docs/standards/INDEX.md` System Rules: one line recording the
    repo-over-global authority rule.
- Includes (cluster C — human review):
  - `docs/standards/crura_method.md`: stages tied to today's artifacts (R =
    the Self-Review section of `ai_guidelines.md` locally; RA = the Files
    Changed pass backed by the PR Review Checklist of `github.md`), plus a
    Review Composition paragraph: the human review is the final arbiter and
    consumes the R1/R2/R3 layers' recorded results rather than repeating
    them; the "reviews the same code at least 3 times" claim restated
    accurately in terms of the actual stages.
- Includes (cluster D — shop window and R3):
  - `docs/standards/github.md` Badges: the rationale sentence — badges
    communicate health; the honesty duty is discharged by the mandatory
    Known Issues section, never by the badge strip.
  - `docs/standards/codex_review.md`: a line recording that R3 on this
    repository is CodeRabbit (wired via the GitHub app), adjudicated in the
    PR discussion like any reviewer finding.
- Includes (guard tests): one guard per doc invariant above, placed in
  `scripts/test/docs-consistency.test.sh` at distinct anchors per cluster
  (B after `codex_review_doc_depinned`; C after the skills-guidelines guard;
  D before `repo_scripts_are_executable`) so the parallel branches merge
  cleanly.
- Includes (user side, at execution, not in the PR): rewrite
  `~/.claude/CLAUDE.md` to the thin form described in the Design Decision,
  after saving a timestamped backup next to it.
- Does NOT include:
  - Batch 2 items (durable specs + spec-lite, Issue Model generalization,
    framework README/versioning) — next cycle, own SPEC.
  - Any behavior change to scripts, hooks, CI, or templates, beyond the guard
    tests above and one extension adjudicated at R2: the docs-consistency
    deprecated-wording list gains the retired "only makes R2 concrete" claim,
    so the contradiction it named cannot reappear.
  - Changes to the repo `CLAUDE.md` (it already defers to the standards).
  - Changes to `var_method.md` content (the pointer moves, the table stays
    where it lives).

## Acceptance Criteria
- precedence_names_repo_over_global: `code_conventions.md` Precedence states
  the repo-over-global rule (guard grep for the rule's key phrase).
- ambiguity_policy_is_hybrid: `ai_guidelines.md` Declare Assumptions contains
  both halves of the hybrid rule, including "one focused question" (guard
  grep).
- index_records_authority_rule: `INDEX.md` System Rules records the
  repo-over-global authority line (guard grep).
- crura_composes_with_review_layers: `crura_method.md` references R1, R2, R3
  and the PR Review Checklist (guard grep).
- badges_rationale_present: `github.md` Badges section contains the Known
  Issues rationale (guard grep).
- r3_wired_reviewer_named: `codex_review.md` names CodeRabbit as the wired R3
  (guard grep).
- docs_consistency_passes_after_edits: the docs-consistency check passes on
  the tree with every edit above (including the reference scan over all
  standards bodies).
- global_claude_md_thin (verified by inspection at execution, outside CI —
  recorded with before/after evidence in the PR): hybrid ambiguity wording,
  "Spec Gate" qualified, VAR table replaced by a pointer, "PR Review
  Checklist" naming, precedence line present, backup saved.

## Reproducibility
- `bash scripts/test/docs-consistency.test.sh && bash scripts/test/docs-consistency.sh`
  — expected: all pass; `all checks passed.`
- `bash scripts/test/codex-review.test.sh && bash scripts/test/setup.test.sh`
  — expected: all pass (regression only; nothing in scope touches them).
- Versions: bash (Git for Windows), git ≥ 2.40, gh ≥ 2.40, Codex CLI 0.132.0.
  No randomness involved.

## Risks and Assumptions
- Assumes three parallel clusters editing distinct standards files, with
  guard tests at distinct anchors of one shared test file, cherry-pick
  cleanly; a trivial same-file merge conflict is resolved by the controller.
- Assumes rewriting the global `CLAUDE.md` affects every project the user
  works on; the thin form deliberately keeps Prohibited, Delivery, and
  Definition-of-done intact, and a timestamped backup precedes the rewrite.
- Assumes the hybrid ambiguity policy matches the user's working preference
  as approved in this cycle's design questions.
- Invalidated if the framework later hosts multiple repos with conflicting
  standards — the authority rule would need a hierarchy, not a single line.
