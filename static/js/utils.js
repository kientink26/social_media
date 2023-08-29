export function isOject(x) {
  return typeof x === "object" && x != null;
}

export function isPlainOject(x) {
  return isOject(x) && !Array.isArray(x);
}

export function escapeHTML(s) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}
