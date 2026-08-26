# SPEC: fix(init): record the hooks' executable bit where a clone can read it

## Problem

`mf init` writes the hooks 0755 on disk and stops there, so on a checkout with
`core.fileMode` false — the Windows default — git stages them 0644 and every
clone receives both gates non-executable, which git silently skips: the adopted
repository reports a wired gate and has none, on every platform except the one
that adopted it.

## Design Decision

Record the mode where a clone actually reads it. `materialiseHooks` already
chmods each hook it wrote; it now also stages that hook with
`git update-index --add --chmod=+x`, so the index carries what the filesystem
was told. Failing to reach the index is not fatal — `mf init` outside a work
tree still leaves a usable checkout — because the second half of the answer is a
report: `mf doctor` reads the index and names any hook it records as
non-executable, so a repository already adopted the old way is told rather than
left to discover it when a gate does not fire.

Reading the index rather than the filesystem is the whole point. The filesystem
is what looks correct on the machine that wrote it and what nobody else receives.

## Alternatives Considered

- **Set `core.fileMode true` in the repository.** Rejected: it is a machine
  setting about a filesystem, and forcing it on a Windows checkout makes git
  report spurious mode changes across every tracked file, not just these two.
- **Report it and let the adopter run the command.** Rejected as the only
  measure: the trap stays armed for every future adoption, and the failure it
  produces is silent, which is the class of defect this framework exists to
  close. The report is kept as the half that helps repositories already adopted.
- **Ship the hooks as something git always executes** — a wrapper, or a
  `.gitattributes` rule. Rejected: git has no attribute for the executable bit,
  and the wrapper needs the bit too.

## Scope

- Includes: `materialiseHooks` staging each hook it wrote with the executable
  bit; `Repo.IndexIsExecutable` and `Repo.MarkIndexExecutable`;
  `activate.NonExecutableHooks`; the `mf doctor` line that names them.
- Does NOT include: staging anything else `mf init` writes — the policy file,
  the standards and the instruction source are the adopter's to stage; changing
  `core.fileMode`; repairing repositories already adopted, which is a change to
  those repositories and is specified there.

## Acceptance Criteria

- `init_stages_every_hook_it_wrote_as_executable_when_core_filemode_is_false`
- `non_executable_hooks_names_a_hook_the_index_records_without_the_bit`
- `non_executable_hooks_reports_nothing_right_after_init`
- `doctor_names_a_wired_hook_the_index_records_as_non_executable`

## Reproducibility

```sh
git init -b main probe && cd probe && git config core.fileMode false
mf init
git ls-files --stage .githooks/
```

Before this change: `100644` for both hooks. After: `100755`.

Versions: Go 1.26.7, `mf` at the commit under review.

## Risks and Assumptions

- Assumption: staging the two hooks is acceptable as a side effect of `mf init`.
  They are files init wrote and the adopter must commit for the gate to reach
  anyone, and the alternative is a gate that does not run.
- Risk: an adopter who then runs `git commit` for something else sweeps the
  hooks in. They are the files init was asked to install, and `mf init`'s output
  names them.
- Assumption: `git update-index` is available wherever `mf init` runs. It is
  part of git, and init has already required git to read `core.hooksPath`.
