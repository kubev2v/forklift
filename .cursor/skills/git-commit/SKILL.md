---
name: git-commit
description: Compose CI-valid commit messages with Jira linking and sign-off. Use when the user asks to commit, write a commit message, or git commit. Never execute the commit — only present the message and command for the user to run.
---

# Git Commit Skill

## Step 1: Check Branch

Run `git branch --show-current`. If on `main`, **warn the user** and ask if they want to continue or switch to a new branch first.

## Step 2: Gather Info

Use AskQuestion to ask:
1. **Jira issue ID** (e.g. `MTV-1234`) — if "None", ask for **chore type** (deps, docs, ci, test, refactor, lint, build)
2. **Add AI co-author line?** (Yes / No)

## Step 3: Analyze Changes

Run `git diff HEAD` and `git status` to see all changes since the last commit (staged, unstaged, and untracked files). Use these diffs to understand what changed and why.

## Step 4: Generate Commit Message

Format:

```
MTV-XXXX | <imperative description, ~72 chars total>

<body: what changed and why, lines wrapped at 72 chars>

Ref: https://issues.redhat.com/browse/MTV-XXXX
Resolves: MTV-XXXX
```

Title (~72 chars max, imperative mood):
- **With Jira:** `MTV-XXXX | description`
- **Without Jira:** `chore(type): description` (skips CI — no `Ref:`/`Resolves:` needed)

Trailers (with Jira only):
- `Ref: https://issues.redhat.com/browse/MTV-XXXX`
- `Resolves: MTV-XXXX` (required by CI)

If co-author requested, append: `Co-authored-by: AI Assistant <noreply@cursor.com>`

## Step 5: Present Message and Command

Present the full commit message in a code block, followed by a ready-to-run `git` command the user can copy-paste:

```bash
git add <relevant files>
git commit -s -m "$(cat <<'EOF'
<the commit message>
EOF
)"
```

**Never run `git add` or `git commit` yourself.** The user will review, adjust if needed, and execute the commit manually.
