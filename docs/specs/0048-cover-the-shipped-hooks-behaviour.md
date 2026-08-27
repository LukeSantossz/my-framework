# SPEC: test(harness): cover the shipped hooks' own behaviour

## Problem

`internal/activate` proves the hooks are written, made executable, wired and
never overwritten, and nothing proves what they do — so the fail-closed
behaviour that `docs/specs/0020` and `0027` were written to establish is
asserted by a comment at the top of `pre-push` and by nothing else.

## Design Decision

A Go test executes each hook as a process under `bash`, against a temporary
repository, with a stub `mf` whose exit status, output and argument log the test
controls. One test language, no new dependency, and it runs in the `gate.yml`
matrix on Linux, macOS and Windows because that matrix already runs
`go test ./...`. The hooks under test are read from the embedded `framework.Hooks`
rather than from the working tree, so what is exercised is the bytes an adopter
receives. Runner discovery is asserted by identity, not by side effect: each
stub prints which of the four discovery paths it was reached through, so
"the gate ran" and "the gate could not run" cannot be confused.

## Alternatives Considered

- **A shell test framework (bats, shunit2).** A second test language and a
  second dependency to install on three runners, for scripts that are already
  reachable from `os/exec`. The suite's own history — a `scripts/test/` runner
  deleted with the shell layer it tested — is the argument against re-creating
  one.
- **Refactor the hook logic into Go and leave the scripts as thin shims.** git
  runs a script, so a shim is still a script and its failure paths are still
  untested; and the discovery logic exists precisely to find the Go binary, so
  it cannot live inside the binary it is looking for.
- **Assert on the hook text rather than its behaviour** — grep for the absence
  of `|| exit 0`. It passes on a script that never runs at all, and the
  regression it is meant to catch is behavioural: a failure path that exits 0 by
  some other spelling reads as clean.
- **Drive the hooks through `git commit` and `git push` rather than executing
  them directly.** A push needs a remote and a commit needs an author, both of
  which add failure modes the test does not care about, and neither lets the
  test observe the hook's exit status separately from git's.

## Scope

- Includes: a `hooks_test.go` in `internal/activate` executing `.githooks/commit-msg`
  and `.githooks/pre-push` from the embedded filesystem; coverage of runner
  discovery (`MF_BIN` executable, `MF_BIN` not executable, `mf` on `PATH`,
  `$repo_root/mf`, `$repo_root/mf.exe`, nothing found), of fail-closed on a
  missing runner and an undiscoverable repository, of exit-status propagation
  for both hooks, of `SKIP_R2_REVIEW=1`, and of the arguments each hook passes
  to the runner.
- Does NOT include: any change to the hook scripts themselves, to
  `internal/activate`'s production code, or to `gate.yml`; testing `mf check` or
  `mf review` (the stub stands in for both); driving git's own `commit` and
  `push` plumbing.

## Acceptance Criteria

- `hook_uses_mf_bin_when_it_is_executable` — both hooks run the binary `MF_BIN`
  names, in preference to one on `PATH`.
- `hook_refuses_a_non_executable_mf_bin` — both hooks exit non-zero, name
  `MF_BIN` on stderr, and run no runner.
- `hook_falls_back_through_path_then_repo_root` — with no `MF_BIN`, `mf` on
  `PATH` is used; with neither, `$repo_root/mf` is; with neither of those,
  `$repo_root/mf.exe` is.
- `hook_fails_closed_when_no_runner_is_found` — both hooks exit non-zero and say
  the gate did not run.
- `hook_fails_closed_outside_a_repository` — with `git rev-parse
  --show-toplevel` answering nothing, both hooks exit non-zero.
- `commit_msg_passes_the_message_file_to_the_runner` — the runner is called as
  `check commit --message <path>` with the path git handed the hook.
- `commit_msg_stops_the_commit_when_the_check_fails` — a runner exiting non-zero
  makes the hook exit non-zero.
- `pre_push_stops_the_push_when_the_checks_fail` — and the review is never
  reached.
- `pre_push_does_not_stop_the_push_when_the_review_only_reports_findings` — a
  review exiting 0 leaves the hook exiting 0.
- `pre_push_propagates_a_blocking_review` — a review exiting non-zero makes the
  hook exit with that status.
- `skip_r2_review_skips_only_the_review` — with `SKIP_R2_REVIEW=1` the check
  still runs, the review does not, and the hook exits 0.

## Reproducibility

```sh
go test ./internal/activate/ -run Hook -v
```

Go 1.26.7, `bash` on `PATH` (Git Bash on Windows, the system shell elsewhere).
The tests skip rather than fail where `bash` is absent, which is the only
platform fact they depend on.

## Risks and Assumptions

- Assumption: `bash` is present on all three `gate.yml` runners, which is
  already relied on — every `run:` step in that workflow sets `shell: bash`.
- Assumption: a file with a `#!` line is what each platform's `[ -x ]` accepts
  as executable; the non-executable case therefore uses a file with neither a
  shebang nor an executable extension, which fails the test on every platform.
- Assumption: stubbing `git` for the no-repository case is more reliable than
  finding a directory outside every repository on the machine.
- What would invalidate this spec: moving the gates out of shell hooks, which
  would leave nothing here to execute.
