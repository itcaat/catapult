# Working on Catapult

## Single source of truth

All product work is managed through GitHub Issues in `itcaat/catapult`.
GitHub Issues is the only task manager and the source of truth for task status.

Do not maintain parallel task lists in README files, notes, chat messages, local files,
or external task managers. Documentation may describe rules and architecture, but every
piece of product work must reference a GitHub Issue.

## Starting work

No substantial development work starts without an Issue. When a problem is found during
research, create an Issue first with its priority, scope, and acceptance criteria.

Before starting work:

1. Find an existing Issue to avoid duplicates.
2. Make sure the Issue describes the desired outcome, not only an implementation idea.
3. Set the priority and relevant labels.
4. Link the Issue from the branch, commits, and pull request.

When the user says “take issue #123” or “pick up #123”, this means:

- review the current description, comments, and related Issues;
- apply `status:in-progress`;
- assign the current owner when appropriate;
- implement only the agreed scope;
- update the Issue with progress, decisions, and discovered constraints;
- after verification, apply `status:ready-for-review` or close it when no separate review
  is required.

If an Issue is underspecified or conflicts with another requirement, add a question or
comment to the Issue before expanding its scope.

## Status labels

Use these labels:

- `status:triage` — discovered, but not yet analyzed;
- `status:ready` — specified and ready to start;
- `status:in-progress` — actively being worked on;
- `status:blocked` — blocked by an external dependency or a user decision;
- `status:ready-for-review` — implementation is complete and needs review.

Close an Issue only after its acceptance criteria and checks are complete. Record known
limitations in the Issue before closing it.

## Priorities

- `priority:critical` — data loss, secret exposure, integrity violation, or a blocking
  security defect;
- `priority:high` — recurring incorrect synchronization, service unavailability, or a
  serious operational failure;
- `priority:medium` — important architecture or technical debt without immediate data
  loss;
- `priority:low` — quality, UX, documentation, or secondary technical debt.

Priority defines the order of work, not the estimated complexity.

## Issue requirements

Every Issue should include:

- a concise title describing the problem or outcome;
- context and the observable current behavior;
- expected behavior;
- affected technical areas and files/components when known;
- testable acceptance criteria;
- risks, constraints, and a migration plan when state or data formats change.

For defects, document a reproducible scenario and its consequences first. The exact
implementation may be refined during development, but changing acceptance criteria must
be explicit.

## Development rules

- One Issue represents one logically complete task. Split larger work into a parent Issue
  and related Issues.
- Do not mix a critical fix with unrelated refactoring.
- When changing user data, provide recovery, atomic writes, and interruption handling
  where applicable.
- Sync changes must include tests for conflicts, deletion, retries, and network failure.
- Never write secrets, tokens, or complete API response bodies to stdout, logs, Issues, or
  commits.
- Final progress updates must list changed files, checks performed, and remaining risks.
- When finishing work, the final progress update must include the commit title. If no
  commit was created, explicitly state that and provide the recommended commit title.

## Current audit priority order

Create and take Issues in this order:

1. Data loss during conflicts and deletions.
2. Token security and excessive GitHub permissions.
3. Concurrent autosync access to shared state.
4. Atomic state and queue persistence.
5. GitHub Contents API limits and scalability.
6. Watcher, network, and platform-service reliability.

Any new task involving data loss or secret exposure takes precedence over this list.
