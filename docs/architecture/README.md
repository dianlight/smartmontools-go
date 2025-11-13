# Architecture Documentation

This directory contains Architecture Decision Records (ADRs) and other architectural documentation for the smartmontools-go project.

## What is an ADR?

An Architecture Decision Record (ADR) is a document that captures an important architectural decision made along with its context and consequences. ADRs help teams:

- Document the reasoning behind architectural choices
- Provide context for future developers
- Track the evolution of the system architecture
- Support informed decision-making

## ADR Index

### [ADR-001: SMART Data Access Approaches](./ADR-001-smart-access-approaches.md)

**Status:** Proposed

**Summary:** Comprehensive analysis of four different approaches for accessing SMART data from storage devices:

1. **smartctl Command Wrapper (Current)** - Execute external smartctl binary and parse JSON output
2. **Direct ioctl Access** - Low-level kernel system calls for maximum performance
3. **Shared Library with FFI** - Use smartmontools as a shared library without CGO
4. **Hybrid Approach** - Combine ioctl and shared library for optimal flexibility

The document includes:
- Detailed architecture diagrams for each approach
- Code examples and implementation details
- Performance comparisons and benchmarks
- Platform support matrix
- Security and maintenance considerations
- Recommendations for different use cases

**Key Recommendation:** Continue with Option 1 (smartctl wrapper) for current v0.x development, with a roadmap toward Option 4 (hybrid) for future v1.0+ releases.

## ADR Template

When creating new ADRs, use this template:

```markdown
# ADR-XXX: [Title]

## Status

[Proposed | Accepted | Deprecated | Superseded]

## Context

What is the issue that we're seeing that is motivating this decision or change?

## Decision

What is the change that we're proposing and/or doing?

## Consequences

What becomes easier or more difficult to do because of this change?

## References

Links to related documents, discussions, or implementations.
```

## Contributing

When making significant architectural decisions:

1. Create a new ADR using the template above
2. Number it sequentially (ADR-002, ADR-003, etc.)
3. Discuss with the team before marking as "Accepted"
4. Update this index with a summary
5. Reference the ADR in related code changes

## Architecture Principles

The smartmontools-go project follows these architectural principles:

1. **Simplicity First**: Prefer simple, maintainable solutions over complex optimizations
2. **Cross-Platform**: Support Linux, macOS, and Windows where feasible
3. **Minimal Dependencies**: Avoid unnecessary external dependencies
4. **Performance Awareness**: Design with performance in mind, but don't sacrifice maintainability
5. **Clear Abstractions**: Provide clean, well-documented interfaces
6. **Backward Compatibility**: Maintain API stability within major versions
7. **Security**: Consider security implications of all architectural decisions

## Related Documentation

- [API Documentation](../../APIDOC.md) - Complete API reference
- [README](../../README.md) - Project overview and quick start
- [Examples](../../examples/) - Usage examples
