# Frontend: a friendly Persian messenger

Plain HTML, CSS and JavaScript; local Vazir regular/medium/bold fonts. No new runtime dependency.

## Design decisions
The interface uses minimal layouts, small interaction feedback, brand-colored message bubbles, and transient typing status. RTL behavior is explicit for contact order, navigation, message alignment, and copy actions. Vazir is bundled locally so the interface does not depend on a font CDN.

Tokens: paper #FFFFFF, canvas #EDF0F7, ink #202B45, blue #3854C8, muted #626D83, line #E2E6EF. RTL sidebar on desktop; a single task per screen on mobile. 700px navigation breakpoint. CSS sections group entry screen, workspace, messages, composer and responsive states. Motion is limited to feedback/typing and honors reduced motion.

## Interactions
- Search with a clear control; all/unread filters with pressed states and contextual unread announcements.
- Contact timestamps, last-message preview, online dots and persisted unread counts.
- Header avatar and typing status; connection status remains distinct from peer presence.
- Per-peer drafts survive switching contacts during this page session. Ambiguous send attempts use sessionStorage and their original idempotency key across reload.
- Small emoji picker with named buttons, Escape/outside-click dismissal and focus restoration.
- Copy-message action available by keyboard/hover and visible on touch layouts; contextual toast.
- Jump-to-latest button when reading older messages; resync does not unconditionally jump to the bottom.
- Password visibility toggle; password-manager/paste support unchanged.

Copy is conversational Persian rather than formal. Errors stay specific and preserve unsent text. Request IDs remain available in HTTP headers and server logs without being exposed in ordinary user-facing errors. All message text and previews use textContent; SVG markup is fixed local icon data, never user input.

## Deployment consistency
index.html links versioned script/style URLs. The local server sends Cache-Control: no-store for frontend resources, including unversioned URLs. An already-running old tab still needs one reload; copy an unsent draft first if it predates draft recovery. Missing/malformed Idempotency-Key now gets invalid_idempotency_key rather than a misleading message-text validation error. No fallback silently disables duplicate protection.

## Verified
Go tests include frontend cache policy and explicit missing-key errors. Browser checks cover keyed sending, lost response after commit, reload/retry deduplication, typing expiry, unread summaries, read receipts, draft switching, emoji/Escape, clipboard, search/clear, filters, 375px and landscape layouts, reduced motion and no JavaScript errors. Desktop login/chat and mobile chat screenshots were visually inspected. Tests use isolated temporary users and clean their data afterward.

## Readability and session renewal
The frontend now uses native ES modules: app.js owns state and coordination, api.js owns HTTP and refresh locking, contacts.js/messages.js render supplied data, and ui.js holds small UI helpers. No framework or runtime package was added. Formatting and descriptive state names make the flow easier to follow. The old forced 15-minute logout timer is replaced by coordinated renewal and socket reconnection.
