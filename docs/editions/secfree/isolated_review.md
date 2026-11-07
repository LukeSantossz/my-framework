# Isolated-Context Review

How the review layers run in the sec-free edition, where no review may reach a
second provider and nothing on the machine runs a gate. It replaces the
cross-provider R2 and the forge-app R3 of the full edition with reviews by the
same model in contexts kept apart from each other, and says how the pull
request records them so a reader can never mistake them for what they are not.

## Why the layers change

The full edition's R2 sends the diff to a model from a different provider, and
its R3 is an app the forge runs. Both send code outside the organisation, which
is the thing this edition exists to avoid. What survives without them is
separation of context: a reviewer that did not write the change and did not see
it being written still reads it with fresh eyes, even when it is the same model.

That is weaker than cross-provider review. Two contexts of one model share its
blind spots, and nothing here pretends otherwise. The edition accepts the loss
and records it on every pull request.

## What an isolated context is

A context is isolated when it starts empty: a subagent or a new session that
did not inherit the conversation in which the change was authored.

- It receives only the inputs its layer names below.
- It never receives the authoring conversation, the Author's account of why the
  change is right, or another review's findings.
- It reports findings and never edits the change.

A review run inside the authoring conversation is R1, whatever it is called.
If the agent in use cannot open an isolated context, R2 and R3 did not run, and
the pull request says so.

## The three layers

| Layer | When | Who | Reads |
|---|---|---|---|
| R1 | Before the change is handed over | The Author, in the authoring session | The whole change, against the approved spec |
| R2 | Before the branch is pushed | The same model, isolated context | The diff against the base branch, the approved spec, `docs/standards/`, and the Reviewer section of the instruction file |
| R3 | After the pull request is opened | The same model, a second isolated context | The pull request's title and body, its full diff, the approved spec, and the PR Review Checklist in `docs/standards/github.md` |

R2 and R3 run in separate contexts: R3 does not see R2's findings, so it can
find what R2 missed rather than confirm what R2 said.

## Findings

A finding falls into one of the categories the Reviewer section of the
instruction file lists. Every finding is addressed or justified by the Author in
the pull request, never dropped in silence.

## Recording the layers in the pull request

Each layer gets one line in the PR Review Checklist, in this form:

```text
R1: performed by <model>, in the authoring session.
R2: same-provider, isolated context, <model>. <N> finding(s), each addressed or justified.
R3: same-provider, isolated context, <model>. <N> finding(s), each addressed or justified.
```

A layer that did not run says `not run` and why. These lines never use
`verified`, `declared` or "cross-provider": those words belong to a review by a
second provider, and writing them here would report a review that did not
happen.

## What nothing checks for you

In the full edition a program refuses a commit, a push or a pull request that
breaks these rules. Here nothing runs, so the Author checks each one before
handing the change over and the Developer confirms it in the pull request:

- **Spec**: a non-trivial change has a spec under `docs/specs/` that passed the
  Spec Gate, per `docs/standards/spec_method.md`.
- **Records**: spec and ADR numbers run from `0001` with no gap and no reuse,
  no record was deleted, and every spec opens with its `# SPEC:` title line.
- **Commits**: every subject is a Conventional Commit whose type is in the Type
  Table of `docs/standards/github.md`, with no co-author or AI-attribution line.
- **Branch**: the branch name follows Branch Naming in `docs/standards/github.md`.
- **Index**: every standard in `docs/standards/` is listed in `INDEX.md`.
