- R1 runs inside this session. Perform it before handing the change over; then
  have R2 run in an isolated context before the branch is pushed, and R3 in a
  second isolated context after the pull request is opened, per
  `docs/standards/isolated_review.md`. Record each in the pull request as that
  document says, never as a cross-provider review.

- Nothing in this edition checks the change for you. Before handing it over,
  check by hand what `docs/standards/isolated_review.md` lists under What
  nothing checks for you.
