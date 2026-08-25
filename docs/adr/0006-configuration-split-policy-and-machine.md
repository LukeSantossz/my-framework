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

Accepted, and amended below — see "Amendment: how a named object resolves across the two
layers". Supersedes the configuration decision recorded in
`docs/specs/0013-detach-r2-from-codex.md`, which is marked Retired in place.

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

## Amendment: how a named object resolves across the two layers

_Recorded by `docs/specs/0027-close-the-audit-pendings.md`._ The machine layer holds
backends, which is what this decision said from the start and the loader did not
implement. Until it did, naming a reviewer at all meant committing it, so a machine with
a locally reachable model and a CI runner with a secret had no way to supply one — which
is why R3 spent a runner on every pull request to report that it did not run.

Wiring it forced a question the decision above left open, because `roles.*` and
`backends.*` are named objects rather than scalars and "the higher layer wins" does not
say what a layer wins. Three rules answer it, and they are part of the decision rather
than of its implementation:

- **A project definition of a backend name shadows a machine definition of that name
  whole, not field by field.** Policy outranks machine state everywhere else here, and a
  machine that could redefine one field of a name a committed chain already uses could
  substitute the reviewer that repository chose — point a `provider = "openai"` label at
  another vendor's endpoint, say, which nothing downstream can detect. Merging would also
  produce a backend that is half of each: a definition nobody wrote and nobody can
  predict. A machine backend therefore completes a chain by *adding* a name, never by
  replacing one.
- **A machine may supply a role's chain only for a role the project leaves undeclared.**
  The same rule applied to `[roles.*]`: a machine may not review a repository with a
  chain that repository did not choose. The environment layer is what remains for one
  run and one person — `MF_ROLES_R2_BACKENDS=... mf review`.
- **The refusal of a `command` in a project backend does not extend to a machine one.**
  That rule's subject is code arriving with a repository and running on whoever clones
  it. A machine file is its owner's own, and refusing them a reviewer they wrote
  themselves protects nobody from anything.
