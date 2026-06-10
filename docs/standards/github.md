# GitHub Standards

Conventional Commits, branch naming, and the Pull Request, Issue, and README templates.

## Conventional Commits

Structure: `type(?scope): subject`

- type: kind of change (see table).
- scope: context of the change (optional, in parentheses).
- subject: descriptive message (required).

Subject rule: imperative mood, completing "If applied, this commit will...". Never include co-author messages, regardless of AI model used.

### Type Table

Single canonical vocabulary for the whole project. Commits, PR titles, issue titles, and branch names draw from it. No reduced or parallel list exists elsewhere.

- feat: new feature for the user.
- fix: bug fix.
- docs: documentation-only changes.
- style: formatting, whitespace, semicolons (no logic change).
- refactor: code refactoring (no bug fixes or new features).
- perf: performance improvement.
- test: creating or adjusting tests.
- chore: build, tooling, or configuration changes (e.g. eslint).
- build: external dependencies (npm) or build system.
- ci: CI configuration files.
- revert: reverting a previous commit.

Examples: `feat(auth): add Google integration`, `fix(api): handle 500 error on users endpoint`, `docs: update installation guide in README`.

### Branch Naming

`type/TASK-NNN-short-description` (e.g. `feat/AUTH-12-google-login`, `fix/api-500-on-users`)

- type: one of the canonical types above.
- TASK-NNN: tracker ID when one exists; omit otherwise.
- description: lowercase, hyphen-separated, English.

## Pull Request Model

Title: Conventional Commits format, type from the canonical Type Table. Example: `feat(auth): implement password recovery`.

### 1. Context

- Motivation: reason for the change.
- Task Link: tracker URL if any.
- Spec Link: link to the approved `SPEC.md` if one exists for this change.

### 2. What Was Done

List the technical changes in summary form.

### 3. How to Test

Step-by-step for the reviewer:

1. Check out the branch.
2. Install dependencies.
3. Run the project.
4. Verify the build runs without errors.

### 4. Evidence

Screenshots or videos if the change is visual; omit if backend or configuration only.

### 5. Self-Review Checklist

- Self-review done in the Files Changed tab.
- Spec approved at the Gate before implementation, and the change matches its Scope (per `spec_method.md`).
- Each Acceptance Criterion has a passing test; tests were written before their implementation.
- Commented-out code and unnecessary debug statements removed.
- Code follows the project style guide.
- New dependencies work without breaking the build.
- Review layers recorded: internal Superpowers review (R1), cross-provider review (R2), automated PR review (R3) where applicable, with Author and Reviewer models named (per `ai_guidelines.md` Review Composition). Note any layer that did not run and why.

## Issue Model

Title: Conventional Commits format. Example: `refactor(database): migrate local storage from Hive to an actively supported alternative`. Valid types: the canonical set above. Use fix for defects, not bug.

### Description

What needs to be done and why.

### Context

History or technical motivation justifying the issue.

### Current Usage

Current state of what will change: entities, operations, affected dependencies.

### Recommended Alternative

Proposed solution and the criteria justifying it (name plus reasons).

### Acceptance Criteria

Concrete deliverables that define the issue as complete.

## README Model

Canonical section order, do not reorder: What It Does, What It Is, Tech Stack, Architecture, Engineering Decisions, Results, Getting Started, API Reference, Project Structure, Project Status, Known Issues, Contributing, License.

Remove all HTML comments and `{...}` placeholders before publishing. Sections marked OPTIONAL are included only if they add real signal; remove if empty.

### Badges

Badges communicate health only. Never advertise low coverage, yellow status, or any weakness metric. If CI is not green, omit the CI badge. Order: language(s), main framework, CI, license.

### Title

`{Project Name} — {One-Line Subtitle}` plus a one-sentence tagline. Include the highest-impact number if any.

### What It Does

One purpose sentence, then working features in a short list. A feature is what the user or system can do, not an internal component.

### What It Is

Classify the artifact unambiguously: web app, desktop app, mobile app, REST API, CLI tool, library, data pipeline, or research codebase, plus what it produces and the problem it solves.

### Tech Stack

Layer to technology, listing only structurally relevant items: Language, Framework/Runtime, State/Data layer, ML/Inference, Testing/CI.

### Architecture (OPTIONAL)

Include a Mermaid diagram only if the flow is non-trivial. Remove for simple scripts. Add one to three sentences on non-obvious topology.

### Engineering Decisions

Minimum 3 rows, each with: decision made, alternative considered, why this approach. Proves trade-off reasoning.

### Results (OPTIONAL)

Include only with defensible numbers (benchmarks, model metrics, measured speedups). Comparison table with baseline; best row in bold. Each number carries the command and versions needed to reproduce it. Remove if no real data.

### Getting Started

Prerequisites, Installation, Running, Tests, each with concrete commands.

### API Reference (OPTIONAL)

For projects exposing an interface (REST, CLI). Endpoint or flag table plus one minimal runnable example. Becomes Usage for libraries.

### Project Structure

Lean tree, commenting only directories carrying architectural intent.

### Project Status

Status line: complete, MVP complete, or in development with version. For in-progress, split Done and Pending checklists.

### Known Issues & Limitations (Mandatory)

Real limitations with technical honesty, not apologies. Each item: what it is plus why it exists or when it goes away.

### Contributing (OPTIONAL)

Reference CONTRIBUTING.md. Summary: fork, branch (`type/TASK-NNN-description`), tests, Conventional Commits, PR.

### License

State the license (e.g. MIT).
