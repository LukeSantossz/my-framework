# SPEC: fix(test): stop two flakes that failed a release the tree could build

## Problem

The `v0.5.0` release job failed on a tree that builds, vets and tests clean,
because two tests depend on timing rather than on what they assert: every
scratch repository lets git fork `gc --auto`, which goes on writing into
`.git/objects` after the test body returns and makes `t.TempDir()` fail its own
cleanup, and the usage store's concurrency test runs two hundred lock
acquisitions in series, each waiting out production's ten-second budget, which
puts the test close enough to that ceiling to fail on a loaded machine.

## Scope

- Includes: turning off `gc.auto` and `maintenance.auto` in every test helper
  that builds a scratch git repository; reducing the in-process concurrency
  test's writer count and iterations to a size that still demonstrates the
  property; and re-cutting the `v0.5.0` tag once the gate is green.
- Does NOT include: any change to `Store.lock` or its constants — the ten-second
  budget is production behaviour, chosen so a jammed store degrades to a
  reported accounting failure rather than a hung review; the cross-process
  concurrency test beside it, which is the one that proves the real shape;
  loosening any gate so the release passes.

## Acceptance Criteria

- `a_scratch_repository_leaves_no_writer_behind_for_temp_dir_cleanup`
- `the_concurrency_test_still_fails_when_the_lock_is_removed`
- `the_full_suite_passes_twice_in_a_row_on_a_loaded_machine`
- `the_release_job_completes_and_publishes_the_tag`
