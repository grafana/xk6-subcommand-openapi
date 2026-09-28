# Contributing

Participation in this project is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

This project follows spec-driven development, using [OpenSpec](https://github.com/Fission-AI/OpenSpec). No implementation code is added until a spec exists and is approved for that piece of work. That's not a formality: it means the team agrees on what's being built and how before anyone writes code. Design gets reviewed first, while it's still cheap and easy to reason about, rather than only surfacing problems once a full implementation already exists.

Security fixes, and bug fixes that just correct an implementation defect against an already-approved spec (no new design decision involved), are exceptions: they can skip the proposal step and land as a single PR.

## Starting a change

Specs and changes live under `openspec/`. Start a change with:

- `/opsx:propose` in Claude Code
- `$openspec-propose` in Codex
- `/opsx-propose` in GitHub Copilot

This generates a proposal, a design doc, a task list, and a delta spec under `openspec/changes/<name>/`. Project context shown to these tools while drafting a proposal lives in `openspec/config.yaml`.

## Two PRs per change

Each change is delivered as two separate PRs, not one:

1. A proposal PR: the spec only, no code. Once this is reviewed and merged, the design is agreed on.
2. An impl PR: the actual code, following the approved proposal's tasks. Once all tasks are done, this PR also runs the archive step (moving the change into `openspec/changes/archive/` and syncing `openspec/specs/`), as its last action before merging. Archive is not a third PR.

The proposal PR should reference its issue without a closing keyword (for example "Part of #123"), so merging it doesn't close the issue early. Only the impl PR, the one that actually finishes the change, should use a closing keyword ("Closes #123").

## Status tracking

Because of the two-PR split, a change moves through several stages before it's actually done:

- `planned`: not started yet.
- `proposal: in progress`: the proposal is being written.
- `proposal: PR open`: the proposal PR is open for review.
- `proposal: PR merged`: the proposal PR has merged. Impl hasn't started yet.
- `impl: in progress`: code is being written.
- `impl: PR open`: the impl PR is open for review.
- `impl: PR merged`: the impl PR has merged and the change is archived. This is the only state that counts as done.

These same names work as GitHub labels. Once a specific change's work starts, its line in the roadmap issue can be converted into its own sub-issue (a native GitHub feature), labeled with whichever state applies, instead of relying on hand-edited status text.

Changes are often stacked: a later one's branch can start before an earlier one has merged, to avoid waiting. So there's no fixed moment to update a change's status. Whoever notices a change move to a new stage updates it then, no schedule needed.

Planned and in-progress work is tracked in a pinned issue in this repo (see the [Issues tab](https://github.com/grafana-experimental/xk6-subcommand-openapi/issues)).

## Before opening a proposal PR

Run `openspec validate <change-name> --strict` and make sure it passes.

## Before opening an impl PR

Run `make lint`. It uses the same golangci-lint configuration as k6 itself.
