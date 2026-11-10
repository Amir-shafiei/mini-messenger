import { element, icon, toast } from "./ui.js";

// Preserve database timestamp precision when ordering equal-millisecond messages.
function sortableTimestamp(value) {
  return value.replace(/(?:\.(\d+))?Z$/, (_, part = "") => "." + part.padEnd(9, "0") + "Z");
}
export function compareMessages(a, b) {
  return (
    sortableTimestamp(a.created_at).localeCompare(sortableTimestamp(b.created_at)) ||
    a.id.localeCompare(b.id)
  );
}
export function receiptCursor(receipt) {
  return { id: receipt.message_id, created_at: receipt.message_created_at };
}
function isSeen(message, peerReadReceipt) {
  return peerReadReceipt && compareMessages(message, receiptCursor(peerReadReceipt)) <= 0;
}
export function renderMessageList({ messages, user, peerReadReceipt }, scroll = false) {
  const area = element("chat-area"),
    oldTop = area.scrollTop;
  const nearBottom = area.scrollHeight - oldTop - area.clientHeight < 100;
  const sorted = [...messages.values()].sort(compareMessages);
  element("messages").replaceChildren();
  let previous = null,
    previousDay = "";
  for (const message of sorted) {
    const date = new Date(message.created_at),
      day = date.toLocaleDateString("fa-IR");
    if (day !== previousDay) {
      const divider = document.createElement("div");
      divider.className = "day-divider";
      divider.textContent = date.toLocaleDateString("fa-IR", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
      element("messages").append(divider);
    }
    const mine = message.sender_id === user?.id;
    const grouped =
      previous &&
      previous.sender_id === message.sender_id &&
      day === previousDay &&
      date - new Date(previous.created_at) < 5 * 60 * 1000;
    const bubble = document.createElement("article");
    bubble.className = "message" + (mine ? " mine" : "") + (grouped ? " grouped" : "");
    bubble.dataset.messageId = message.id;
    const text = document.createElement("p");
    text.dir = "auto";
    text.textContent = message.text;
    const meta = document.createElement("div");
    meta.className = "message-meta";
    const time = document.createElement("time");
    time.dateTime = message.created_at;
    time.textContent = date.toLocaleTimeString("fa-IR", { hour: "2-digit", minute: "2-digit" });
    meta.append(time);
    if (mine) {
      const seen = !!isSeen(message, peerReadReceipt),
        tick = document.createElement("span");
      tick.className = "receipt" + (seen ? " seen" : "");
      tick.textContent = seen ? "✓✓" : "✓";
      tick.setAttribute("aria-label", seen ? "خونده شده" : "ذخیره شده");
      tick.title = seen ? "خونده شده" : "ذخیره شده";
      meta.append(tick);
    }
    const copy = document.createElement("button");
    copy.type = "button";
    copy.className = "copy-message";
    copy.title = "کپی پیام";
    copy.setAttribute("aria-label", "کپی پیام");
    copy.innerHTML = icon("copy");
    copy.onclick = async () => {
      try {
        await navigator.clipboard.writeText(message.text);
        toast("کپی شد!");
      } catch {
        toast("کپی نشد؛ متن رو انتخاب کن و کپی بگیر.");
      }
    };
    bubble.append(copy, text, meta);
    element("messages").append(bubble);
    previous = message;
    previousDay = day;
  }
  if (!sorted.length) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = "اینجا هنوز خبری نیست. اولین سلام با تو!";
    element("messages").append(empty);
  }
  area.scrollTop = scroll || nearBottom ? area.scrollHeight : oldTop;
}
