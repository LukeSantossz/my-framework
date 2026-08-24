# SPEC: docs(standards): realign Review Composition around backend chains and Author provenance

## Problem

Review Composition defines R1 as the same-provider Superpowers pass and lets R2's
cross-provider requirement rest on an Author identity nothing records, so R1 names an
executor that is not installed and R2 cannot tell an enforced cross-provider claim from
an unverified assumption.

## Design Decision

Rewrite the review layers in the standards to match `docs/adr/0010-r1-provider-constraint-is-per-backend.md`
and `docs/adr/0007-author-provenance-belongs-to-the-change.md`, before any binary exists
to implement them. The standards state what the runner is required to do; writing them
after the code would make them a transcript of whatever got built, which is the inversion
the SPEC Method exists to prevent.

R1 stops being defined by its executor. It becomes a role satisfied by a chain of
backends, each declaring its own provider identity, with `superpowers` named as one
backend among several rather than as the layer itself. The same-provider requirement is
dropped from R1 and kept on R2, where it is the entire point. What distinguishes the
layers becomes when they run, how much of the change they see, and what they cost.

The Author stops being an assumed session fact and becomes a recorded property of the
change, declared per branch at authoring time. R2's cross-provider claim gains three
states — `verified`, `declared`, `unknown` — and `unknown` does not satisfy R2. The PR
review-layers record carries the state alongside the backend and model, because the
human adjudicating a PR is the one who needs to tell an enforced claim from an asserted
one, and the record exists for exactly that judgment.

Because `superpowers` cannot be invoked as a subprocess, the standards name its
participation honestly: it is an in-session backend whose contribution is an instruction
plus an attestation, and an absent session counts as unavailability so the chain
advances. This is the same honesty the R2 gate already applies to a backend that is out
of quota.

This slice is documentation only. Its tests are guards in
`scripts/test/docs-consistency.test.sh`, written before the documents are edited, which
is the repository's existing practice for a normative change.

## Alternatives Considered

- **Change the standards after the binary exists.** Rejected. The standards are what the
  binary is required to implement, so writing code first makes the documents a record of
  whatever was built rather than a specification of what should be — the inversion
  `spec_method.md` exists to prevent.
- **Fold this into the role-runner slice.** Rejected. It mixes a normative change with an
  implementation, so a reviewer cannot approve one without accepting the other and the
  Spec Gate stops discriminating.
- **Edit `ai_guidelines.md` only and leave the `CONTEXT.md` glossary alone.** Rejected.
  `CONTEXT.md` is the declared source of truth for what each term means; leaving R1
  defined there as a same-provider review while `ai_guidelines.md` says otherwise
  manufactures exactly the contradiction between standards that the docs-consistency
  deprecated-wording check exists to catch.
- **State the three cross-provider states without changing the PR record.** Rejected. A
  state the runner computes and the PR never shows is a state nobody acts on, and the
  review-layers record is the one place the distinction changes a human decision.

## Scope

- Includes:
  - `docs/standards/ai_guidelines.md`: the Review Composition and Cross-Provider Review
    sections.
  - `docs/standards/r2_gate.md`: the Roles section, the Behavior section, and Recording
    in the PR.
  - `CONTEXT.md`: the `R1 / Internal Review`, `R2 / Cross-Provider Review`, `Author`,
    `Provider` and `Reviewer` entries, plus new entries for the Author Declaration, the
    Backend, the Role and the Cross-Provider State.
  - `docs/standards/skills_guidelines.md`: the pipeline stage of the `## Superpowers`
    entry, restated as one backend of the R1 chain, keeping the heading the existing
    inventory guard pins.
  - `docs/standards/INDEX.md`: the System Rules line describing Review Composition.
  - `docs/standards/github.md` PR Review Checklist and
    `.github/PULL_REQUEST_TEMPLATE.md`: both currently read "internal Superpowers review
    (R1)" and both gain the cross-provider state.
  - New guards in `scripts/test/docs-consistency.test.sh`, written first.

- Does NOT include:
  - Any Go code, any binary, any executable behavior. This slice changes what the
    standards require and nothing that runs.
  - The configuration schema that will carry the chains; that is the configuration slice.
  - The CRURA re-scope; that is its own slice, and `crura_method.md` is untouched here.
  - Naming which environment variable identifies which agent. The standard fixes the
    three states and the loud-error rule on a contradiction; the fingerprint mechanism is
    an implementation detail of the runner slice.
  - Renaming R1, R2 or R3.
  - Any change to R3.
  - Weakening or removing the test-first order clauses that the existing
    `superpowers_claim_is_not_enforcement` guard pins in `INDEX.md`,
    `code_conventions.md` and `ai_guidelines.md`.

## Acceptance Criteria

- `context_md_no_longer_defines_r1_as_a_same_provider_review`
- `context_md_defines_the_author_declaration_and_the_three_cross_provider_states`
- `ai_guidelines_describes_r1_as_a_chain_of_backends_naming_superpowers_as_one_of_them`
- `ai_guidelines_and_r2_gate_state_the_same_three_cross_provider_states`
- `r2_gate_records_that_unknown_does_not_satisfy_r2`
- `r2_gate_records_that_an_in_session_backend_contributes_an_attestation_not_an_execution`
- `skills_guidelines_superpowers_entry_names_r1_as_a_chain_it_participates_in`
- `skills_guidelines_keeps_the_six_headings_the_inventory_guard_pins`
- `index_md_review_composition_rule_matches_ai_guidelines`
- `github_md_and_the_pull_request_template_carry_the_cross_provider_state`
- `github_md_and_the_pull_request_template_state_the_same_review_layers_record`
- `no_standard_still_calls_r1_the_superpowers_review`
- `superpowers_claim_is_not_enforcement_still_passes_after_the_edits`
- `docs_consistency_still_passes_on_the_edited_tree`

## Reproducibility

- `bash scripts/test/docs-consistency.test.sh`
- `bash scripts/test/docs-consistency.sh`

No model is called and no network is used, so the run is deterministic and needs no
version pinning beyond the shell.

## Risks and Assumptions

- **The standards will describe a runner that does not exist yet.** Between this slice
  and the runner slice, the documents specify behavior nothing performs — which is the
  framework's own Gap in miniature. It is accepted only because the interval is short and
  the ordering is deliberate, and it is mitigated the way `skills_guidelines.md` already
  handles an absent capability: the documents say plainly which part has no executor yet
  and what stands in for it, rather than implying coverage that does not exist.
- **Dropping "same provider" from R1 removes a stated guarantee.** It weakens nothing
  operationally, because R1 has no executor today, but a reader of an older Pull Request
  will find R1 described one way there and another way in the current standard. The
  durable spec archive is what preserves the earlier meaning, and this spec is the record
  of when it changed.
- **The new guards pin wording, and wording guards are brittle.** This repository already
  carries several, and each new one must pin the claim rather than the sentence;
  otherwise routine editing turns into CI failure and the guard gets weakened to silence
  it, which is worse than not having it.
- **The PR template change reaches open Pull Requests.** A PR opened before this slice
  carries the old checklist, so its review-layers record will lack the state. That is
  history rather than a defect, and no PR is rewritten to add it.
- **The assumption that `superpowers` is worth keeping configured is the Developer's,
  not evidence.** The plugin is still not installed, so this slice records a backend that
  nothing has yet exercised; if it stays uninstalled indefinitely, a later slice should
  ask whether naming it is documentation of an intention rather than of a capability.
