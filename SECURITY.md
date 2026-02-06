# Security Policy

## Supported Versions

We provide security updates for the following versions of Cilo:

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| < 0.2.0 | :x:                |

## Reporting a Vulnerability

We take the security of Cilo seriously. If you believe you have found a security vulnerability, please do not open a public issue. Instead, please report it following these steps:

1.  **Send an email** to security@cilo.dev (placeholder - please use GitHub private reporting if available).
2.  **Include a detailed description** of the vulnerability, including steps to reproduce.
3.  **Provide any relevant logs or screenshots**.

We will acknowledge receipt of your report within 48 hours and provide a timeline for resolution.

## Sudo and Privilege Escalation

Cilo requires `sudo` privileges during `init` to configure system DNS. We strive to minimize the use of elevated privileges:
- Sudo is only used for system-level configuration.
- Cilo drops privileges and fixes file ownership for the `~/.cilo` directory immediately after setup.
- Runtime operations (`up`, `down`, `create`) run as the local user.
