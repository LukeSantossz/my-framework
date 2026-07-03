# SPEC: chore(framework): archive specs durably, generalize the issue model, and add the framework readme

## Problem
The framework discards every approved spec by overwriting the root `SPEC.md`,
its Issue Model only fits replacement-shaped changes, and the repository that
mandates a README model for adopters has no README of its own.

## Design Decision
Approved specs become durable: they are authored directly under
`docs/specs/NNNN-<slug>.md` (numbered like ADRs), the root `SPEC.md` is retired
and its six historical versions are backfilled from git history, and
`spec_method.md` gains a spec-lite tier for changes that need no recorded
trade-off; ADR 0002 records this amendment to the decision-records flow. The
Issue Model in `github.md` becomes type-agnostic (a conditional "Current State"
and a "Proposed Solution" replace the refactor-shaped sections) with the issue
template aligned and its literal title pre-fill removed. The repository gets a
`README.md` that follows its own README Model, carrying the adoption story and
the versioning policy. The change is executed as three parallel disjoint
clusters cherry-picked into one PR (the proven PR #5/#6 pattern), with one
guard block per cluster in `scripts/test/docs-consistency.test.sh`.

## Alternatives Considered
- Keep authoring at the root `SPEC.md` and archive a copy at merge: rejected —
  it creates dual maintenance when adjudication amends the spec mid-PR (which
  happened in PR #6), and no cheap guard can force the copy to stay equal to
  the original.
- Keep specs transient and rely on ADRs alone (the status quo per ADR 0001):
  rejected — an ADR records one curated decision, not the approved scope and
  acceptance criteria of a change; gate-approved intent is currently lost every
  cycle and survives only through git archaeology.
- Three separate PRs, one per cluster: rejected — it triples the R2/R3 review
  cycles for no isolation benefit; the single-PR disjoint-cluster pattern is
  proven on PRs #5 and #6.
- GitHub issue forms (YAML) to solve the literal title placeholder: rejected —
  a heavier migration with new syntax to maintain; removing the `title:`
  pre-fill from the existing markdown template achieves the same outcome.

## Scope
- Includes (cluster A — durable specs; owns `spec_method.md`, `CONTEXT.md`,
  `docs/adr/`, `docs/specs/`, `docs/standards/INDEX.md`,
  `scripts/test/docs-consistency.sh`):
  - `docs/standards/spec_method.md`: the durable home (`docs/specs/NNNN-<slug>.md`,
    authored there directly, numbered sequentially) and the spec-lite tier —
    permitted when the change needs no Design Decision worth recording; it
    keeps exactly the three Gate-checked sections (Problem, Scope with a
    non-empty "Does NOT include", Acceptance Criteria); if Alternatives
    Considered turns out to be needed, the spec is full-tier. The Spec Gate
    criteria are unchanged for both tiers.
  - Backfill `docs/specs/0001`–`0006` verbatim from git history, slugs drawn
    from each spec's own title, extraction points pinned:
    0001 `git show 1d1742a^:SPEC.md` (codex pre-push gate), 0002
    `git show 326bf49^:SPEC.md` (domain glossary), 0003 `git show 4f03e9e^:SPEC.md`
    (activation gap), 0004 `git show 6a0680f^:SPEC.md` (toolchain portability),
    0005 `git show 7e62d08^:SPEC.md` (maintenance batch), 0006
    `git show 619ea7d:SPEC.md` (standards coherence). Each extraction must
    start with `# SPEC:`; a wrong extraction point is corrected by searching
    adjacent commits, never by editing recovered content.
  - This spec becomes `docs/specs/0007-durable-specs-issue-model-readme.md`;
    the root `SPEC.md` is then deleted.
  - `docs/adr/0002-durable-spec-archive.md`: new ADR recording that specs are
    durable while the ADR remains the curated home for decision rationale
    (audience and lifetime differ); `docs/adr/0001-decision-records-flow.md`
    gains an amended-by-0002 note on its transiency claim.
  - `CONTEXT.md`: the SPEC.md and Alternatives Considered entries drop the
    overwritten-each-change wording in favor of the durable archive; a
    Spec-lite term is added.
  - `docs/standards/INDEX.md`: one System Rules line recording the durable
    spec archive.
  - Adjudicated during controller review, same invariant: the pre-existing
    transiency claims in the `INDEX.md` decision-records-flow rule and the
    `CONTEXT.md` decision-records intro are corrected to the durable-archive
    wording, and the cluster A guard is extended to pin both phrases.
  - `scripts/test/docs-consistency.sh`: `HYPOTHETICAL_REFS` gains `SPEC.md`
    with its justification comment (the artifact's generic name in prose;
    concrete specs live under `docs/specs/`), and the Check 3 comment example
    no longer cites the root `SPEC.md`.
- Includes (cluster B — issue model; owns `docs/standards/github.md` and
  `.github/ISSUE_TEMPLATE/issue.md`):
  - `github.md` Issue Model: Description, Context, and Acceptance Criteria stay
    mandatory; "Current Usage" becomes "Current State", conditional — included
    only when the issue changes existing behavior; "Recommended Alternative"
    becomes "Proposed Solution" (name plus reasons, criteria kept).
  - `github.md` README Model, What It Is: the classification list is made
    explicitly illustrative (e.g.-wording) so artifacts outside the list — such
    as a standards framework — can classify themselves honestly. This edit
    lives in cluster B only because cluster B owns `github.md`; no two
    clusters touch the same file.
  - `.github/ISSUE_TEMPLATE/issue.md`: section headers aligned to the
    generalized model; the `title:` pre-fill line removed so a literal
    placeholder can never ship as an issue title (the comment guidance stays).
- Includes (cluster C — framework readme; owns root `README.md` and `LICENSE`):
  - `README.md` at the repo root following the canonical README Model order,
    dogfooded: title with tagline; badges per the Badges rule (language, CI on
    `LukeSantossz/my-framework`, license badge only if the license lands);
    What It Is classifying the artifact as a development-standards framework;
    Engineering Decisions indexing ADR 0001 and ADR 0002 (number fixed by this
    spec, so cluster C can link it before integration); Getting Started as the
    adoption story (copy `docs/standards/`, `scripts/`, `.githooks/`, and the
    `.github/` templates, then run `bash scripts/setup.sh`); Project Status
    with the versioning policy; a mandatory Known Issues & Limitations section
    with real limitations (Windows exec-bit reliance, R2 requires a local
    Codex CLI, open backlog follow-ups).
  - `LICENSE` file (MIT, approved at the Gate; copyright Lucas Gonçalves) and
    the README License section and badge naming it.
  - Versioning policy approved at the Gate: semver git tags starting at
    v0.1.0 when this PR merges (the tag is created by the controller at merge,
    outside the PR diff); adopters record the tag they copied from.
- Includes (guards — one per cluster, distinct anchors in
  `scripts/test/docs-consistency.test.sh`):
  - Cluster A guard after `standards_authority_and_ambiguity_recorded`:
    spec_method names the durable home and the spec-lite tier; `docs/specs/`
    holds the numbered archive with `# SPEC:` headers; root `SPEC.md` absent;
    ADR 0002 present and ADR 0001 amended; CONTEXT.md transiency wording gone.
  - Cluster B guard after `docs_consistency_honors_docs_dir_override`:
    generalized section names present in `github.md` and the refactor-shaped
    ones absent; template headers match the model; no `title:` pre-fill in the
    template; the What It Is list reads as illustrative.
  - Cluster C guard after `badges_rationale_and_wired_r3_recorded`: root
    `README.md` exists, canonical section order holds, Known Issues &
    Limitations present, Engineering Decisions links both ADRs (string
    presence in the README, not file existence — ADR 0002 lands with cluster
    A), the `LICENSE` file exists and the README License section names MIT,
    and no HTML comments or `{...}` placeholders remain.
- Does NOT include:
  - The PR #5 deferred follow-ups (loud exec-bit guard on `git ls-files`
    failure, `sort -u` symmetry in the parity guard, allowlist scope comment
    for INDEX.md refs) — separate backlog, own cycle.
  - Changes to the repo `CLAUDE.md`, `AGENTS.md`, or `.gitignore` prose that
    mentions `SPEC.md` generically (still accurate; not scanned by the checker).
  - The `Each test maps to an Acceptance Criterion in SPEC.md` header comments
    in the three test scripts (generic prose about the method, not a file
    reference the checker scans).
  - Changes to the Type Table, the PR Model, the README Model's section order,
    CI workflows, hooks, or `setup.sh`.
  - Growing the deprecated-wording list in `docs-consistency.sh`; if R2/R3
    adjudication extends it, this exclusion is amended in the same cycle (per
    the recorded SPEC-drafting lesson).

## Acceptance Criteria
- spec_archive_holds_all_prior_specs: `docs/specs/0001`–`0007` exist, each
  file starts with `# SPEC:`, and 0001–0006 match their pinned extraction
  points (cluster A guard).
- spec_method_names_durable_home_and_lite_tier: `spec_method.md` contains the
  `docs/specs/` home, the numbering scheme, and the spec-lite tier with its
  no-Design-Decision trigger (cluster A guard).
- root_spec_retired_and_allowlisted: the root `SPEC.md` is absent and the
  docs-consistency check passes with `SPEC.md` justified in
  `HYPOTHETICAL_REFS` (cluster A guard plus checker run).
- decision_flow_amended: `docs/adr/0002-durable-spec-archive.md` exists,
  ADR 0001 carries the amended-by note, and `CONTEXT.md` no longer states that
  the active spec is overwritten by the next change (cluster A guard).
- issue_model_sections_generalized: `github.md` Issue Model contains "Current
  State" (conditional) and "Proposed Solution", and no longer contains
  "Current Usage" or "Recommended Alternative" (cluster B guard).
- issue_template_aligned_without_prefilled_title: the template's section
  headers match the generalized model and the template has no `title:`
  pre-fill line (cluster B guard).
- readme_model_classification_nonexhaustive: the What It Is classification in
  `github.md` reads as illustrative (cluster B guard).
- framework_readme_follows_canonical_order: root `README.md` exists and its
  section headers appear in the canonical order (cluster C guard).
- readme_engineering_decisions_index_adrs: the README Engineering Decisions
  section links `docs/adr/0001-decision-records-flow.md` and
  `docs/adr/0002-durable-spec-archive.md` (cluster C guard).
- readme_known_issues_present_and_no_placeholders: Known Issues & Limitations
  is present with content, and the README contains no HTML comments and no
  `{...}` placeholders (cluster C guard).
- license_present_and_named: the root `LICENSE` file exists with the MIT terms
  and the README License section names MIT (cluster C guard).
- docs_consistency_passes_on_final_tree: `bash scripts/test/docs-consistency.sh`
  exits 0 on the assembled tree.
- all_test_suites_green: the three suites pass on the assembled tree.

## Reproducibility
- `bash scripts/test/docs-consistency.test.sh && bash scripts/test/docs-consistency.sh`
  — expected: all pass; `all checks passed.`
- `bash scripts/test/setup.test.sh && bash scripts/test/codex-review.test.sh`
  — expected: all pass (regression only; nothing in scope touches them).
- Versions: bash (Git for Windows), git ≥ 2.40, gh ≥ 2.40, Codex CLI 0.132.0.
  No randomness involved.

## Risks and Assumptions
- Assumes the three clusters are file-disjoint (the only shared file is
  `scripts/test/docs-consistency.test.sh`, edited at three distinct anchors)
  and cherry-pick cleanly; the controller resolves any trivial same-file
  conflict, as proven in PRs #5 and #6.
- Assumes the README's link to ADR 0002 resolves only after integration
  (cluster A creates the file); no checker scans README references, and the
  controller verifies the assembled tree before push.
- Adding `SPEC.md` to `HYPOTHETICAL_REFS` means a future dangling literal
  `SPEC.md` reference is never flagged; accepted because the name becomes
  generic prose by design once no root file exists.
- Assumes the historical spec bodies extract cleanly at the pinned commits;
  each must start with `# SPEC:`, and a failed extraction is resolved by
  searching adjacent commits, never by editing recovered content.
- Gate decisions recorded 2026-07-03: MIT license, semver tags starting at
  v0.1.0 at merge, and merge pre-authorized on a clean R2/R3 outcome.
- Invalidated if the Developer prefers keeping the root `SPEC.md` working-copy
  convention — cluster A reshapes to author-at-root with archive-at-approval.
