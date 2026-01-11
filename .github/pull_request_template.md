## Description

<!-- Provide a clear and concise description of what this PR does -->

## Type of Change

<!-- Mark the relevant option with an "x" -->

- [ ] 🐛 Bug fix (non-breaking change that fixes an issue)
- [ ] ✨ New feature (non-breaking change that adds functionality)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📝 Documentation update
- [ ] 🔧 Configuration change
- [ ] ♻️ Code refactoring (no functional changes)
- [ ] ⚡ Performance improvement
- [ ] ✅ Test improvement
- [ ] 🔨 Build/CI improvement
- [ ] 🎨 UI/UX improvement

## Related Issues

<!-- Link related issues. Use "Closes #123" or "Fixes #123" for issues this PR resolves -->

Closes #
Related to #

## Changes Made

<!-- List the specific changes made in this PR -->

-
-
-

## Component(s) Affected

<!-- Mark all that apply -->

- [ ] Server
- [ ] Operator
- [ ] Agent
- [ ] Web UI
- [ ] CLI
- [ ] Documentation
- [ ] Deployment/Infrastructure
- [ ] Tests
- [ ] CI/CD

## Testing

<!-- Describe the testing you've performed -->

### Test Environment

- Kubernetes Platform: <!-- e.g., GKE, EKS, kind, minikube -->
- Kubernetes Version: <!-- e.g., 1.28.0 -->
- KubeRDE Version: <!-- e.g., commit hash or branch -->

### Test Cases

<!-- Describe test scenarios and results -->

- [ ] Unit tests pass (`make test` or `go test ./...`)
- [ ] Integration tests pass (if applicable)
- [ ] Manual testing completed

**Test Steps:**
1.
2.
3.

**Test Results:**
-

## Screenshots/Recordings

<!-- If applicable, add screenshots or recordings to demonstrate the changes -->

### Before
<!-- Screenshot or description of behavior before changes -->

### After
<!-- Screenshot or description of behavior after changes -->

## Breaking Changes

<!-- If this is a breaking change, describe the impact and migration path -->

**Impact:**
-

**Migration Guide:**
1.
2.

## Documentation

<!-- Have you updated relevant documentation? -->

- [ ] Code comments updated
- [ ] README.md updated (if needed)
- [ ] CLAUDE.md updated (if needed)
- [ ] docs/ updated (if needed)
- [ ] API documentation updated (if applicable)
- [ ] No documentation changes needed

## Deployment Notes

<!-- Any special deployment considerations? -->

- [ ] Requires database migration
- [ ] Requires configuration changes
- [ ] Requires new environment variables
- [ ] Backward compatible
- [ ] Requires coordinated deployment (server + operator + agent)

**Deployment Steps:**
1.
2.

## Performance Impact

<!-- Does this change affect performance? -->

- [ ] No performance impact
- [ ] Improves performance (describe below)
- [ ] May impact performance (describe below)

**Details:**
-

## Security Considerations

<!-- Does this change have security implications? -->

- [ ] No security impact
- [ ] Enhances security (describe below)
- [ ] Potential security impact (describe below and notify security team)

**Details:**
-

## Checklist

<!-- Ensure all items are completed before requesting review -->

### Code Quality

- [ ] My code follows the project's coding standards
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have removed any debug code, console.logs, or commented-out code
- [ ] My changes generate no new warnings or errors
- [ ] I have run `make fmt` and `make lint` (for Go code)
- [ ] I have run linting for frontend code (if applicable)

### Testing

- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] I have tested this change in a real Kubernetes environment
- [ ] I have verified backward compatibility (if applicable)

### Documentation

- [ ] I have updated the documentation accordingly
- [ ] I have added/updated code comments for public APIs
- [ ] I have updated the CHANGELOG (if applicable)

### Review Readiness

- [ ] I have rebased my branch on the latest main
- [ ] I have resolved all merge conflicts
- [ ] I have kept commits atomic and well-described
- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/) format
- [ ] I have assigned appropriate labels to this PR
- [ ] I have requested review from appropriate team members

## Additional Notes

<!-- Any additional information that reviewers should know -->

## Reviewer Guidelines

<!-- For reviewers -->

**Focus Areas:**
- Code correctness and edge cases
- Security implications
- Performance impact
- Documentation clarity
- Test coverage

**Approval Criteria:**
- [ ] All CI checks passing
- [ ] Code reviewed and approved
- [ ] Documentation reviewed
- [ ] No unresolved conversations
- [ ] Breaking changes acknowledged (if any)

---

<!--
Thank you for contributing to KubeRDE! 🎉
Please ensure all checklist items are completed before requesting review.
-->
