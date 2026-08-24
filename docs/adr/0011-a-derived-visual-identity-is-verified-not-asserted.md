# A derived visual identity is verified by fingerprint, not asserted in prose

The visual identity of the one page this framework generates is derived from a
third-party design document — the Warp entry in `voltagent/awesome-design-md` — rather
than authored from nothing or copied wholesale. That middle position is the one the
Developer chose, and it is the one with no natural evidence: "inspired, not copied" is a
claim about intent, and intent is exactly what a reader cannot check.

So the claim is not made in prose. `docs/standards/design.md` records a one-way
fingerprint of every identity-carrying value in the source entry — its colours and its
typefaces — and `mf check design` refuses any declared token that matches one. Reusing
the source's literal palette becomes impossible rather than discouraged, and the
repository carries no readable copy of another project's palette as content while still
being able to prove non-identity.

The check is deliberately narrow about what it proves. Direction is not a value: a
layout, a restraint, a decision to carry no accent colour cannot be fingerprinted, and
this gate would pass a page that reproduced all of them. It makes one specific failure
impossible and claims nothing beyond that. The fingerprints also conceal nothing — a
six-digit hex is trivially brute-forced — so they are a comparison mechanism, not a
privacy one.

## Status

Accepted.

## Considered Options

- **Fingerprint the source's identity-carrying values and gate on them (chosen)**: it
  converts the one claim in this decision that cannot otherwise be checked into a
  property CI enforces, and it does so without republishing the source's palette.
- **State the derivation in prose and trust it**: rejected. Every other standard here is
  enforced by something; a standard whose central claim is unverifiable is the failure
  this framework exists to fix, and this claim in particular is the one with legal weight.
- **Copy the source entry as-is**: rejected at the Developer's decision. The collection's
  MIT licence covers the documents and explicitly disclaims ownership of any site's
  visual identity, so the licence is not what would make shipping another company's
  trade dress acceptable.
- **Author values with no source**: rejected at the Developer's decision. It has no
  question to answer, but it also has no direction — the values would be chosen the way
  the explainer's original ones were, which is the drift the standard exists to record.
- **Fingerprint every value in the source, spacing and radii included**: rejected. A 4px
  radius and an 8px step are nobody's identity. A gate that flagged them would fail on
  arithmetic rather than on appropriation, and a gate nobody can predict is one people
  route around.
- **Store the source's values in the repository so the comparison is readable**:
  rejected. It would make the check easier to audit and would put a readable copy of
  another project's palette in this repository as content, which is the thing being
  avoided.

## Consequences

- `docs/standards/design.md` is binding, and `mf check` reports a seventh gate.
- The palette declares no chromatic accent, and that is enforced by measuring each
  non-semantic colour's distance from grey. It is the rule that carries this project's
  own premise: a tool whose provider is configuration must not wear a vendor's colour.
- Semantic colour is exempt from that measure but never a state's only signal, which is
  prose in the standard rather than something the gate can measure.
- The source entry carries no version. The recorded read date is the only thing that
  makes a later divergence legible, and the fingerprints then describe a document that
  may no longer exist.
- The gate reads colour literals with a regular expression, so the standard forbids the
  forms it cannot read — named colours, `oklch()`, computed values — rather than the gate
  pretending to catch them.
