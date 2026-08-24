# Configuration in TOML, split by policy and machine state

`docs/specs/0013-detach-r2-from-codex.md` configured the R2 gate through git's own scope
cascade and rejected a dedicated configuration file on three grounds: it duplicates a
cascade git implements correctly, it adds a format to parse in shell, and it creates a
second place to look when a value resolves surprisingly. We supersede that decision.
Configuration moves to TOML in two layers, split by the nature of the data rather than by
scope: policy — roles, backend chains, constraints, thresholds — in a versioned
`.framework.toml`, and machine state — endpoints, the *name* of the variable holding a
key, local preferences — in `~/.config/framework/config.toml`. A project file that
declares an endpoint or a credential reference is a validation error, which makes
"secrets never enter the repository" a rule the loader enforces rather than a convention
the author observes.

The reason to change is not the one that expired. "Adds a format to parse in shell" was
substrate-dependent and does not survive the move to Go, but it was never the
load-bearing reason and its expiry alone would not justify reopening a gate-approved
decision. The load-bearing reason was not available in `0013`: `.git/config` is never
committed, so project policy cannot travel with the repository, and a team adopting this
framework has nowhere versioned to state its own review policy. Note also that what
`0013` rejected was a machine-level file under `~/.my-framework/`; it never ruled on a
versioned project file, because the requirement had not yet appeared.

## Status

Accepted. Supersedes the configuration decision recorded in
`docs/specs/0013-detach-r2-from-codex.md`.

## Considered Options

- **TOML in both layers, split by the nature of the data (chosen)**: one format, tool
  state in the tool's own directory, and the one layer git structurally cannot provide.
  The legacy `r2.*` keys are read as a deprecated fourth layer until `mf config migrate`
  moves them.
- **Add only the versioned project layer and keep `git config` for machine state**:
  rejected — it supersedes nothing and costs existing adopters no reconfiguration, but it
  leaves two configuration technologies in permanent coexistence and keeps a tool's
  machine state hanging off git's configuration, a place it landed only because shell
  could not parse a format.
- **Keep `git config` alone**: rejected — it cannot express committed policy at all,
  which is the requirement that forced this decision.
- **Materialize a committed file into `git config` at `mf init`**: rejected — it keeps a
  single runtime store, but creates two representations of the same data with nothing
  keeping them in sync, which is the drift failure this framework designs against
  elsewhere.

## Consequences

- `0013`'s third objection survives intact: a value can now resolve from four places. It
  is accepted only against per-layer provenance reporting — `mf config get` names the
  layer a value resolved from, and `mf doctor` prints the whole resolved table with its
  provenance. Without both, this change is a net regression against `0013`, which is why
  both are acceptance criteria of the spec rather than conveniences.
- Existing adopters keep resolving as they do today: the deprecated read layer answers
  from `r2.*`, and `mf config migrate` moves those keys on demand rather than on upgrade.
- The credential rule is unchanged. The configuration holds the *name* of the environment
  variable carrying a key, never the key, because `git config --list` output and now a
  TOML file alike end up in bug reports and screenshots.
- Policy resolves as environment, then project, then machine, then the deprecated git
  keys, then the built-in default. Credentials and endpoints have no project layer at
  all, so precedence does not apply to them — a project that names one is refused, not
  overridden.
