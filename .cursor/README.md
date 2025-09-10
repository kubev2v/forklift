# Cursor AI Configuration for Forklift

This directory contains configuration and guidance files for AI assistants working with the Forklift project.

## Directory Structure

```
.cursor/
├── README.md                    # This file
└── rules/                      # AI assistant rules and guidelines
    ├── ci-automation-tooling.md # CI/CD, build tools, and development automation
    ├── coding-standards.md      # Go language standards and code style guidelines
    ├── controller-patterns.md   # Kubernetes controller patterns and reconciliation
    ├── git-commit-rules.md      # Git workflow and commit message requirements
    ├── migration-workflows.md   # Migration types, processes, and workflows
    ├── project-overview.md      # Comprehensive project information and architecture
    ├── testing-guidelines.md    # Testing patterns and best practices
    └── validation-patterns.md   # Validation logic patterns and OPA policies
```

## Purpose

The `.cursor` directory provides comprehensive guidance for AI assistants to work effectively with the Forklift codebase. This includes:

- **Project Overview**: Complete understanding of Forklift's purpose, architecture, and components
- **Code Standards**: Go language standards, formatting, error handling, and style guidelines
- **Development Workflow**: CI/CD tools, build automation, testing infrastructure, and debugging
- **Git Standards**: Commit message format, branching workflow, and contribution requirements
- **Architecture Patterns**: Controller patterns, validation flows, and migration workflows  
- **Testing Approaches**: Unit testing, integration testing, and OPA policy testing
- **Domain Knowledge**: Migration types, provider specifics, and validation requirements

## For AI Assistants

When working with Forklift:

1. **Read the main guide** in `/AGENTS.md` for project overview
2. **Reference specific rules** in `.cursor/rules/` for detailed patterns
3. **Follow established conventions** for consistency with existing code
4. **Understand migration workflows** before making changes
5. **Validate changes** against existing patterns and requirements

## For Developers

These files serve as:

- Documentation of coding standards and patterns
- Reference for new team members
- Guidelines for code reviews
- Templates for common implementations

## Key Concepts to Understand

- **Critical vs Warning conditions**: Critical conditions block migration execution
- **Provider adapters**: Each source platform (VMware, oVirt, etc.) has specific requirements
- **Migration types**: Cold, warm, live, and conversion-only have different workflows
- **Raw copy mode**: Requires VDDK for VMware but skips guest conversion
- **Validation pipeline**: Both controller and OPA policy validations run

## Common Patterns

### Adding Validations
1. Add controller validation in `pkg/controller/plan/validation.go`
2. Consider adding OPA policy in `validation/policies/`
3. Use appropriate condition categories (Critical, Warning, etc.)
4. Provide clear, actionable error messages

### Provider-Specific Logic
1. Check provider type before applying provider-specific logic
2. Use provider adapters for platform-specific operations
3. Handle provider capabilities appropriately
4. Validate provider-specific requirements

### Error Handling
1. Wrap errors with context using `liberr.Wrap()`
2. Set conditions for business logic errors
3. Return errors only for unexpected system issues
4. Use structured logging for debugging

## Contributing

When updating these guidelines:

1. Keep them current with code changes
2. Add examples for new patterns
3. Update documentation for new features
4. Ensure consistency across all rule files

These files help maintain code quality and consistency while enabling efficient AI-assisted development.
