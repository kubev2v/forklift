---
name: minimal-changes
description: >-
  Keep code changes and commits small, simple, and focused. Use when implementing
  features, fixing bugs, refactoring, or reviewing diffs in this repo.
---

# Minimal Changes

Apply these principles to every change in this repository.

## Core rules

- I want the changes to be as small as possible
- as simple as possible, don't create many unnecessary functions
- keep all code changes and commits short and straight forward

## Code

1. **Smallest correct diff** — Change only what the task requires. No drive-by refactors, renames, or formatting outside touched lines.
2. **Solve inline** — Prefer editing existing code over new helpers, wrappers, or abstractions. Add a function only when reuse or clarity clearly demands it.
3. **Match the file** — Follow naming, types, and patterns already in the surrounding code.
4. **Skip noise** — No extra comments, tests, or docs unless requested or required for the fix.

## Commits

Keep each commit focused on one logical change with a short, direct message. For commit format and Jira linking, use the [git-commit](git-commit/SKILL.md) skill.

## Before finishing

- Can anything be removed and still solve the problem?
- Would a reviewer ask why unrelated lines changed?
- Is there a helper that could be three inline lines instead?

If yes to the last two, simplify further.
