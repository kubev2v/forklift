---
name: git-commit
description: Compose CI-valid commit messages with Jira linking and sign-off. Use when the user asks to commit, write a commit message, or git commit. Never execute the commit — only present the message and command for the user to run.
---

# Git Commit Skill

**Never run `git add` or `git commit` yourself.** The user will review, adjust if needed, and execute the commit manually.

For full commit message validation rules, formats, and troubleshooting, see [commit-message-guide.md](commit-message-guide.md) in this skill directory.

## Step 1: Check Branch

Run `git branch --show-current`. If on `main`, **warn the user** and ask if they want to continue or switch to a new branch first.

## Step 2: Gather Info

Use AskQuestion to ask:
1. **Jira issue ID** (e.g. `MTV-1234`) — if "None", ask for **commit type**: `chore` or `fix`, then ask for **scope** (deps, docs, ci, test, refactor, lint, build, validation, deployment, etc.)
2. **Add AI co-author line?** (Yes / No)

## Step 3: Analyze Changes

Run `git diff HEAD` and `git status` to see all changes since the last commit (staged, unstaged, and untracked files). Use these diffs to understand what changed and why.

## Step 4: Generate Commit Message

### Title Format (PR title validation enforced by CI)

The title (~72 chars max, imperative mood) must match one of these patterns:

- **With Jira:** `MTV-XXXX | description`
- **Without Jira (chore):** `chore(scope): description`
- **Without Jira (fix):** `fix(scope): description`

> **Important:** The `chore` and `fix` formats **require parentheses with a scope**.
> `fix: description` is **invalid** — use `fix(scope): description` instead.
> The scope should be a short word describing the area (e.g., deps, docs, ci, deployment, validation).

### Full Message Format

**With Jira:**
```
MTV-XXXX | <imperative description, ~72 chars total>

<body: what changed and why, lines wrapped at 72 chars>

Ref: https://redhat.atlassian.net/browse/MTV-XXXX
Resolves: MTV-XXXX
```

**Without Jira (fix or chore):**
```
fix(scope): <imperative description, ~72 chars total>

<body: what changed and why, lines wrapped at 72 chars>

Resolves: none
```

### Trailers

- **With Jira:** `Ref: https://redhat.atlassian.net/browse/MTV-XXXX` and `Resolves: MTV-XXXX`
- **Without Jira:** `Resolves: none` (required by CI for non-chore commits)
- **Chore commits:** No `Resolves:` line needed (CI skips validation for chore commits)

If co-author requested, append: `Co-authored-by: AI Assistant <noreply@cursor.com>`

## Step 5: Branch Naming Convention

If the user requests a new branch name, follow this convention:

- **With Jira:** `mtv-XXXX-short-description` (lowercase ticket, kebab-case summary)
- **Without Jira:** `fix-short-description` or `chore-short-description`

Rules:
- All lowercase
- Use hyphens to separate words (kebab-case)
- Keep it short (3–5 words max after the prefix)
- Use the lowercase Jira ticket as prefix when available

Examples:
- `mtv-6189-fix-warm-cutover-naming`
- `fix-protect-system-namespaces`
- `chore-update-dependencies`

## Step 6: Present Message and Command

Present the full commit message in a code block, followed by a ready-to-run `git` command the user can copy-paste:

```bash
git add <relevant files>
git commit -s -m "$(cat <<'EOF'
<the commit message>
EOF
)"
```
