# SPEC: fix(release): publish without losing the whole release to one upload

## Problem

`Publish` is one command: `gh release create "$TAG" dist/* --generate-notes`.
`gh` creates the release and then uploads the six assets, and if any upload
fails it deletes the release it just made. So a single flaky upload discards a
build that passed every gate, leaves no release for the tag, and cannot be
fixed by re-running without a person noticing and asking for it.

It happened on `v0.7.1`: the first asset stalled for five minutes and came back
`HTTP 400`, the release vanished, and the tag sat published with nothing behind
it until the run was re-run by hand. A repository that pins a tag would have
found a version that does not exist.

## Design Decision

Separate creating the release from filling it, and make each step safe to
repeat. The release is created only if it is not already there, with
`--verify-tag` so a tag that does not resolve fails loudly rather than creating
a release for nothing. Each asset is then uploaded on its own with `--clobber`,
retried three times with a widening pause.

Idempotence is the point, not the retries. A step that can be re-run leaves a
half-published release recoverable by pressing the button, which is what the
`v0.7.1` incident actually needed. The retries only keep the common case from
reaching the button at all.

## Alternatives Considered

- **Retry the whole `gh release create`.** Rejected: it is not idempotent. The
  second attempt finds the release it deleted may or may not be gone, and a
  partial upload leaves assets that make the next `create` fail on a name that
  already exists — which is the `400` this incident produced.
- **Publish as a draft and flip it at the end.** Rejected: it narrows the window
  rather than closing it, and a failure between the last upload and the flip
  leaves an invisible release nobody looks for.
- **Upload with a third-party release action.** Rejected: every action in this
  workflow is pinned to a SHA precisely because this workflow decides what gets
  published, and adding one more supply-chain edge to avoid writing six lines of
  shell is a poor trade.

## Scope

- Includes: the `Publish` step in `.github/workflows/release.yml`; a guard in
  `cmd/mf/release_publish_test.go` asserting the properties that make it
  recoverable.
- Does NOT include: what is built, how it is stamped, the checksum file, or the
  gate the release calls; retrying anything else in the workflow; republishing
  any past release.

## Acceptance Criteria

Each is a test in `cmd/mf/release_publish_test.go`, which reads the workflow.
That is a weaker test than any other here and it is deliberate: the behaviour
runs in GitHub Actions, so what can be asserted locally is that the properties
which make the step recoverable are still in it. A later edit dropping one is
then caught by a test rather than by a tag published with nothing behind it.

- `publish_creates_the_release_only_when_it_does_not_already_exist`
- `publish_verifies_the_tag_before_creating_anything`
- `publish_uploads_each_asset_so_a_rerun_replaces_rather_than_conflicts`
- `publish_retries_a_failed_upload_and_then_fails`

## Reproducibility

The failure this fixes is a network one and does not reproduce on demand. What
reproduces is the recovery: re-running the `build` job of a release run that
half-published now completes, where before it failed on `already exists`.

```sh
gh run rerun <id> --failed
gh release view v0.7.1 --json assets -q '.assets|length'   # 6
```

Versions: `gh` as shipped on `ubuntu-latest`, Go 1.26.7.

## Risks and Assumptions

- Assumption: `gh release upload --clobber` replaces an asset of the same name.
  It is documented to, and it is what makes a re-run finish rather than collide.
- Risk: three attempts with a widening pause can hide a real outage as a slow
  job. The step still fails, and the warnings name each attempt, so the run
  reports what happened rather than silently succeeding.
- Risk: a release created by an earlier attempt keeps the notes it generated
  then. They are generated from the tag, which has not moved.
