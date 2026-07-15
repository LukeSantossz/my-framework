# SPEC: chore(standards): restore the reviewer-switch record and harden the numbering and claim guards

## Problem

PR #10 deleted the approved spec and the ADR that recorded the `gpt-5.5` →
`gpt-5.6-terra` reviewer switch, so the durable archive states the old default
while the scripts run the new one with nothing reconciling them; the same commit
weakened the guard that would have caught the deletion, the deleted numbers were
reused for unrelated records, and three further guards pin claims that are stale,
literal-scoped, or incomplete.

## Design Decision

Restore the reviewer-switch decision as a new ADR `0004` rather than reinstating
it at its original number `0003`, which the CRUX explainer decision now occupies
and which the README and the test suite already reference. Recycling a durable
number a second time would repeat the defect this spec exists to close, so the
restored ADR carries the numbering history in its own text: it records that
`0003` and spec `0009` once held these records, that PR #10 deleted them, and
that the numbers were reused. The restored content is recovered verbatim from
`git show 3cc2ba0:docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md`, not
rewritten, so the benchmark rationale survives as authored.

The rule the deletion broke is then made explicit and guarded. `spec_method.md`
gains one rule: a durable number is never reused, and a record that is retired
stays in place marked Retired rather than being deleted. The
`durable_spec_archive_recorded` guard regains its teeth in a form that survives
new records: instead of a frozen number list (which PR #10 replaced with a glob
that requires nothing), it asserts that spec and ADR numbers run contiguously
from `0001` with no gaps and no duplicates, so any future deletion fails CI
without the guard needing an edit per record.

The R3 discrepancy is closed by changing the repository rather than the document.
`codex_review.md` claims CodeRabbit is this repository's R3, while
`copilot-pull-request-reviewer` has in fact reviewed more PRs (9 of 11) than the
CodeRabbit the document names as the only one. The Developer's decision is that
R3 is CodeRabbit alone: the automatic Copilot code review is disabled in the
repository settings, which makes the existing claim true rather than rewriting it
to describe a second reviewer that is not wanted. The document and its guard are
therefore unchanged, and the correction is an operational step recorded here.

The two remaining guards are corrected to pin what they claim.
`codex_review_doc_depinned` stops forbidding the single literal `claude-opus-4-8`
and forbids the pattern of any concrete Anthropic model id, with the illustrative
Author pin in the Recording section replaced by a placeholder so the example
cannot go stale the way `claude-opus-4-8` did. `setup.test.sh` pins the `high`
effort default alongside the model default it already pins.

## Alternatives Considered

- **Renumber the CRUX ADR back to `0004` and restore the reviewer ADR at `0003`**:
  rejected — it restores the original numbering at the cost of moving a durable
  record that `README.md` and two guards already reference, which is the same
  reference-breaking move that caused the defect. Numbers are cheap; stable
  references are not.
- **Restore the deleted spec `0009` as well as the ADR**: rejected — spec `0009`
  is occupied by the CRUX method and its 143-line benchmark body is
  disproportionate to the decision, which is precisely the judgment PR #10 made
  and which this spec does not reopen. The ADR is the curated home for the
  rationale per ADR 0002; restoring it recovers the decision without the archive
  bloat. The benchmark numbers survive inside the restored ADR.
- **Leave the numbering as-is and only document the reuse in a note**: rejected —
  a note records the accident without preventing the next one; the guard is what
  makes the rule binding, and the framework's own premise is that an unactivated
  standard is no standard.
- **Restore the guard's frozen number list (`0001`–`0009`)**: rejected — it needs
  a manual edit per new spec and silently rots, which is why PR #10 removed it.
  Contiguity is the same invariant expressed in a form that does not decay.
- **Complete the R3 claim by documenting both wired reviewers**: rejected by the
  Developer — a stale claim can be closed from either side, by correcting the
  document or by correcting the repository, and the Developer wants one automated
  PR reviewer rather than two. Disabling the Copilot automatic review makes the
  existing CodeRabbit claim true and keeps R3 a single, adjudicable signal instead
  of two overlapping ones. This is the chosen approach.
- **Drop the R3 CodeRabbit claim entirely**: rejected — CodeRabbit is genuinely
  wired and its findings are adjudicated in the PR; the defect was never the
  presence of CodeRabbit.

## Scope

- Includes:
  - `docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md`: the reviewer-switch
    decision restored verbatim from `3cc2ba0`, with a numbering-history note
    recording the PR #10 deletion and the `0003`/`0009` reuse, and its
    `docs/specs/0009-...` benchmark pointer corrected to name the retired spec
    rather than dangle.
  - `README.md`: an Engineering Decisions row linking ADR 0004; the
    `Codex CLI 0.144.1` prerequisite relaxed to a minimum (`>=`) to match the
    `git`/`gh` entries beside it.
  - `docs/standards/spec_method.md`: one rule stating that a durable number is
    never reused and a retired record is marked, not deleted.
  - `docs/standards/codex_review.md`: the Recording in the PR example replaces its
    concrete Author model with a placeholder.
  - An operational step, outside the versioned diff: the Developer disables the
    automatic Copilot code review in the repository settings, so that the
    document's existing "R3 is CodeRabbit" claim describes the repository. There
    is no ruleset and no REST API for this toggle; it is a settings change only
    the Developer can make.
  - `scripts/test/docs-consistency.test.sh`: `durable_spec_archive_recorded`
    asserts contiguous, gapless, duplicate-free spec and ADR numbering;
    `codex_review_doc_depinned` forbids the Anthropic-model-id pattern rather
    than one literal; a guard pins the ADR 0004 file, its README row, and the
    spec_method numbering rule.
  - `scripts/test/setup.test.sh`: a pin on the `high` effort default.
  - Every change lands test-first (red, then green), per `code_conventions.md`
    Testing.
- Does NOT include:
  - Restoring spec `0009` or renumbering the CRUX ADR `0003` / spec `0009`.
  - Editing any archived spec under `docs/specs/0001`–`0009`; the archive is
    immutable and the `0001`/`0004` Codex CLI version discrepancy stays as
    authored, recorded here as a known limitation rather than corrected.
  - Changing the R2 gate's advisory behavior, resolution precedence, or the
    `gpt-5.6-terra` / `high` defaults themselves.
  - Adding a CodeRabbit or Copilot configuration file to the repository, or
    editing the R3 paragraph in `codex_review.md` and its
    `badges_rationale_and_wired_r3_recorded` guard; both stay as authored, and the
    Copilot reviewer is removed from the repository instead.
  - Naming `copilot-pull-request-reviewer` in any standard.
  - Any commit, branch, tag, or push; the working tree is left for the Developer.
  - Adding `copilot-pull-request-reviewer` to the `CONTEXT.md` R3 term, whose
    `e.g.` wording is already non-exhaustive.

## Acceptance Criteria

- reviewer_switch_adr_restored: `docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md`
  exists, states `gpt-5.6-terra` as the chosen default, and records that PR #10
  deleted the original records and that `0003`/`0009` were reused.
- readme_indexes_reviewer_adr: the README Engineering Decisions table links
  `docs/adr/0004-r2-reviewer-model-gpt-5-6-terra.md`.
- spec_method_forbids_number_reuse: `spec_method.md` states in a grep-detectable
  form that a durable number is never reused and a retired record is marked, not
  deleted.
- spec_numbering_is_contiguous: the guard fails on a fixture whose spec numbers
  skip a value or repeat one, and passes on the real tree.
- adr_numbering_is_contiguous: the guard fails on a fixture whose ADR numbers
  skip a value or repeat one, and passes on the real tree.
- codex_review_doc_rejects_any_model_pin: the guard fails when `codex_review.md`
  contains any concrete Anthropic model id (not only `claude-opus-4-8`), and the
  current document contains none.
- r3_is_coderabbit_alone: the automatic Copilot code review is disabled in the
  repository settings, so that no reviewer other than CodeRabbit posts an
  automated review on a new PR. Verified on the first PR opened after the change,
  not by a shell test: this is a GitHub settings state, outside the tree the
  suites can read. `codex_review.md` and its guard are unchanged and continue to
  pass.
- setup_pins_effort_default: `setup.test.sh` fails when the `high` effort default
  literal in `setup.sh` changes, as it already does for the model default.
- readme_codex_prerequisite_is_a_minimum: the README Codex CLI prerequisite reads
  as a minimum version, not an exact pin.
- all_suites_green: `docs-consistency.test.sh`, `docs-consistency.sh`,
  `setup.test.sh`, and `codex-review.test.sh` all pass on the final tree.

## Reproducibility

Run, from the repository root, with git >= 2.40 and bash (Git for Windows):

```sh
bash scripts/test/docs-consistency.test.sh
bash scripts/test/docs-consistency.sh
bash scripts/test/setup.test.sh
bash scripts/test/codex-review.test.sh
```

All four suites pass, 0 failed. The numbering guards need full history only for
the pre-existing `0001`–`0006` blob pins (CI already fetches with
`fetch-depth: 0`). The restored ADR body is recovered with
`git show 3cc2ba0:docs/adr/0003-r2-reviewer-model-gpt-5-6-terra.md`. No
randomness is involved.

## Risks and Assumptions

- Assumption: the Spec Gate approval for this scope is the Developer's approval
  of the seven-item correction list on 2026-07-15; the scope here is that list
  and nothing more.
- Risk, and the one open item in this spec: the CodeRabbit-only claim in
  `codex_review.md` is accurate only once the Developer disables the automatic
  Copilot code review. Until that setting is flipped, the document and its guard
  assert something the repository does not do — the same class of stale claim this
  spec exists to close, pointing the other way. The guard cannot detect this,
  because it greps prose and the truth lives in GitHub settings; it will keep
  passing while the claim rots. This is accepted deliberately, and closes when the
  setting changes.
- Assumption: no ruleset or REST API exposes the Copilot automatic-review toggle
  (verified: the repository has no rulesets and no requested reviewers), so the
  step cannot be automated or guarded from the tree and stays the Developer's.
- Risk: the contiguity guard rejects a deliberate future gap (for example a
  number reserved but not yet written). Mitigation: the rule is that numbers are
  assigned when the record lands; a reserved-but-absent number is the reuse
  hazard in another form.
- Risk: the restored ADR's benchmark cites prices and a GA date that were true in
  July 2026 and will age. Accepted: an ADR records a decision at its moment, and
  the alternative is losing the rationale entirely, which is the current state.
- Assumption: the `0001` (`codex-cli 0.140.0`) versus `0004`
  (`0.132.0, downgraded from 0.135.0`) discrepancy cannot be corrected without
  breaking the blob pins that make the archive verifiable; it is recorded as a
  known limitation, not fixed.
- What would invalidate this spec: a Developer decision that spec `0009` must be
  restored too, which reopens the PR #10 curation judgment and changes Scope.
