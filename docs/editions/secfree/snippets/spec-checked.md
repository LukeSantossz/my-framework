### The rule is checked by hand

Nothing in this edition runs, so the Author checks the rule before handing a
change over and the Developer confirms it in the pull request, in three ways,
because no one of them sees what the others do:

- **Contiguity.** The numbers in each archive run from `0001` with no gap and no
  duplicate. A gap at a number another open branch holds is a claim rather than
  a hole — numbers are taken when a record is written — and is named in the pull
  request so the reader can tell the two apart.
- **History.** Every spec and ADR ever committed is still present. This is the
  load-bearing one: contiguity alone cannot see the deletion of the
  highest-numbered record, which leaves the rest contiguous.
- **Headers.** Each file in the specs archive opens with `# SPEC:`. A directory
  of files is not an archive unless each file says what it is.
