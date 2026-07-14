# CRUX Method

Change Review Understanding eXplanation. The review-time discipline that makes
comprehension of an implemented change a produced, checkable outcome: before a
change is reviewed, it is explained by a transient, interactive HTML explainer
generated outside version control. CRUX is an aid feeding the R1 and CRURA
review layers (see `ai_guidelines.md` Review Composition and `crura_method.md`);
it is advisory only, never a review layer and never a blocking gate. Durable
rationale continues to live in ADRs; CRUX adds no versioned per-change record.

## When It Runs

At review time, on an implemented change (a diff, branch, or Pull Request). It
does not govern build-time; it does not overlap the Test-First order.

## The Artifact

A single self-contained HTML file (inline CSS and JavaScript), written outside
the code repository with a filename prefixed by the current date in
`YYYY-MM-DD-` format so the files stay time-sorted and out of version control.
One long page with a table of contents and section headers, no tabs for the
top-level structure, basic responsive styling. Sections, in order: Background
(a deep, skippable version for beginners, then a narrow background directly
relevant to the change), Intuition (the essence, with concrete toy-data examples
and figures), Code (a high-level, grouped walkthrough of the changes), and Quiz
(five interactive multiple-choice questions testing real understanding, not
gotchas). Diagrams reuse a small number of families and are HTML, never ASCII;
code blocks use `<pre>` or a `div` with `white-space: pre`/`pre-wrap`, confirmed
before saving; callouts mark key concepts and edge cases. The full artifact
contract is recorded verbatim in
`docs/specs/0009-add-crux-review-explanation-method.md`.

## Behaviors

- Anti-slop: the explainer's prose is passed through the `humanizer` skill to
  remove signs of AI-generated writing before the final render.
- Quiz difficulty: the invocation accepts a difficulty of `easy`, `medium`, or
  `hard`, defaulting to `medium`.
- Wrong-answer remediation: when the Developer answers a quiz question
  incorrectly, the explainer reveals a deeper explanation of that concept before
  advancing, with a control that lets the Developer skip it and proceed.

## Placement and Fallback

CRUX feeds R1 and the CRURA Review; it is not R1, R2, or R3, and it never blocks
a ship. Fallbacks degrade deliberately, never silently. If the generating skill
is absent, no explainer is produced: the reviewer reads the diff directly and the
Pull Request notes the CRUX aid was absent — mirroring the R2 Codex-absent
fallback. If only `humanizer` is absent, the explainer is still produced and its
prose gets a manual anti-slop pass; the degraded step is flagged, not silent. The
skill and its install/verify path are recorded in `skills_guidelines.md`.
