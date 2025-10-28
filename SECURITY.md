# Security Policy

## Supported Versions

Use this section to tell people about which versions of your project are currently being supported with security updates.

| Version | Supported          |
| ------- | ------------------ |
| 1.1.x   | :white_check_mark: |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

The nqjson team takes security bugs seriously. We appreciate your efforts to responsibly disclose your findings, and will make every effort to acknowledge your contributions.

### How to Report a Security Vulnerability

If you believe you have found a security vulnerability in nqjson, please report it to us through coordinated disclosure.

**Please do not report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

Instead, please send an email to: **security@[your-domain].com**

Please include as much of the information listed below as you can to help us better understand and resolve the issue:

- Type of issue (e.g. buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit the issue

This information will help us triage your report more quickly.

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 2 business days.
- **Investigation**: We will investigate the issue and determine its validity and severity.
- **Updates**: We will provide updates on our progress every 7 days until the issue is resolved.
- **Resolution**: We aim to resolve critical security issues within 30 days of the initial report.

### Security Measures in nqjson

The nqjson library implements several security measures:

1. **Memory Safety**: 
   - Proper bounds checking for all array/slice operations
   - Safe use of unsafe operations with validation
   - No buffer overflows or memory leaks

2. **Input Validation**:
   - Comprehensive JSON validation
   - Path syntax validation
   - Malformed input rejection

3. **Dependency Security**:
   - Zero external dependencies
   - Only uses Go standard library
   - Regular vulnerability scanning with govulncheck

4. **Code Quality**:
   - Comprehensive linting with golangci-lint
   - Static analysis with staticcheck
   - Security scanning with gosec
   - Race condition detection

### Security Best Practices for Users

When using nqjson in your applications:

1. **Input Validation**: Always validate JSON input from untrusted sources
2. **Error Handling**: Check and handle all errors returned by nqjson functions
3. **Resource Limits**: Implement appropriate limits for JSON size and nesting depth
4. **Regular Updates**: Keep nqjson updated to the latest version

### Vulnerability Disclosure Timeline

- **Day 0**: Vulnerability reported
- **Day 2**: Acknowledgment sent to reporter
- **Day 7**: Initial assessment completed
- **Day 14**: Fix developed and tested
- **Day 21**: Security release prepared
- **Day 30**: Public disclosure (coordinated with reporter)

### Security Hall of Fame

We recognize security researchers who help make nqjson safer:

(List will be updated as vulnerabilities are reported and fixed)

### Contact

For any security-related questions or concerns, please contact:
- Email: dhawalhost@gmail.com

---

Thank you for helping keep nqjson and our users safe!
