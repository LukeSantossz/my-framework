# CRURA becomes triggered review, shipped instrumented

The CRURA Method prescribes an always-on human review track on every change. The argument
against that shape is specific and it is about attention, not about competence: if a
system is good enough that intervention is rare, staying vigilant is close to impossible;
if it is bad enough to keep a reviewer vigilant, it is slowing them down. There is no
accuracy at which "a human reviews the model's output" is simultaneously effective and
valuable. Fixed-rate attention is how alert fatigue is manufactured, and human attention
is the scarcest resource in this pipeline.

The answer is not to delete the human layer. This framework already implements the
inversion that critique implies and calls it the Spec Gate: the human decides first —
problem, scope, acceptance criteria — and the machine checks conformance afterwards. So
CRURA's always-on obligation becomes the Spec Gate, and post-change human review becomes
triggered: by a blocking or security-category finding, by a failing deterministic check,
or by a change touching a declared high-risk path. Because this is a bet rather than a
proof, it ships instrumented — each human review records whether it found a defect the
automated layers missed.

## Status

Accepted.

## Considered Options

- **Re-scope to triggered review, with instrumentation (chosen)**: it moves the
  mandatory human act to where the human has an independent basis for judgment, and it
  makes the bet falsifiable instead of permanent.
- **Leave CRURA unchanged and only record the tension**: rejected — recording a known
  defect in a standard while continuing to prescribe it is exactly the
  documentation-without-activation failure this framework exists to remove.
- **Instrument first and decide later**: rejected as the only step. Its discipline is
  right — treat a recommendation as a hypothesis until there is measurement — and the
  instrumentation is kept for that reason, but waiting keeps prescribing fixed-rate
  attention while the data accumulates, which is the cost the change exists to stop
  paying.
- **Invert completely**, with no scheduled post-change review at all and the harness
  interrupting only on disagreement with the stated acceptance criteria: rejected — the
  cleanest form of the argument, but it bets everything on acceptance criteria being
  complete, and they rarely are. The triggered form keeps a path for what the criteria
  did not anticipate.

## Consequences

- The Developer's mandatory, non-delegable act moves earlier: it is the Spec Gate, not
  the post-change read.
- The acronym is kept, but its content changes: "Review Again" becomes conditional rather
  than unconditional. The glossary has to say so, because a reader who knows the acronym
  will otherwise assume the old cadence.
- The triggers must be defined concretely and kept current, or "triggered" degrades into
  "never" — which would be a worse outcome than the fixed rate this replaces.
- The instrumentation depends on the Developer honestly recording whether a review found
  something new. That is the same honor-system problem the framework exists to remove,
  accepted here because no mechanism can know what a person noticed.
- The next cut is made from evidence rather than argument. If triggered review finds
  nothing the automated layers missed across a meaningful number of changes, that is the
  case for narrowing it further; if it finds a great deal, the trigger set was too narrow.
- `docs/standards/crura_method.md`, `docs/standards/ai_guidelines.md` and `CONTEXT.md`
  change; the Review Composition hierarchy of R1, R2 and R3 is untouched.
