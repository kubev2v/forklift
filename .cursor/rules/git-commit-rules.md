# Git Commit Rules for Forklift

## Required Commit Rules

### 1. Single Commit for Upstream Merge
**All pull requests to upstream must contain exactly ONE commit.**

If you have multiple commits on your branch, you MUST squash them into a single commit before creating the pull request.

#### Squashing Multiple Commits
```bash
# Interactive rebase to squash commits
git rebase -i HEAD~n  # where n is the number of commits to squash

# In the interactive editor:
# - Keep the first commit as 'pick'  
# - Change all others to 'squash' or 's'
# - Save and exit
# - Edit the combined commit message in the next editor

# Alternative: Reset and recommit
git reset --soft HEAD~n  # where n is number of commits
git commit -s -m "MTV-XXXX Your consolidated commit message

Link: https://issues.redhat.com/browse/MTV-XXXX"
```

### 2. Jira Task Link Required
**Every commit message MUST include a link to the corresponding Jira task.**

The link should be in the format:
```
Link: https://issues.redhat.com/browse/MTV-XXXX
```

#### AI Assistant Guidance
When creating commits, always ask the user:
> "Please provide the Jira task link for this commit (format: https://issues.redhat.com/browse/MTV-XXXX)"

### 3. MTV Task ID in Commit Header
**The commit header/title MUST start with the MTV task ID.**

Format: `MTV-XXXX Brief description of the change`

Examples:
- `MTV-2687 Add VDDK validation for RawCopyMode migrations`
- `MTV-1234 Fix network mapping validation for VMware VMs`
- `MTV-5678 Update warm migration documentation`

### 4. Signed Commits Required
**All commits MUST be signed using the `-s` flag.**

```bash
git commit -s -m "commit message"
```

This adds the "Signed-off-by" line required by the project.

## Complete Commit Message Format

```
MTV-XXXX Brief description of the change (max 50 chars)

Detailed explanation of what was changed and why. This should
explain the problem being solved and how the solution works.
Reference any important implementation details.

- Key change 1
- Key change 2  
- Key change 3

Link: https://issues.redhat.com/browse/MTV-XXXX
```

## Example Commit Messages

### Good Examples

```
MTV-2687 Add VDDK validation for RawCopyMode migrations

When SkipGuestConversion (RawCopyMode) is enabled, VDDK is required
for VMware migrations. This adds a critical validation that blocks
migration execution when VDDK image is not configured on the provider,
similar to the existing warm migration validation.

The validation prevents runtime failures by catching the missing
requirement during plan validation phase.

Link: https://issues.redhat.com/browse/MTV-2687
```

```
MTV-1234 Fix CBT validation for warm migrations

Updated the Change Block Tracking validation to properly handle
edge cases where CBT is disabled on individual disks rather than
the entire VM. This prevents false positives in warm migration
validation.

- Check CBT status per disk instead of per VM
- Add detailed error messages with disk identifiers
- Update validation tests for new logic

Link: https://issues.redhat.com/browse/MTV-1234
```

### Bad Examples

```
❌ fix bug
❌ MTV-1234 updated code  
❌ Add validation (missing MTV ID, no link, not signed)
❌ Updated validation for VDDK (missing MTV ID and link)
```

## Workflow for AI Assistants

When helping users create commits, follow this checklist:

1. **Ask for Jira link**: "What is the Jira task link for this change?"
2. **Confirm MTV ID**: Extract and confirm the MTV ID from the link
3. **Create proper header**: `MTV-XXXX Brief description`
4. **Add detailed body**: Explain what and why
5. **Include Jira link**: Add the link at the bottom
6. **Use signed commit**: Always include `-s` flag

### Commit Template for AI Assistants

```bash
git commit -s -m "MTV-{ID} {Brief description}

{Detailed explanation of changes}

{List key modifications if applicable}

Link: {Jira URL}"
```

## Branch Management

### Working with Feature Branches

```bash
# Create feature branch
git checkout -b feature/mtv-xxxx-description

# Make changes and commit (can be multiple commits during development)
git add .
git commit -s -m "MTV-XXXX Work in progress - component A"
git commit -s -m "MTV-XXXX Work in progress - component B"  
git commit -s -m "MTV-XXXX Complete implementation"

# Before creating PR, squash to single commit
git rebase -i HEAD~3  # Squash all 3 commits

# Push to your fork
git push origin feature/mtv-xxxx-description

# Create PR from your fork to upstream
```

### Updating Branch with Upstream Changes

```bash
# Fetch latest upstream
git fetch upstream

# Rebase your branch on latest upstream  
git rebase upstream/main

# If conflicts, resolve them and continue
git add .
git rebase --continue

# Force push to update your branch (after squashing)
git push --force-with-lease origin feature/mtv-xxxx-description
```

## Pre-commit Checklist

Before creating a pull request, verify:

- [ ] Only ONE commit in the branch
- [ ] Commit starts with `MTV-XXXX`
- [ ] Commit includes Jira link
- [ ] Commit is signed (`git log` shows "Signed-off-by")
- [ ] Commit message is clear and descriptive
- [ ] Branch is up to date with upstream/main

## Troubleshooting

### Forgot to Sign Commit
```bash
# Amend the last commit to add signature
git commit --amend -s --no-edit
```

### Need to Add Jira Link to Existing Commit
```bash
# Amend the commit message
git commit --amend -s

# Edit the message to add the Jira link
```

### Multiple Commits Need Squashing
```bash
# Interactive rebase
git rebase -i HEAD~n  # where n = number of commits

# Or reset and recommit
git reset --soft HEAD~n
git commit -s  # Create new commit with proper format
```

### Commit Message Too Long
Keep the first line under 50 characters:
- Use present tense ("Add" not "Added")
- Be concise but descriptive
- Put details in the body, not the header

## Integration with Development Tools

### VS Code Integration
Configure VS Code to use commit templates:

```json
{
    "git.inputValidation": "always",
    "git.inputValidationLength": 50,
    "git.inputValidationSubjectLength": 50
}
```

### Git Hooks
Consider setting up pre-commit hooks to validate:
- MTV ID presence
- Signed-off-by line
- Jira link format
- Commit message length

This ensures all commits follow the required format before they are created.
