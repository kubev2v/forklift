# Code Review

## Design First

Before writing code, the idea, design, and architecture must be discussed and approved by a maintainer.

### Feature Design Document (FID)

New features require a Feature Design Document (FID) that has been reviewed and approved by a maintainer before implementation begins. The FID captures the problem, proposed solution, design decisions, and trade-offs so that reviewers and future contributors can understand the reasoning behind the implementation.

Use the [FID template](../docs/enhancements/fid-template.md) to create your document. The FID should be submitted as a PR to `docs/enhancements/` for review.

For smaller changes (bug fixes, refactors, minor improvements), a Jira ticket with a clear description is sufficient -- a full FID is not required. When in doubt, ask a maintainer.

**Code that lacks an approved design may be rejected regardless of quality.**

## Review Philosophy

When reviewing a pull request, consider these questions in order:

1. **Is the idea behind the contribution sound?** The problem being solved should be real, and the approach should be backed by an approved FID or Jira ticket.
2. **Is the contribution architected correctly?** The design should be maintainable, consistent with the rest of the codebase, and match the approved FID.
3. **Is the contribution polished?** Code style, tests, documentation, and commit messages should all be in good shape.

## Reviewer Checklist

- Changes are clear and easy to review
- Any non-trivial change has appropriate comments explaining intent
- Code is consistent with the PR description
- No unnecessary or unrelated changes are included

## Submitter Checklist

- Tests pass locally (`make test`)
- Linting passes (`make lint`)
- PR description clearly explains what changed and why
- Commit messages follow the [required format](../.cursor/skills/git-commit/commit-message-guide.md)

### If the PR changes CRDs

- Generated code is up to date (`make generate-verify`)
- Upstream and downstream manifests are regenerated (`make update-manifests`)
- CRD schema validation passes (`make validate-forklift-controller-crd`)
- Backward compatibility is considered -- existing CRs should not break on upgrade
- New fields have appropriate defaults, validation markers, and documentation

## Resolving Review Feedback

If the above criteria are not met or the change does not make sense to the reviewer, the developer must address the feedback before the PR can be reviewed again.

## References

These review practices are informed by Kubernetes community standards:

- [Kubernetes Code Review Expectations](https://github.com/kubernetes/community/blob/master/contributors/guide/expectations.md#code-review)
- [The Gentle Art of Patch Review](http://sage.thesharps.us/2014/09/01/the-gentle-art-of-patch-review/)
