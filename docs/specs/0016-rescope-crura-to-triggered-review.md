# SPEC: docs(standards): re-scope CRURA from always-on to triggered review with instrumentation

## Problem

The CRURA Method prescribes human review of every change at a fixed rate, which is the
shape the evidence condemns: at a quality level where intervention is rare, sustained
vigilance is not achievable, and at a quality level where vigilance is sustainable, the
review is costing more than it returns.

## Design Decision

Implement `docs/adr/0008-crura-becomes-triggered-review.md` in the standards, and do it
by splitting a conflation the current method carries rather than by weakening the method
wholesale.

CRURA has four stages, of which only two are review. **R** reads the changed files
locally before the commit. **RA** currently does two different jobs at once: it
adjudicates what the recorded review layers found, and it re-reads the diff line by line.
Only the second is fixed-rate verification of model output. The first is a decision, and
the Developer is accountable for what ships, so it cannot be delegated or skipped.

So the re-scope is precise. Adjudicating the recorded layers and making the merge
decision stay unconditional at RA. The line-by-line reads — R in full, and the re-read
half of RA — become triggered. The acronym is kept and the stages keep their names; what
changes is that two of them carry a condition, and the standard says which.

The always-on human obligation does not disappear, it moves earlier, to where the
Developer has an independent basis for judgment instead of verifying output against
nothing: the Spec Gate. That is the inversion the evidence argues for, and this framework
already implements it under another name.

Triggers are enumerated in the standard rather than left to judgment, because a trigger
set that is not written down degrades into "never" and the re-scope becomes a deletion.

The change ships instrumented: each human review records whether it found a defect the
automated layers missed, in a fixed vocabulary a later slice can count from merged Pull
Requests. Instrumenting only triggered reviews would be a broken measurement — it can
confirm the trigger set but never correct it, because the untriggered population is never
observed. So a periodic sample of untriggered changes is reviewed and recorded too, and
the standard says that the sample is what makes the evidence able to falsify the bet
rather than merely ratify it.

## Alternatives Considered

- **Make all of R and RA triggered, adjudication included.** Rejected. It would remove
  the human from the merge decision, and `CONTEXT.md` holds that the Developer, not any
  model, is accountable for what ships. Accountability without a mandatory decision point
  is a claim nobody can act on.
- **Keep CRURA unchanged and record the tension.** Rejected at the Spec Gate for
  `0014`, and rejected again here for the same reason: prescribing a discipline while
  documenting that it does not work is the documentation-without-activation failure this
  framework exists to remove.
- **Instrument only the triggered reviews.** Rejected. It is the cheaper measurement and
  it is the wrong one: observing only the changes already suspected produces evidence
  that can confirm the trigger set and never correct it. The untriggered sample is what
  makes the instrumentation falsifying rather than self-confirming.
- **Replace CRURA with a new acronym.** Rejected. The stages are unchanged and the term
  is referenced across the standards, the specs and the PR history; renaming would break
  those references to signal a change that a redefinition already carries.
- **Set triggers by change size rather than by finding.** Rejected. Size is a poor proxy
  for risk — a one-line change to an authentication path outranks a large mechanical
  rename — and it would make the trigger a function of the diff rather than of what the
  automated layers actually reported.

## Scope

- Includes:
  - `docs/standards/crura_method.md`: the Stages section, a new enumerated Triggers
    section, a new Instrumentation section, and the Composition section updated for the
    adjudication/re-read split.
  - `CONTEXT.md`: the `CRURA Review` entry, plus new entries for the Review Trigger and
    the Untriggered Sample.
  - `docs/standards/ai_guidelines.md`: the Review Composition sentence that has CRURA
    standing in for R2 when no second provider is available, which must now say which
    part of CRURA does so.
  - `docs/standards/github.md` PR Review Checklist and
    `.github/PULL_REQUEST_TEMPLATE.md`: the instrumentation field, in a fixed vocabulary.
  - `docs/standards/INDEX.md`: the System Rules line covering human review.
  - New guards in `scripts/test/docs-consistency.test.sh`, written first.

- Does NOT include:
  - Any Go code. Nothing here runs; the aggregation of the recorded results is a later
    slice.
  - Choosing the sampling rate for the untriggered sample as a fixed number. The standard
    requires a periodic sample and names who sets the period; pinning a rate before any
    data exists would be inventing a parameter.
  - Deciding CRURA's final shape. This slice ships the bet and the means to test it.
  - Changing the Spec Gate itself, which already exists in `spec_method.md` and is only
    referenced here as the relocated always-on obligation.
  - Changing R1, R2 or R3, whose realignment is `docs/specs/0015-realign-review-composition.md`.
  - Removing the human merge decision, or making any part of adjudication conditional.
  - Weakening the five clauses the existing `crura_composes_with_review_layers` guard
    pins in `crura_method.md`.

## Acceptance Criteria

- `crura_method_marks_the_line_by_line_reads_as_triggered`
- `crura_method_keeps_adjudication_and_the_merge_decision_unconditional`
- `crura_method_enumerates_its_triggers_rather_than_leaving_them_to_judgment`
- `crura_method_names_the_spec_gate_as_the_relocated_always_on_obligation`
- `crura_method_requires_a_periodic_sample_of_untriggered_changes`
- `crura_method_states_why_sampling_only_triggered_reviews_cannot_falsify`
- `context_md_defines_the_review_trigger_and_the_untriggered_sample`
- `context_md_no_longer_calls_crura_an_always_on_track`
- `github_md_and_the_pull_request_template_record_whether_review_found_something_new`
- `github_md_and_the_pull_request_template_use_the_same_fixed_vocabulary`
- `ai_guidelines_names_which_part_of_crura_stands_in_for_r2`
- `index_md_human_review_rule_matches_crura_method`
- `crura_composes_with_review_layers_still_passes_after_the_edits`
- `docs_consistency_still_passes_on_the_edited_tree`

## Reproducibility

- `bash scripts/test/docs-consistency.test.sh`
- `bash scripts/test/docs-consistency.sh`

No model is called and no network is used, so the run is deterministic.

## Risks and Assumptions

- **The instrumentation depends on honest self-report.** Nothing can know what a person
  noticed, so the record of whether a review found something new is an attestation. This
  is the honor-system problem the framework exists to remove, accepted here because the
  alternative is no evidence at all — but it means the resulting data is weaker than a
  measurement and must be read as such.
- **Triggered review produces less data, not more, exactly where the bet is boldest.**
  If triggers rarely fire, almost nothing is observed, and the evidence needed to
  evaluate the re-scope accumulates most slowly under the regime that most needs
  evaluating. The untriggered sample is the whole mitigation; if it is skipped in
  practice, the instrumentation degrades into a record that can only ever agree with the
  trigger set.
- **An enumerated trigger set ages.** Triggers reference finding categories and check
  names that later slices will change, so the list needs revisiting whenever the review
  runtime gains a category. A stale trigger list fails silently — it simply stops firing.
- **The re-scope reduces how often a human reads the diff, before the automated layers
  that justify the reduction actually exist.** R1 has no executor and the runner is not
  built, so for the duration of the intervening slices this trades human attention for
  machine attention that is specified rather than running. The standard must say so, in
  the same terms `0015` used, rather than implying the trade has already been made.
- **The sampling period is left to the Developer.** That is honest given no data exists,
  but an unset period is indistinguishable from a period of never, and nothing in this
  slice detects that.
