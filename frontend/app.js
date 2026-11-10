import { initializeLoginScene } from "./login.js";
import { element, icon, toast } from "./ui.js";
import { renderContactList } from "./contacts.js";
import { renderMessageList, compareMessages, receiptCursor } from "./messages.js";
import { request, renewSession, withSessionLock } from "./api.js";

const state = {
  user: null,
  conversations: [],
  selectedUser: null,
  filter: "all",
  drafts: new Map(),
  messages: new Map(),
  historyCursor: null,
  conversationVersion: 0,
  socket: null,
  reconnectTimer: null,
  reconnectAttempts: 0,
  renewalTimer: null,
  sending: false,
  summaryTimer: null,
  summaryVersion: 0,
  typing: new Map(),
  typingTimer: null,
  typingSent: 0,
  typingStop: null,
  onlineUsers: null,
  peerReadReceipt: null,
  readAck: new Map(),
  savingReadReceipt: false,
  readTimer: null,
};

function showFieldError(id, message = "") {
  element(id).textContent = message;
  element(id).hidden = !message;
}
function showError(message) {
  showFieldError("app-error", message);
}
// Session and login lifecycle
function endSession(message = "") {
  element("app-view").classList.remove("chat-open");
  state.user = null;
  state.selectedUser = null;
  state.conversationVersion++;
  clearTimeout(state.reconnectTimer);
  clearTimeout(state.renewalTimer);
  clearTimeout(state.readTimer);
  clearTimeout(state.summaryTimer);
  clearTimeout(state.typingTimer);
  clearTimeout(state.typingStop);
  state.typing.clear();
  state.summaryVersion++;
  if (state.socket) {
    state.socket.onclose = null;
    state.socket.close();
    state.socket = null;
  }
  state.onlineUsers = null;
  state.conversations = [];
  state.readAck.clear();
  state.messages.clear();
  state.drafts.clear();
  closeEmoji();
  element("messages").replaceChildren();
  element("message").value = "";
  element("app-view").hidden = true;
  element("login-view").hidden = false;
  element("boot").hidden = true;
  element("login-error").textContent = message;
}
// Retry an authenticated request once, after one coordinated session renewal.
async function apiRequest(path, options = {}) {
  try {
    return await request(path, options);
  } catch (error) {
    if (error.status !== 401 || path === "/auth/login") throw error;
    try {
      const session = await renewSession();
      if (state.user && session.user.id !== state.user.id) {
        endSession("حساب این مرورگر عوض شده؛ دوباره وارد شو.");
        throw new Error("حساب این مرورگر عوض شده.");
      }
      scheduleRenewal(session.expires_at);
      return await request(path, options);
    } catch (renewalError) {
      if (renewalError.status === 401) endSession("ورودت تموم شده؛ دوباره وارد شو.");
      throw renewalError;
    }
  }
}

function scheduleRenewal(expiresAt) {
  clearTimeout(state.renewalTimer);
  const delay = Math.max(30000, new Date(expiresAt) - Date.now() - 60000);
  state.renewalTimer = setTimeout(renewCurrentSession, delay);
}

async function renewCurrentSession() {
  if (!state.user) return;
  const userID = state.user.id;
  try {
    const session = await renewSession();
    if (state.user?.id !== userID) return;
    if (session.user.id !== userID) {
      endSession("حساب این مرورگر عوض شده؛ دوباره وارد شو.");
      return;
    }
    scheduleRenewal(session.expires_at);
    showError("");
    // Reopen the socket using the fresh cookie before its old JWT expires.
    if (state.socket) {
      state.socket.onclose = null;
      state.socket.close();
    }
    connectRealtime();
  } catch (error) {
    if (!state.user) return;
    if (error.status === 401) {
      endSession("ورودت تموم شده؛ دوباره وارد شو.");
      return;
    }
    showError(error.message);
    state.renewalTimer = setTimeout(renewCurrentSession, 15000);
  }
}

// Conversation list and presence
function getPresenceText(id) {
  if ((state.typing.get(id) || 0) > Date.now()) return "در حال تایپ…";
  return state.onlineUsers === null
    ? "اتصالش مشخص نیست"
    : state.onlineUsers.has(id)
      ? "آنلاین"
      : "آفلاین";
}
function renderPresence() {
  if (state.selectedUser) {
    element("peer-status").textContent = getPresenceText(state.selectedUser.id);
    element("peer-status").classList.toggle(
      "is-online",
      !!state.onlineUsers?.has(state.selectedUser.id),
    );
    element("peer-status").classList.toggle(
      "is-typing",
      (state.typing.get(state.selectedUser.id) || 0) > Date.now(),
    );
  }
  for (const button of element("contacts").children) {
    button.classList.toggle("is-online", !!state.onlineUsers?.has(button.dataset.userId));
  }
}
function renderConversations() {
  renderContactList(state, selectConversation);
  renderPresence();
}
// Database snapshots avoid double counting retried or replayed events.
function scheduleConversations() {
  clearTimeout(state.summaryTimer);
  state.summaryTimer = setTimeout(loadConversations, 150);
}
async function loadConversations() {
  if (!state.user) return;
  const version = ++state.summaryVersion,
    userID = state.user.id;
  element("refresh-users").disabled = true;
  try {
    const result = await apiRequest("/conversations");
    if (state.user?.id !== userID || version !== state.summaryVersion) return;
    state.conversations = result.conversations;
    renderConversations();
    showFieldError("users-error");
    element("refresh-users").hidden = true;
  } catch (error) {
    if (state.user?.id === userID && version === state.summaryVersion) {
      showFieldError("users-error", error.message);
      element("refresh-users").hidden = false;
    }
  } finally {
    if (version === state.summaryVersion) element("refresh-users").disabled = false;
  }
}

function applyReceipt(receipt) {
  if (
    !receipt ||
    !state.selectedUser ||
    receipt.reader_id !== state.selectedUser.id ||
    receipt.peer_id !== state.user?.id
  )
    return;
  if (
    !state.peerReadReceipt ||
    compareMessages(receiptCursor(receipt), receiptCursor(state.peerReadReceipt)) > 0
  )
    state.peerReadReceipt = receipt;
}
function renderMessages(scroll = false) {
  renderMessageList(state, scroll);
  updateJump();
  scheduleRead();
}
async function loadPeerRead() {
  if (!state.selectedUser) return;
  const version = state.conversationVersion;
  try {
    const result = await apiRequest("/users/" + state.selectedUser.id + "/read");
    if (version === state.conversationVersion && state.user) {
      applyReceipt(result.receipt);
      renderMessages();
    }
  } catch (error) {
    if (version === state.conversationVersion)
      showFieldError("read-error", "وضعیت خوندن رو نگرفتیم؛ دوباره که وصل بشیم می‌گیریمش.");
  }
}
async function loadHistory(older = false, resync = false) {
  if (!state.selectedUser) return;
  const version = state.conversationVersion,
    peer = state.selectedUser.id,
    known = new Set(state.messages.keys());
  let cursor = older ? state.historyCursor : null;
  const oldHeight = element("chat-area").scrollHeight,
    oldTop = element("chat-area").scrollTop;
  element("history-status").textContent = "داریم پیام‌ها رو می‌گیریم…";
  element("older").disabled = true;
  element("history-retry").hidden = true;
  showFieldError("history-error");
  try {
    do {
      const page = await apiRequest(
        "/users/" +
          peer +
          "/messages?limit=50" +
          (cursor ? "&before=" + encodeURIComponent(cursor) : ""),
      );
      if (version !== state.conversationVersion || !state.user) return;
      const reached = page.messages.some((m) => known.has(m.id));
      for (const message of page.messages) state.messages.set(message.id, message);
      if (older || !resync || known.size === 0) state.historyCursor = page.next_before;
      cursor = page.next_before;
      if (!resync || reached) break;
    } while (cursor);
    renderMessages(!older && !resync);
    if (older)
      element("chat-area").scrollTop = oldTop + element("chat-area").scrollHeight - oldHeight;
    element("older").hidden = !state.historyCursor;
  } catch (error) {
    if (version === state.conversationVersion) {
      showFieldError("history-error", error.message);
      element("history-retry").hidden = false;
      element("history-retry").onclick = () => loadHistory(older, resync);
    }
  } finally {
    if (version === state.conversationVersion) {
      element("history-status").textContent = "";
      element("older").disabled = false;
    }
  }
}
function selectConversation(user) {
  if (state.sending) return;
  element("app-view").classList.add("chat-open");
  showError("");
  if (state.selectedUser && !pendingAttempt(state.selectedUser.id))
    state.drafts.set(state.selectedUser.id, element("message").value);
  closeEmoji();
  stopTyping();
  state.selectedUser = user;
  state.conversationVersion++;
  state.messages.clear();
  state.historyCursor = null;
  state.peerReadReceipt = null;
  element("message").value = pendingAttempt(user.id)?.text || state.drafts.get(user.id) || "";
  element("peer-avatar").textContent = user.username[0];
  updateComposer();
  element("peer-name").textContent = user.username;
  element("empty").hidden = true;
  element("chat-area").hidden = false;
  element("send-form").hidden = false;
  element("older").hidden = true;
  showFieldError("send-error");
  showFieldError("read-error");
  renderConversations();
  renderMessages();
  loadHistory();
  loadPeerRead();
}

// Read only incoming bubbles visible in a focused, visible conversation.
// Read receipts
function scheduleRead() {
  clearTimeout(state.readTimer);
  state.readTimer = setTimeout(markVisibleRead, 250);
}
async function markVisibleRead() {
  if (
    state.savingReadReceipt ||
    !state.user ||
    !state.selectedUser ||
    document.visibilityState !== "visible" ||
    !document.hasFocus()
  )
    return;
  if (!element("app-view").classList.contains("chat-open")) return;
  const area = element("chat-area").getBoundingClientRect();
  if (!area.height) return;
  const visible = [...element("messages").querySelectorAll(".message:not(.mine)")].filter(
    (node) => {
      const rect = node.getBoundingClientRect();
      return rect.top < area.bottom && rect.bottom > area.top;
    },
  );
  const last = visible.at(-1),
    message = last && state.messages.get(last.dataset.messageId);
  if (!message) return;
  const peer = state.selectedUser.id,
    acknowledged = state.readAck.get(peer);
  if (acknowledged && compareMessages(message, acknowledged) <= 0) return;
  const version = state.conversationVersion;
  state.savingReadReceipt = true;
  let saved = false;
  try {
    const result = await apiRequest("/users/" + peer + "/read", {
      method: "POST",
      body: JSON.stringify({ message_id: message.id }),
    });
    if (state.user && version === state.conversationVersion && result.receipt) {
      state.readAck.set(peer, receiptCursor(result.receipt));
      showFieldError("read-error");
      saved = true;
      scheduleConversations();
    }
  } catch {
    if (version === state.conversationVersion)
      showFieldError("read-error", "ثبت نشد که پیام رو خوندی؛ روی گپ بزن تا دوباره امتحان کنیم.");
  } finally {
    state.savingReadReceipt = false;
    // A newer message may have arrived while this acknowledgement was in flight.
    if (state.user && (saved || version !== state.conversationVersion)) scheduleRead();
  }
}
// Realtime connection and event handling
function connectRealtime() {
  if (!state.user) return;
  clearTimeout(state.reconnectTimer);
  const socket = new WebSocket(
    (location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/api/v1/ws",
  );
  state.socket = socket;
  element("connection").textContent = "داریم وصل می‌شیم…";
  element("connection").classList.remove("online");
  socket.onopen = () => {
    if (state.socket !== socket) return;
    state.reconnectAttempts = 0;
    element("connection").textContent = "وصل شدیم";
    element("connection").classList.add("online");
    loadHistory(false, true);
    loadPeerRead();
    loadConversations();
  };
  socket.onmessage = (event) => {
    if (state.socket !== socket || !state.user) return;
    let data;
    try {
      data = JSON.parse(event.data);
    } catch {
      return;
    }
    if (data.type === "presence.snapshot") {
      state.onlineUsers = new Set(data.user_ids);
      renderPresence();
      return;
    }
    if (data.type === "typing") {
      if (data.active) state.typing.set(data.sender_id, Date.now() + 4500);
      else state.typing.delete(data.sender_id);
      renderPresence();
      return;
    }
    if (data.type === "message.read") {
      applyReceipt(data.receipt);
      renderMessages();
      scheduleConversations();
      return;
    }
    const message = data.message;
    if (data.type !== "message.created" || !message) return;
    scheduleConversations();
    state.typing.delete(message.sender_id);
    renderPresence();
    if (!state.selectedUser) return;
    if (
      (message.sender_id === state.user.id && message.recipient_id === state.selectedUser.id) ||
      (message.recipient_id === state.user.id && message.sender_id === state.selectedUser.id)
    ) {
      state.messages.set(message.id, message);
      renderMessages();
    }
  };
  socket.onclose = () => {
    if (state.socket !== socket || !state.user) return;
    state.onlineUsers = null;
    state.typing.clear();
    renderPresence();
    element("connection").textContent = "قطع شد؛ داریم وصل می‌شیم…";
    element("connection").classList.remove("online");
    state.reconnectTimer = setTimeout(
      async () => {
        try {
          await apiRequest("/auth/me");
          if (state.user) connectRealtime();
        } catch (error) {
          if (state.user) {
            showError(error.message);
            socket.onclose();
          }
        }
      },
      Math.min(1000 * 2 ** state.reconnectAttempts++, 15000),
    );
  };
  socket.onerror = () => socket.close();
}
async function showMessenger(data) {
  element("app-view").classList.remove("chat-open");
  state.user = data.user;
  element("search").value = "";
  setFilter("all");
  element("boot").hidden = true;
  element("login-view").hidden = true;
  element("app-view").hidden = false;
  element("my-name").textContent = data.user.username;
  element("my-avatar").textContent = data.user.username[0];
  element("empty").hidden = false;
  element("chat-area").hidden = true;
  element("send-form").hidden = true;
  element("peer-name").textContent = "گپ تازه‌ای شروع کنیم؟";
  element("peer-status").textContent = "";
  showError("");
  clearTimeout(state.renewalTimer);
  scheduleRenewal(data.expires_at);
  await loadConversations();
  connectRealtime();
  clearInterval(state.typingTimer);
  state.typingTimer = setInterval(renderPresence, 500);
}
// These are the same public, local demo credentials shown on the login page.
for (const button of document.querySelectorAll("[data-demo-user]")) {
  button.onclick = () => {
    element("username").value = button.dataset.demoUser;
    element("password").value = "LocalDemo-2026!";
    element("login-error").textContent = "";
    element("demo-status").textContent = "حساب " + button.dataset.demoUser + " آماده‌ست؛ بزن بریم!";
    element("login-button").focus();
  };
}
async function handleLogin(event) {
  event.preventDefault();
  element("login-button").disabled = true;
  element("login-error").textContent = "";
  try {
    await showMessenger(
      await withSessionLock(() =>
        request("/auth/login", {
          method: "POST",
          body: JSON.stringify({
            username: element("username").value,
            password: element("password").value,
          }),
        }),
      ),
    );
    element("password").value = "";
  } catch (error) {
    element("login-error").textContent = error.message;
  } finally {
    element("login-button").disabled = false;
  }
}
element("login-form").onsubmit = handleLogin;
async function handleLogout() {
  element("logout").disabled = true;
  try {
    await apiRequest("/auth/logout", { method: "POST" });
    endSession();
  } catch (error) {
    showError(error.message);
  } finally {
    element("logout").disabled = false;
  }
}
element("logout").onclick = handleLogout;
// Drafts, typing and message submission
function updateComposer() {
  const field = element("message"),
    length = [...field.value].length;
  field.style.height = "auto";
  field.style.height = Math.min(field.scrollHeight, 160) + "px";
  element("count").textContent = length
    ? length.toLocaleString("fa-IR") + " / ۴۰۰۰"
    : "Enter برای ارسال · Shift + Enter خط بعد";
  element("count").classList.toggle("over-limit", length > 4000);
  const pending = state.selectedUser && pendingAttempt(state.selectedUser.id);
  field.readOnly = state.sending || !!pending;
  element("send").textContent = state.sending ? "داره می‌ره…" : pending ? "دوباره بفرست" : "بفرست";
  element("send").disabled = state.sending || !field.value.trim() || length > 4000;
}
function pendingKey(peer) {
  return "mm.pending." + state.user?.id + "." + peer;
}
function pendingAttempt(peer) {
  try {
    return JSON.parse(sessionStorage.getItem(pendingKey(peer)));
  } catch {
    return null;
  }
}
function stopTyping() {
  clearTimeout(state.typingStop);
  if (state.selectedUser && state.socket?.readyState === WebSocket.OPEN)
    state.socket.send(
      JSON.stringify({ type: "typing", recipient_id: state.selectedUser.id, active: false }),
    );
  state.typingSent = 0;
}
function announceTyping() {
  if (!state.selectedUser || state.socket?.readyState !== WebSocket.OPEN) return;
  if (!element("message").value.trim()) {
    stopTyping();
    return;
  }
  if (Date.now() - state.typingSent > 1000) {
    state.socket.send(
      JSON.stringify({ type: "typing", recipient_id: state.selectedUser.id, active: true }),
    );
    state.typingSent = Date.now();
  }
  clearTimeout(state.typingStop);
  state.typingStop = setTimeout(stopTyping, 2500);
}
element("message").oninput = () => {
  if (state.selectedUser) state.drafts.set(state.selectedUser.id, element("message").value);
  updateComposer();
  announceTyping();
};
element("message").addEventListener("blur", stopTyping);
element("message").onkeydown = (event) => {
  if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    if (!element("send").disabled) element("send-form").requestSubmit();
  }
};
async function handleSendMessage(event) {
  event.preventDefault();
  if (
    state.sending ||
    !state.selectedUser ||
    !element("message").value.trim() ||
    [...element("message").value].length > 4000
  )
    return;
  const peer = state.selectedUser.id,
    version = state.conversationVersion;
  const storageKey = pendingKey(peer);
  let attempt;
  try {
    attempt = pendingAttempt(peer) || { key: crypto.randomUUID(), text: element("message").value };
    // Store before sending: reload and lost responses must reuse exactly this key and body.
    sessionStorage.setItem(storageKey, JSON.stringify(attempt));
  } catch {
    showFieldError(
      "send-error",
      "مرورگر اجازه نمی‌ده ارسال رو نگه داریم. دسترسی به حافظهٔ مرورگر رو فعال کن.",
    );
    return;
  }
  stopTyping();
  state.sending = true;
  updateComposer();
  showFieldError("send-error");
  try {
    const result = await apiRequest("/users/" + peer + "/messages", {
      method: "POST",
      headers: { "Idempotency-Key": attempt.key },
      body: JSON.stringify({ text: attempt.text }),
    });
    sessionStorage.removeItem(storageKey);
    if (version === state.conversationVersion && state.user) {
      state.messages.set(result.message.id, result.message);
      state.drafts.delete(peer);
      element("message").value = "";
      renderMessages(true);
      scheduleConversations();
    }
  } catch (error) {
    const rejected = [400, 404, 409, 413, 415].includes(error.status);
    if (rejected) sessionStorage.removeItem(storageKey);
    if (version === state.conversationVersion)
      showFieldError(
        "send-error",
        error.message +
          (error.code === "invalid_idempotency_key"
            ? ""
            : rejected
              ? " متن رو درست کن و دوباره بفرست."
              : " متنت توی همین تب مونده؛ «دوباره بفرست» رو بزن. پیام تکراری نمی‌شه."),
      );
  } finally {
    state.sending = false;
    updateComposer();
    element("message").focus();
  }
}
element("send-form").onsubmit = handleSendMessage;
element("back-to-contacts").onclick = () => {
  stopTyping();
  element("app-view").classList.remove("chat-open");
  element("search").focus();
};
element("older").onclick = () => loadHistory(true);
element("search").oninput = renderConversations;
element("refresh-users").onclick = loadConversations;
element("chat-area").addEventListener("scroll", () => {
  updateJump();
  scheduleRead();
});
element("chat-area").addEventListener("click", scheduleRead);
window.addEventListener("focus", () => {
  scheduleRead();
  if (state.user) {
    scheduleConversations();
    loadHistory(false, true);
    loadPeerRead();
  }
});
window.addEventListener("blur", stopTyping);
document.addEventListener("visibilitychange", () => {
  scheduleRead();
  if (document.hidden) stopTyping();
});
window.addEventListener("online", () => {
  if (state.user && state.socket?.readyState !== WebSocket.OPEN) state.socket?.close();
});
for (const node of document.querySelectorAll("[data-icon]"))
  node.innerHTML = icon(node.dataset.icon);
// Small interface interactions
function setFilter(value) {
  state.filter = value;
  element("filter-all").setAttribute("aria-pressed", String(value === "all"));
  element("filter-unread").setAttribute("aria-pressed", String(value === "unread"));
  renderConversations();
}
element("filter-all").onclick = () => setFilter("all");
element("filter-unread").onclick = () => setFilter("unread");
element("clear-search").onclick = () => {
  element("search").value = "";
  renderConversations();
  element("search").focus();
};
element("start-chat").onclick = () => {
  element("search").focus();
};
element("show-password").onclick = () => {
  const show = element("password").type === "password";
  element("password").type = show ? "text" : "password";
  element("show-password").textContent = show ? "پنهان" : "نمایش";
  element("show-password").setAttribute("aria-pressed", String(show));
};
function updateJump() {
  const area = element("chat-area");
  element("jump-latest").hidden =
    !state.selectedUser || area.scrollHeight - area.scrollTop - area.clientHeight < 160;
}
element("jump-latest").onclick = () => {
  element("chat-area").scrollTo({
    top: element("chat-area").scrollHeight,
    behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth",
  });
};
function closeEmoji() {
  element("emoji-picker").hidden = true;
  element("emoji-toggle").setAttribute("aria-expanded", "false");
}
element("emoji-toggle").onclick = () => {
  const open = element("emoji-picker").hidden;
  element("emoji-picker").hidden = !open;
  element("emoji-toggle").setAttribute("aria-expanded", String(open));
  if (open) element("emoji-picker").querySelector("button").focus();
};
for (const [emoji, label] of [
  ["😊", "لبخند"],
  ["😂", "خنده"],
  ["❤️", "قلب"],
  ["👍", "عالی"],
  ["👋", "سلام"],
  ["🙏", "ممنون"],
  ["🎉", "جشن"],
  ["☕", "قهوه"],
]) {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = emoji;
  button.setAttribute("aria-label", label);
  button.onclick = () => {
    const field = element("message");
    if (!field.readOnly) {
      const at = field.selectionStart;
      field.setRangeText(emoji, at, field.selectionEnd, "end");
      field.dispatchEvent(new Event("input"));
    }
    closeEmoji();
    field.focus();
  };
  element("emoji-picker").append(button);
}
document.addEventListener("click", (event) => {
  if (!event.target.closest("#emoji-picker, #emoji-toggle")) closeEmoji();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !element("emoji-picker").hidden) {
    closeEmoji();
    element("emoji-toggle").focus();
  }
});
apiRequest("/auth/me")
  .then(showMessenger)
  .catch((error) => {
    if (state.user) showError(error.message);
    else endSession(error.status === 401 ? "" : error.message);
  });

initializeLoginScene();
