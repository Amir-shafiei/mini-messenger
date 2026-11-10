# HTTP and realtime contract — v1

Status: implemented local API contract. Base path /api/v1. JSON uses snake_case; UUIDs are strings, timestamps UTC RFC3339 with fractional seconds allowed. Browser and API share an origin. No signup in v1.

## Routes
| Method | Path | Auth | Input | Success |
|---|---|---|---|---|
| POST | /auth/login | no | username, password | 200 user and expires_at; session cookie |
| POST | /auth/refresh | refresh cookie | empty body | 200 user and access expires_at; rotate both cookies |
| GET | /auth/me | yes | none | 200 user and expires_at |
| POST | /auth/logout | yes | empty body | 204; clear cookie and close session sockets |
| GET | /users | yes | none | 200 users array excluding self |
| POST | /users/{user_id}/messages | yes | text + UUID Idempotency-Key header | 201 message |
| GET | /users/{user_id}/messages | yes | before optional UUID; limit default 50, range 1-100 | 200 messages and next_before |
| GET | /ws | yes | WebSocket upgrade | 101 |

Paths in the table are relative to the base path. /auth/me returns the current public user profile and session expiry.

## JSON examples
Login request: `{"username":"alice","password":"..."}`.
Login/me response: `{"user":{"id":"<uuid>","username":"alice"},"expires_at":"2026-11-08T12:15:00Z"}`. Token is never returned in JSON.
Users response: `{"users":[{"id":"<uuid>","username":"bob"}]}`.
Send request: `{"text":"Hello"}`. Recipient comes from the path; sender always comes from authentication.
Send response: `{"message":{"id":"<uuid>","sender_id":"<uuid>","recipient_id":"<uuid>","text":"Hello","created_at":"2026-11-08T12:00:00Z"}}`.
History response: `{"messages":[],"next_before":null}`. Messages are newest first; next_before is the last returned message UUID for a full page, otherwise null. The UI reverses display ordering as needed. JSON lists are empty arrays, never null.

## Authentication and request protection
Use HS256 via an established JWT library, pin the algorithm, and require sub, sid, exp, iat, iss=mini-messenger and aud=mini-messenger-web. No roles or permissions are derived from arbitrary claims. Access JWT TTL is 15 minutes; sessions have a fixed seven-day lifetime and rotating refresh tokens. Validate nbf when present. Require a configured cryptographically random key of at least 32 bytes; no hardcoded fallback. JWT signing-key rotation invalidates access JWTs; refresh sessions remain usable unless explicitly revoked. Reject unexpected signing algorithms and key URLs; never fetch keys from token headers.

Local cookie name mm_session (HTTPS: __Host-mm_session): HttpOnly, SameSite=Strict, Path=/, no Domain, Max-Age aligned with remaining access-token life. Secure=true outside explicit local HTTP development. Logout clears with identical scope. Mutating browser requests must have the exact configured Origin, including login/refresh/logout; missing or mismatched Origin is rejected. Login/send require application/json. WebSocket upgrade also requires exact Origin. Do not enable wildcard credentialed CORS. Command-line clients must send Origin explicitly. Auth responses use Cache-Control: no-store.

Validate signature/claims then the live session for each protected request. Session user must match sub. Return generic invalid_credentials for unknown user and incorrect password, and perform a dummy password comparison for missing users in the implementation. Rate-limit login attempts before expensive password work. Never log passwords, cookies, JWTs or message bodies.

## Input and error contract
Reject unknown JSON fields, trailing JSON values and bodies above 32 KiB. Username is normalized before login. Password input is capped at 72 UTF-8 bytes because the implemented bcrypt adapter rejects longer input; passwords are never truncated. bcrypt cost is 12. Message text is valid UTF-8, not whitespace-only, at most 4000 Unicode code points and contains no NUL; preserve accepted text exactly. UUID path/cursor syntax and integer limits are checked before use cases. Self-chat returns 400.

Error envelope: `{"error":{"code":"invalid_input","message":"Bad Request","request_id":"<server-generated-id>"}}`. The same ID is returned in X-Request-ID.
- 400 invalid_input: malformed request, invalid message/page/cursor; cursor from another conversation is indistinguishable from a nonexistent one.
- 401 invalid_credentials on login; unauthenticated for missing/invalid/revoked/expired session. A repeat logout after revocation returns 401. Ordinary failed API requests do not clear access cookies, avoiding races with renewal.
- 403 forbidden_origin: failed Origin check.
- 404 not_found: missing recipient or route.
- 405 method_not_allowed; 413 payload_too_large; 415 unsupported_media_type.
- 429 rate_limited, with Retry-After.
- 500 internal_error: generic response; diagnostic details go to server logs.

A committed message returns success even if realtime publication fails. Send requires a UUID Idempotency-Key. Retry an ambiguous request with exactly the same key, text and recipient: it returns the original message and timestamp (201). Reusing a sender/key with different text or recipient returns 409 idempotency_conflict. Keys are retained with their messages; retries may repeat message.created with the same message ID.

## WebSocket lifecycle
Persisted-message server event: `{"type":"message.created","message":{...same fields as HTTP...}}`. Client-to-server messages are not used for sending chat; use POST. Maintain one serialized writer per connection, bounded output queues, ping/pong deadlines and limits on incoming frames. A slow client is disconnected and recovers through history.

Deliver to all active tabs of both sender and recipient. Deduplicate on message.id because the HTTP response and event may arrive in either order. Register the socket, then recheck the session to close the handshake/logout race. If recheck fails, unregister and close. Close all sockets of a revoked session and enforce the earlier token/session expiry. Logout does not revoke other devices' sessions. No promise to retract frames already sent before revocation. During shutdown close sockets and drain the HTTP server.

## Acceptance checks for implementation
Login and me never return password hashes or token JSON. Reject tampered/unsigned/wrong-algorithm tokens, missing expiry, wrong issuer/audience and expired/revoked/mismatched sessions. Check cookie attributes, Origin rejection, JSON limits and rate limiting. Verify logout closes every tab for that session while other sessions remain active. Test real HTTP + PostgreSQL + WebSocket with three users, offline history and reconnection. Automated core, HTTP, realtime and opt-in PostgreSQL integration tests accompany the implementation.

## Implemented status extensions
GET /api/v1/users/{user_id}/read returns {"receipt":null} or the peer's persisted read cursor. POST to the same path accepts {"message_id":"<uuid>"} and returns the caller's updated cursor. Both require authentication; POST also requires exact Origin. A cursor references an incoming message from the path peer; unrelated or outgoing message IDs return 400. Older valid IDs return the existing newer cursor.

WebSocket now also sends presence.snapshot with user_ids (online users) and message.read with receipt {reader_id,peer_id,message_id,message_created_at}. Presence snapshots go to authenticated active sockets; receipts only to the two participants. On reconnect the client fetches /read as well as history. Offline status may take the heartbeat timeout to detect after abrupt connection loss.

## Conversation summaries
GET /api/v1/conversations (authenticated): {"conversations":[{"id":"<peer UUID>","username":"bob","last_message":null,"unread_count":0}]}. last_message uses the same public shape as history. Users with a last message sort by created_at and message ID descending, then empty conversations by username. Counts include incoming messages strictly newer than the caller's persisted read cursor. No actor ID is accepted from the client.

## Typing hints
Client sends {"type":"typing","recipient_id":"<uuid>","active":true} or false through the existing authenticated socket. The recipient's tabs receive {"type":"typing","sender_id":"<authenticated user>","active":true}. The payload contains no draft text. Unknown event types/fields, invalid JSON or frames above 1 KiB close the socket. Hints are throttled and transient; recipients clear them after 4.5 seconds without renewal. A stop hint may be suppressed by throttling, so clients must always apply expiry. Socket expiry/revocation stops further publication. Other participants do not receive the hint.

## Diagnostics
HTTP request IDs are generated by the server, never trusted from the client. JSON failures include request_id; X-Request-ID applies to HTTP responses. Unexpected errors are correlated with safe structured logs. WebSocket handshake errors use HTTP status/text rather than the JSON API envelope.

## Refresh endpoint
POST /api/v1/auth/refresh requires exact Origin and an empty body. It uses the HttpOnly refresh cookie, independently of the access JWT. Refresh response shape matches login/me; no token is returned in JSON. Refresh cookie names: mm_refresh locally, __Host-mm_refresh on HTTPS; both use Path=/, HttpOnly and SameSite=Strict. Secure is mandatory on HTTPS. Rotation consumes the old token and stores the next hash atomically. Reusing a consumed token revokes that session; expired/revoked/unknown tokens return 401. Logout revokes the session and clears both cookies. A seven-day absolute deadline is never extended.
