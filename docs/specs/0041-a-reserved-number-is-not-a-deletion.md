# SPEC: fix(check): stop reading a number another branch holds as a deleted record

## Problem

`mf check records` fails a gap in either archive with "a record was deleted
rather than retired in place". On a repository where two changes are open at
once, that is wrong and unavoidable: durable numbers are claimed when a spec is
written, so a branch that claims `0038` while another still holds `0037` has a
gap in its own tree, has deleted nothing, and cannot push — both hooks fail
closed.

It is not hypothetical in either direction. The repository being migrated has an
open pull request holding `0037`; the migration must claim `0038`, and the gate
refuses it. The same repository's own guard, written before the harness reached
it, already carved out the exception in a comment naming two specs a concurrent
pull request had reserved. So a rule the framework enforces absolutely is one an
adopter had already found to be conditional.

## Design Decision

A gap fails only when nothing anywhere in the repository holds the missing
number. Reading the other refs is what separates the two cases the message
cannot: a number sitting in a branch's tree is claimed, and one that exists on
no ref is a hole.

Deletion stays caught, and by the check that was always load-bearing for it:
`deletedRecords` fails a record this branch's history added and no longer has,
whatever the numbers around it look like. Contiguity was never the thing that
saw a deletion — `spec_method.md` says so in the sentence that introduces it —
so narrowing it takes nothing away from that.

What it does give up is the typo: an author who writes `0039` when the archive
ends at `0037` gets a gap that no ref fills, which still fails, unless some
branch happens to hold `0038`. That is the same evidence a reservation leaves,
and no amount of reading the tree separates them.

Refs are read through `git for-each-ref` and `git ls-tree`, both local: a gate
that reached the network would fail differently on a machine that is offline,
and the branches are already fetched or they are not this repository's problem.

## Alternatives Considered

- **Configure the reserved numbers.** Rejected: a list that has to be edited
  when a branch opens and again when it merges is a list that is wrong most of
  the time, and being wrong here means either a gate that passes over a real
  deletion or one that refuses a legitimate push.
- **Check contiguity only on the base branch.** Rejected: it turns the gate off
  for every feature branch, including for the duplicate a branch introduces,
  which is the part of this rule with no second check behind it.
- **Search history rather than the current refs.** Rejected: a number written on
  a branch that was abandoned and deleted would excuse a gap forever. What a ref
  holds now is a claim someone still has open.
- **Let the adopter's own guard carry the exception.** Rejected: it is the
  answer that produced this situation — a repository quietly holding a different
  rule from the framework it vendors, with nothing reconciling them.

## Scope

- Includes: `numbering` taking the set of numbers other refs hold, and reporting
  a gap only for a number none of them does; `Repo.RecordFilesOnOtherRefs`,
  which reads what sits directly under the archive on every ref but the one at
  `HEAD`; the caller applying the same filename rule `numbering` applies, so a
  draft in a subdirectory cannot claim a number; the Contiguity paragraph in
  `spec_method.md`, which states the narrowed rule so the standard and the gate
  say the same thing.
- Does NOT include: the duplicate check, the starts-at-0001 check, or
  `deletedRecords`, all unchanged; the durable-numbering rule itself — a number
  is still never reused and a superseded record is still retired in place, and
  what narrows is only when a gap counts as evidence that one was not; reading
  refs over the network.

## Acceptance Criteria

- `a_gap_at_a_number_no_ref_holds_still_fails`
- `a_gap_at_a_number_another_branch_holds_passes`
- `a_duplicate_number_still_fails_however_many_refs_hold_it`
- `numbering_outside_a_git_repository_behaves_as_it_did`
- `record_files_on_other_refs_ignores_the_branch_at_head`
- `record_files_on_other_refs_ignores_the_working_tree`
- `a_gap_a_draft_or_a_non_record_file_would_excuse_still_fails`
- `numbering_asks_about_other_refs_only_when_there_is_a_gap`

## Reproducibility

```sh
git init -b main probe && cd probe
mkdir -p docs/specs && printf '# SPEC: a\n' > docs/specs/0001-a.md
git add -A && git commit -m "docs(spec): a"
git checkout -b other && printf '# SPEC: b\n' > docs/specs/0002-b.md
git add -A && git commit -m "docs(spec): b"
git checkout main && git checkout -b mine
printf '# SPEC: c\n' > docs/specs/0003-c.md
git add -A && git commit -m "docs(spec): c"
mf check records
```

Before this change: `gap between 0001 and 0003`. After: passes, because `other`
holds `0002`.

Versions: Go 1.26.7, `mf` at the commit under review.

## Risks and Assumptions

- Risk: a stale local branch holding a number nobody is working on excuses a gap
  that is a typo. Deleting a merged branch is ordinary hygiene, and the failure
  it causes is a message not printed rather than a bad record accepted.
- Assumption: `git for-each-ref`, `git ls-tree` and `git symbolic-ref` are
  available. All three are core git, and this gate already requires git for
  `deletedRecords`. A detached HEAD resolves to no branch, which counts one more
  ref than it should and is the safe direction: the tree it holds is the one
  checked out.
- Assumption: enumerating refs is cheap enough to run on every push. One
  `for-each-ref` and one `ls-tree` per ref, over one directory, and only when a
  gap was found at all.
