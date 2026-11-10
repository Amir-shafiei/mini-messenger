# Session and transport security

Access JWTs expire after 15 minutes; login sessions have a fixed seven-day absolute expiry. Refresh cannot extend the absolute deadline. There is no idle timeout. Logout revokes the database session before closing its sockets; other login sessions remain independent.

Refresh secrets are 256-bit random values. Only SHA-256 digests are stored. Each refresh rotates the secret in a transaction, locking the session row first. Consumed digests remain available for reuse detection; replay revokes the session and disconnects its sockets. Unknown, expired and revoked credentials return a generic 401. A lost rotation response may require login again: security takes precedence over silently accepting a reused secret. Refresh requests are never blindly retried.

The browser uses HttpOnly, SameSite=Strict cookies. Production HTTPS uses `__Host-mm_session` and `__Host-mm_refresh`, with Secure, Path=/ and no Domain. Local HTTP uses `mm_session`/`mm_refresh`. Tokens are not returned in JSON or stored in browser localStorage. Browser Web Locks serialize refreshes across tabs; an in-tab promise merges concurrent renewals. A stale failed API response must not clear a newer access cookie.

## HTTPS deployment

The current local address is HTTP on loopback only. It is not an encrypted public deployment. Do not expose it with a tunnel or proxy and assume that the local configuration protects public traffic.

For public use, supply a certificate chain for your hostname and its private key, then configure:

```text
APP_ORIGIN=https://chat.example.com
HTTP_ADDR=0.0.0.0:8443
COOKIE_SECURE=true
TLS_CERT_FILE=/private/path/fullchain.pem
TLS_KEY_FILE=/private/path/privkey.pem
```

The port in APP_ORIGIN must match the browser URL (include :8443 when browsing that port). The application terminates TLS directly, with minimum TLS 1.2; it does not trust arbitrary X-Forwarded-* headers. Certificate issuance, renewal and private-key file permissions are deployment responsibilities. Never disable browser certificate checks or use a test certificate for public deployment. HSTS is sent only on actual TLS connections. Clients use WSS automatically for an HTTPS page.

HTTP configuration is rejected unless both the listener and origin are loopback. Remote PostgreSQL connections require `sslmode=verify-full` and an appropriate trusted CA, so encryption cannot silently omit server identity verification.

CSP restricts scripts and connections to the application, forbids framing/objects/base URL changes and restricts form submission. Inline style support is retained for the existing dynamic composer; inline script execution is not allowed. Referrer-Policy, nosniff and Permissions-Policy are also set. Exact Origin checks cover login, refresh, logout, message writes, read receipts and WebSocket handshakes.

TLS protects data in transit against network interception when certificates are validated. It does not protect against a compromised device, trusted malicious root CA, XSS or all application vulnerabilities. Do not describe this implementation as universal attack prevention.

## Verification

Unit/database tests cover token shape and hashing, absolute expiry, rotation, replay, concurrent reuse, logout, JWT validation, cookie attributes, HTTPS configuration restrictions and response headers. Browser tests cover renewal after access-cookie loss and one rotation across two tabs, plus replay closing both tabs' sockets. A separate local TLS server was tested with an explicit temporary test CA: HTTPS login/refresh, WSS, Host-prefixed cookies and HSTS passed; an untrusted certificate and TLS 1.1 were rejected. The test CA was not installed into the operating system.

References: [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html), [OWASP TLS](https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Security_Cheat_Sheet.html), [RFC 9700 refresh-token protection](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.14).
