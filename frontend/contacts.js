import { element } from "./ui.js";

export function renderContactList(
  { conversations, selectedUser, filter: selectedFilter, user: currentUser },
  onSelect,
) {
  const filter = element("search").value.toLowerCase();
  element("clear-search").hidden = !filter;
  const total = conversations.reduce((sum, u) => sum + u.unread_count, 0);
  element("unread-total").textContent = total.toLocaleString("fa-IR");
  const announcement = total
    ? total.toLocaleString("fa-IR") + " پیام نخونده داری"
    : "همهٔ پیام‌ها خونده شده";
  if (element("unread-announcement").textContent !== announcement)
    element("unread-announcement").textContent = announcement;
  const focused = element("contacts").contains(document.activeElement)
    ? document.activeElement.dataset.userId
    : null;
  element("contacts").replaceChildren();
  for (const user of conversations.filter(
    (u) => u.username.includes(filter) && (selectedFilter === "all" || u.unread_count > 0),
  )) {
    const button = document.createElement("button");
    button.className = "contact" + (selectedUser?.id === user.id ? " active" : "");
    button.type = "button";
    button.dataset.userId = user.id;
    button.setAttribute("aria-current", selectedUser?.id === user.id ? "true" : "false");
    const avatar = document.createElement("span");
    avatar.className = "avatar";
    avatar.textContent = user.username[0];
    const info = document.createElement("span");
    info.className = "contact-info";
    const name = document.createElement("strong");
    name.textContent = user.username;
    const label = document.createElement("small");
    label.dir = "auto";
    label.textContent = user.last_message
      ? (user.last_message.sender_id === currentUser.id ? "تو: " : "") +
        user.last_message.text.replace(/\s+/g, " ").slice(0, 100)
      : "یه سلام بفرست!";
    const top = document.createElement("span");
    top.className = "contact-top";
    top.append(name);
    if (user.last_message) {
      const time = document.createElement("time");
      time.dateTime = user.last_message.created_at;
      time.textContent = new Date(user.last_message.created_at).toLocaleTimeString("fa-IR", {
        hour: "2-digit",
        minute: "2-digit",
      });
      top.append(time);
    }
    info.append(top, label);
    button.append(avatar, info);
    if (user.unread_count > 0) {
      const badge = document.createElement("span");
      badge.className = "unread-badge";
      badge.textContent = user.unread_count.toLocaleString("fa-IR");
      badge.setAttribute("aria-label", user.unread_count + " پیام نخونده");
      button.append(badge);
    }
    button.onclick = () => onSelect(user);
    element("contacts").append(button);
  }
  if (!element("contacts").children.length) {
    const empty = document.createElement("p");
    empty.className = "list-empty";
    empty.textContent = filter ? "با این اسم کسی رو پیدا نکردیم." : "همه رو خوندی؛ چیزی جا نمونده!";
    element("contacts").append(empty);
  }
  if (focused)
    (
      [...element("contacts").children].find((node) => node.dataset.userId === focused) ||
      element("filter-unread")
    ).focus();
}
