# Commit Message Validation Guide

This guide explains how to write valid commit messages for this repository and how to fix validation errors.

## Required Format

All commit messages must include a description with a `Resolves:` line in one of these formats:

> **Note:** This validation supports any issue tracking system that uses the `PREFIX-NUMBER` format (e.g., MTV-123, DPP-17811, CCSINTL-298, JIRA-456, etc.). The PREFIX must be uppercase letters and NUMBER must be digits.

### Single Issue
```
Resolves: MTV-123
Resolves: DPP-17811
Resolves: CCSINTL-298
Resolves: JIRA-456
```

### Single Issue with Description
```
Resolves: MTV-3111 | Add validations for VDDK and RCM
Resolves: DPP-17811 | Fix authentication bug
Resolves: CCSINTL-298 | Update payment processing
```

### Multiple Issues (Choose ONE separator style)

**Space-separated:**
```
Resolves: MTV-123 MTV-456
Resolves: DPP-17811 DPP-17812
Resolves: MTV-123 DPP-456 CCSINTL-789
Resolves: MTV-123 MTV-456 | Fix multiple authentication issues
```

**Comma-separated:**
```
Resolves: MTV-123, MTV-456
Resolves: DPP-17811,CCSINTL-298
Resolves: MTV-123, DPP-456, CCSINTL-789
Resolves: MTV-123, MTV-456 | Authentication and authorization fixes
```

**"And" separated:**
```
Resolves: MTV-123 and MTV-456
Resolves: DPP-17811 and CCSINTL-298
Resolves: MTV-123 and DPP-456 and CCSINTL-789
Resolves: MTV-123 and MTV-456 | Complete authentication system overhaul
```

### No Associated Ticket
```
Resolves: None
```

## Important Rules

- **Do NOT mix separator styles** in the same line (e.g., `MTV-123, MTV-456 and DPP-789` is invalid)
- Issue numbers must be numeric (e.g., `MTV-abc` or `DPP-xyz` is invalid)
- Issue prefixes must be uppercase (e.g., use `MTV-123` not `mtv-123`)
- Issue format: `PREFIX-NUMBER` where PREFIX is uppercase letters and NUMBER is digits
- The `Resolves:` line can appear anywhere in the commit message body
- You can mix different issue types in the same line (e.g., `MTV-123 DPP-456`)
- Optional descriptions can be added after ` | ` (pipe with spaces): `Resolves: MTV-123 | Description`

## Example Valid Commit Messages

```
Fix user authentication bug

Updated the login validation to handle edge cases properly.
This resolves issues with special characters in passwords.

Resolves: MTV-456
```

```
Add new dashboard features

Implemented user dashboard with analytics and reporting.
Added export functionality and improved UI responsiveness.

Resolves: DPP-17811, CCSINTL-298, MTV-125
```

```
Integrate payment processing system

Connected the new payment gateway and updated checkout flow.
Fixed currency conversion issues for international users.

Resolves: DPP-17811 and CCSINTL-298 | Payment system integration
```

```
chore: update dependencies

Resolves: None
```

## Supported Issue Tracking Systems

This validation works with any issue tracking system that follows the `PREFIX-NUMBER` format:

- **MTV**: MTV-123, MTV-4567
- **DPP**: DPP-17811, DPP-20001
- **CCSINTL**: CCSINTL-298, CCSINTL-1000
- **JIRA**: JIRA-456, JIRA-789
- **GitHub Issues**: GH-123, ISSUE-456
- **Custom prefixes**: Any uppercase letters followed by dash and numbers

## Automatically Skipped Commits

The following commits are automatically skipped and don't need `Resolves:` lines:

- **Bot users:** dependabot, renovate, github-actions, ci, automated, etc.
- **Chore commits:** Messages containing `chore:` or `chore(` format

## Quick Fix Guide

### For the LATEST commit (most common case)
```bash
git commit --amend
# Edit your commit message to include a 'Resolves:' line
```

### For OLDER commits in your branch
```bash
git rebase -i HEAD~N  # where N is the number of commits to go back
# Mark commits as 'edit' or 'reword' to fix them
```

### For commits in a PULL REQUEST
1. Fix the commits using the methods above
2. Force push: `git push --force-with-lease`

## Common Validation Errors

### Missing Description
**Problem:** Your commit only has a subject line.

**Solution:** Add a description with a `Resolves:` line:
```bash
git commit --amend -m "Your subject line

Add your description here explaining what was changed.

Resolves: MTV-XXXX"
```

### Invalid Format
**Problem:** The `Resolves:` line doesn't match the required format.

**Examples of invalid formats:**
- `Resolves: MTV-` (missing number)
- `Resolves: mtv-123` (lowercase prefix)
- `Resolves: MTV-abc` (non-numeric issue number)
- `Resolves: MTV-123, MTV-456 and DPP-789` (mixed separators)
- `Resolves: 123-MTV` (wrong format - number before prefix)

**Solution:** Replace with a valid format from the examples above.

## Testing Your Commit Messages

You can test your commit messages locally using:
```bash
./scripts/validate-commits.sh                    # Validate latest commit
./scripts/validate-commits.sh --range HEAD~5..HEAD  # Validate last 5 commits
./scripts/validate-commits.sh --verbose         # Show detailed output
```

## Need Help?

If you're still having trouble with commit message validation:

1. Check the specific error message for details about what's wrong
2. Review the examples in this guide
3. Use the quick fix commands above to amend your commits
4. Test locally with the validation script before pushing

For questions about this validation process, please reach out to the development team.
