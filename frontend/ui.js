export const element = (id) => document.getElementById(id);

const iconPaths = {
  search: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/>',
  back: '<path d="m9 5 7 7-7 7"/>',
  down: '<path d="M12 4v16m-6-6 6 6 6-6"/>',
  smile: '<circle cx="12" cy="12" r="9"/><path d="M8 14s1 3 4 3 4-3 4-3M8 9h.01M16 9h.01"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="3"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/>',
};
export function icon(name) {
  return (
    '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
    iconPaths[name] +
    "</svg>"
  );
}

let toastTimer;
export function toast(text) {
  clearTimeout(toastTimer);
  element("toast").textContent = text;
  element("toast").hidden = false;
  toastTimer = setTimeout(() => (element("toast").hidden = true), 2800);
}
