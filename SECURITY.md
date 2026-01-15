# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Security Model

Ravenforge is designed with security as a core principle:

### Tool Sandboxing

- All tools run in OCI containers with:
  - Read-only root filesystem
  - No privileged mode
  - All Linux capabilities dropped
  - Resource limits (CPU, memory, PIDs)
  - Network disabled by default
  - Restricted filesystem access (only mounted inputs/outputs)

### Policy Enforcement

- All tool executions require policy approval
- Dangerous capabilities (network, response actions, AI) require explicit policy gates
- Policy decisions are immutably logged

### Audit Trail

- All operations are logged to an append-only audit log
- Logs include cryptographic hashes for integrity verification
- Full provenance tracking for all artifacts

## Reporting a Vulnerability

If you discover a security vulnerability in Ravenforge, please report it responsibly:

### Do NOT

- Open a public GitHub issue for security vulnerabilities
- Disclose the vulnerability publicly before it's fixed

### Do

1. Email security@ravenforge.dev with:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Any suggested fixes

2. Allow up to 90 days for us to address the issue before public disclosure

3. We will:
   - Acknowledge receipt within 48 hours
   - Provide an initial assessment within 7 days
   - Work with you on coordinated disclosure

## Security Best Practices for Users

### Deployment

1. Run the daemon as a dedicated non-root user
2. Use TLS for the REST API in production
3. Restrict API access to authorized users
4. Review tool manifests before registration
5. Enable all audit logging

### Tool Development

1. Follow the principle of least privilege
2. Only request capabilities that are necessary
3. Validate all inputs thoroughly
4. Use the provided SDK for artifact handling
5. Submit to security review before publishing

### Operations

1. Regularly review audit logs
2. Monitor for policy violations
3. Keep tools updated
4. Verify artifact hashes
5. Test pipelines in isolated environments first

## Threat Model

### In Scope

- Malicious or buggy tools attempting to escape sandbox
- Unauthorized access to artifacts or audit logs
- Policy bypass attempts
- Resource exhaustion attacks
- Data exfiltration through covert channels

### Out of Scope

- Physical access to the host system
- Kernel vulnerabilities
- Container runtime vulnerabilities (Docker/containerd)
- Denial of service at the network level

## Security Audits

We welcome security audits of Ravenforge. If you're conducting an audit:

1. Contact us beforehand to coordinate
2. Focus on the sandbox implementation and policy engine
3. Test in an isolated environment
4. Share your findings with us before publication

## Acknowledgments

We maintain a security hall of fame for responsible disclosures:

- (This section will list security researchers who have helped improve Ravenforge)
