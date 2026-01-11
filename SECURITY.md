# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

The KubeRDE team takes security issues seriously. We appreciate your efforts to responsibly disclose your findings.

### How to Report a Security Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **Email**: Send details to [security@kuberde.io](mailto:security@kuberde.io)
2. **GitHub Security Advisory**: Use GitHub's [private vulnerability reporting](https://github.com/xsoloking/kube-rde/security/advisories/new)

### What to Include in Your Report

To help us understand and address the issue quickly, please include:

- **Description**: A clear description of the vulnerability
- **Impact**: What an attacker could achieve by exploiting this vulnerability
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Proof of Concept**: Code, screenshots, or other evidence demonstrating the vulnerability
- **Environment**: Version of KubeRDE, Kubernetes version, cloud provider, etc.
- **Suggested Fix**: If you have ideas on how to fix it (optional)

### What to Expect

After you submit a vulnerability report:

1. **Acknowledgment**: We will acknowledge receipt within 48 hours
2. **Initial Assessment**: We will provide an initial assessment within 5 business days
3. **Investigation**: We will investigate and validate the report
4. **Fix Development**: We will develop and test a fix
5. **Disclosure Timeline**: We will work with you to determine an appropriate disclosure timeline
6. **Credit**: We will credit you in the security advisory (unless you prefer to remain anonymous)

### Disclosure Policy

- **Coordinated Disclosure**: We follow coordinated disclosure practices
- **Embargo Period**: We typically request a 90-day embargo from initial report to public disclosure
- **Early Disclosure**: We may disclose earlier if the vulnerability is being actively exploited
- **Public Advisory**: Once fixed, we will publish a security advisory with full details

## Security Best Practices

### For Deployment

When deploying KubeRDE in production, follow these security best practices:

#### 1. Authentication and Authorization

- **Use OIDC/OAuth2**: Always use proper OIDC authentication with a trusted provider (e.g., Keycloak)
- **Strong Secrets**: Use strong, randomly generated secrets for all credentials
- **Secret Management**: Store secrets in Kubernetes Secrets or external secret managers (e.g., HashiCorp Vault)
- **Rotate Credentials**: Regularly rotate client secrets and tokens

```yaml
# Example: Strong secret generation
apiVersion: v1
kind: Secret
metadata:
  name: kuberde-secrets
  namespace: kuberde
type: Opaque
stringData:
  client-secret: $(openssl rand -base64 32)
```

#### 2. Network Security

- **TLS Everywhere**: Always use TLS for all external-facing services
- **Network Policies**: Implement Kubernetes Network Policies to restrict traffic
- **Ingress Security**: Use WAF (Web Application Firewall) with your Ingress controller
- **Private Networks**: Deploy in private VPCs/VNets when possible

```yaml
# Example: Network Policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kuberde-server
  namespace: kuberde
spec:
  podSelector:
    matchLabels:
      app: kuberde-server
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: kuberde
    ports:
    - protocol: TCP
      port: 8080
```

#### 3. Resource Isolation

- **Pod Security Standards**: Enforce restricted Pod Security Standards
- **Security Contexts**: Use appropriate security contexts for all containers
- **Resource Limits**: Set CPU and memory limits to prevent resource exhaustion
- **User Quotas**: Implement and enforce user quotas

```yaml
# Example: Security Context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
```

#### 4. Database Security

- **Encryption at Rest**: Enable PostgreSQL encryption at rest
- **Encryption in Transit**: Always use SSL/TLS for database connections
- **Strong Passwords**: Use strong, randomly generated database passwords
- **Regular Backups**: Implement automated, encrypted backups
- **Access Control**: Restrict database access to only necessary services

```bash
# Example: Strong database password
export POSTGRES_PASSWORD=$(openssl rand -base64 32)
```

#### 5. Audit Logging

- **Enable Audit Logs**: Configure comprehensive audit logging
- **Log Retention**: Retain audit logs for compliance requirements
- **Log Analysis**: Regularly analyze logs for suspicious activity
- **SIEM Integration**: Integrate with SIEM systems for monitoring

#### 6. Updates and Patching

- **Stay Updated**: Regularly update to the latest KubeRDE version
- **Monitor Advisories**: Subscribe to security advisories
- **Dependency Updates**: Keep all dependencies updated
- **Vulnerability Scanning**: Regularly scan container images for vulnerabilities

```bash
# Example: Check for updates
docker pull soloking/kuberde-server:latest
docker pull soloking/kuberde-operator:latest
docker pull soloking/kuberde-agent:latest
```

### For Development

#### Secure Coding Practices

- **Input Validation**: Validate all user inputs
- **SQL Injection Prevention**: Use parameterized queries (GORM handles this)
- **XSS Prevention**: Sanitize output in Web UI
- **CSRF Protection**: Implement CSRF tokens for state-changing operations
- **Rate Limiting**: Implement rate limiting on API endpoints

#### Dependency Security

- **Dependency Scanning**: Use `go mod verify` and dependency scanning tools
- **Minimal Dependencies**: Keep dependencies to a minimum
- **Regular Updates**: Regularly update dependencies
- **License Compliance**: Ensure dependency licenses are compatible

```bash
# Check for known vulnerabilities
go list -json -deps ./... | nancy sleuth
```

### For Operations

#### Monitoring and Alerting

- **Security Monitoring**: Monitor for security events
- **Anomaly Detection**: Set up alerts for unusual patterns
- **Failed Auth Attempts**: Alert on repeated authentication failures
- **Resource Anomalies**: Alert on unexpected resource usage

#### Incident Response

- **Incident Plan**: Have an incident response plan ready
- **Backup and Recovery**: Test backup and recovery procedures
- **Communication Plan**: Know how to communicate security issues
- **Forensics**: Preserve logs for post-incident analysis

## Known Security Considerations

### Agent Trust Model

- **Agent Authentication**: Agents authenticate using OAuth2 Client Credentials flow
- **Agent Authorization**: Only the workspace owner can access their agents
- **Agent Isolation**: Each agent runs in an isolated pod with resource limits
- **Agent ID Pattern**: Agent IDs follow `user-{owner}-{name}` format for authorization

### WebSocket Security

- **Authentication Required**: All WebSocket connections require valid JWT tokens
- **Token Validation**: Tokens are validated on every connection
- **Session Timeout**: WebSocket sessions have configurable timeouts
- **Reconnection**: Agents automatically reconnect with fresh tokens

### Multi-Tenancy

- **User Isolation**: Workspaces are isolated by user
- **Namespace Isolation**: Each workspace can have dedicated namespaces
- **Resource Quotas**: Per-user resource quotas prevent resource exhaustion
- **Audit Trail**: All actions are logged with user identification

## Security Hardening Checklist

Before deploying to production, ensure:

- [ ] TLS certificates are valid and from a trusted CA
- [ ] All secrets are stored securely (Kubernetes Secrets or external vault)
- [ ] Network policies are in place
- [ ] Pod Security Standards are enforced
- [ ] OIDC authentication is properly configured
- [ ] Database encryption is enabled
- [ ] Audit logging is enabled and monitored
- [ ] Resource quotas are configured
- [ ] Regular backups are configured and tested
- [ ] Security updates are applied
- [ ] Vulnerability scanning is automated
- [ ] Incident response plan is documented

## Security Resources

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

## Contact

For security-related questions or concerns, contact:
- **Email**: [security@kuberde.io](mailto:security@kuberde.io)
- **GitHub**: Create a private security advisory

## Acknowledgments

We would like to thank the following security researchers for responsibly disclosing vulnerabilities:

- (List will be updated as vulnerabilities are reported and fixed)

---

*This security policy is subject to change. Last updated: 2025-01-04*
