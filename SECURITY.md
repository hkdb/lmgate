# Security Policy

## Supported Versions

Only the latest release of LM Gate receives security updates. We recommend always running the most recent version.

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
| Older   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in LM Gate, please report it responsibly. **Do not open a public GitHub issue for security vulnerabilities.**

Instead, email us at **lmgate@3df.io** with:

- A description of the vulnerability
- Steps to reproduce the issue
- The potential impact
- Any suggested mitigations, if applicable

## What to Expect

- **Acknowledgment** within 3 business days of your report
- **Status update** within 10 business days with an initial assessment
- **Coordinated disclosure** — we will work with you on an appropriate timeline before any public disclosure

## Scope

The following are in scope for security reports:

- Authentication and authorization bypasses
- Injection vulnerabilities (SQL, command, XSS, etc.)
- Sensitive data exposure (credentials, tokens, keys)
- TLS/cryptographic weaknesses
- Privilege escalation
- Denial of service via application-level flaws

The following are **out of scope**:

- Vulnerabilities in upstream dependencies with no demonstrated impact on LM Gate
- Social engineering or phishing attacks
- Denial of service via volumetric/network-level attacks
- Issues in environments running unsupported or heavily modified versions

## Recognition

We appreciate the security research community's efforts in helping keep LM Gate secure. With your permission, we will acknowledge your contribution in the release notes for any fix resulting from your report.
