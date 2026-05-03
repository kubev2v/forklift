# Forklift AI Contribution Policy

> This policy is adapted from the [KubeVirt AI Contribution Policy](https://github.com/kubevirt/community/blob/main/ai-contribution-policy.md). Changes have been made to reflect Forklift-specific practices.

## Overview

This policy establishes guidelines for contributions that involve Artificial
Intelligence (AI) tools, including but not limited to Large Language Models
(LLMs), code generation tools, and AI-assisted development environments. This
is a living document that will evolve as AI technology and legal frameworks
mature.

## Motivation

AI tools are powerful assistants that can allow developers to become more
productive when configured and used correctly.

This policy encourages their use within the Forklift project to boost both
productivity and innovation while ensuring transparency. This allows the community
to learn and refine our policies and practices accordingly in order to maximise
the value of these tools.

### Contributor Accountability

AI tools can produce verbose, over-engineered, or superficially-correct code
that places a disproportionate review burden on maintainers. Disclosure creates
accountability and helps ensure contributors take ownership of AI-assisted work.
Contributors are expected to:

- Thoroughly review and understand every line of AI-generated code before
  submission
- Refine and groom AI output to meet project quality standards
- Take full ownership of all submitted content regardless of origin

Low-effort submissions that appear to be unreviewed AI output may be rejected
without detailed feedback until properly refined. This applies to all
contributions, but is particularly relevant for AI-assisted work.

### Legal and Copyright Rationale

Disclosure also serves important legal purposes. Copyright law in this area
continues to evolve, and as of current legal guidance, computer-generated work
may not be considered an original work eligible for copyright protection in many
jurisdictions. Additionally:

- AI training data may originate from materials with unclear or incompatible
  licenses
- Some AI tool vendors may retain rights to generated output, which could
  conflict with open source licensing
- Proper attribution helps maintain the integrity of the project's licensing
  under Apache 2.0

For further reading on these legal considerations, see the [OpenInfra Foundation
AI Policy](https://openinfra.org/legal/ai-policy/) and [AI-Assisted Development
and Open Source: Navigating Legal
Issues](https://www.redhat.com/en/blog/ai-assisted-development-and-open-source-navigating-legal-issues).

## AI Tool Disclosure Requirements

### Disclosure

All contributors **SHOULD** disclose AI tool use when submitting code,
documentation, or other content to the Forklift project.

Disclosure **SHOULD** take the form of a trailer line within the commit
attributing the AI tool used. Acceptable formats include:

- `Assisted-by: Claude `
- `Co-authored-by: Claude `
- `Generated-by: Claude `

Many AI coding tools automatically add `Co-authored-by` trailers—this is
acceptable and need not be changed to `Assisted-by`.

### Scope of Disclosure

Disclosure is expected when AI tools have materially contributed to the
submitted content.

**Requires disclosure:**

- AI wrote a function, class, or significant code block that you included
- AI suggested an algorithm, architecture, or approach you adopted
- AI generated tests, documentation, or commit messages you used
- AI-suggested solutions, refactoring, or significant debugging help that
  shaped the final implementation

**Does not require disclosure:**

- General Q&A or learning (even if it informed your approach)
- IDE autocomplete (Copilot line completions, IntelliSense)
- Using AI to explain existing code
- Asking AI to review your human-written code
- Spell checking or minor syntax corrections
- Content that has been substantially rewritten such that the original AI
  output is no longer recognizable

When in doubt, err on the side of disclosure—transparency benefits the
community.

## Acceptable Uses of AI Tools

AI tools are **accepted** as development assistants for:

- **Code scaffolding**: Generating boilerplate code and initial implementations
- **Refactoring**: Suggesting code improvements and modernization
- **Testing**: Creating test cases and test data
- **Documentation**: Drafting technical documentation and code comments
- **Debugging**: Identifying potential issues and suggesting fixes
- **Research**: Exploring architectural approaches and best practices

## Contributor Responsibilities

Contributors are encouraged to leverage AI tools and are responsible to review and
understand the content they are contributing. For code this must meet the
existing coding standards for the project.

## Legal and Licensing Considerations

### Copyright Compliance

Contributors must ensure that:

- AI tool terms of service do not conflict with Apache 2.0 licensing
- No copyrighted material is inadvertently included in AI-generated output
- All third-party content is properly attributed and licensed

### Employer Policies

Contributors should verify that their use of AI tools complies with their
employer's policies regarding AI-generated code in open source contributions.

## Review Process

### Review Criteria

As with all contributions to the project, reviewers should evaluate:

- Code quality and adherence to project standards
- Appropriate test coverage
- Security implications
- Long-term maintainability

## Policy Evolution

This policy will be regularly reviewed and updated to reflect:

- Changes in AI technology capabilities
- Legal and regulatory developments
- Community feedback and experience
- Industry best practices

This policy could be eventually removed once these tools become standard
development tools and the policy is superseded by other contribution
requirements.

## References

- [KubeVirt AI Contribution Policy](https://github.com/kubevirt/community/blob/main/ai-contribution-policy.md) — the original policy this document is adapted from
- [Linux Foundation Generative AI Guidelines](https://www.linuxfoundation.org/legal/generative-ai)
- [OpenInfra Foundation AI Policy](https://openinfra.org/legal/ai-policy/)
- [AI-Assisted Development and Open Source: Navigating Legal Issues](https://www.redhat.com/en/blog/ai-assisted-development-and-open-source-navigating-legal-issues)
- [Avocado Framework AI Policy](https://avocado-framework.readthedocs.io/en/latest/guides/contributor/chapters/ai_policy.html)
- [QEMU Code Provenance Policy](https://www.qemu.org/docs/master/devel/code-provenance.html#use-of-ai-content-generators)
- [Ghostty AI Policy](https://github.com/ghostty-org/ghostty/blob/main/AI_POLICY.md)
