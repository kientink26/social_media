import { createRouter } from "./lib/router.js";
import { guard } from "./auth.js";

let currentPage;
const disconnectEvent = new CustomEvent("disconnect");
const r = createRouter();
r.route("/", guard(view("home"), view("access")));
r.route("/notifications", guard(view("notifications"), view("access")));
r.route("/search", view("search"));
r.route(/^\/users\/(?<username>[a-zA-Z][a-zA-Z0-9_-]{0,17})$/, view("user"));
r.route(/^\/users\/(?<username>[a-zA-Z][a-zA-Z0-9_-]{0,17})\/followers$/, view("followers"));
r.route(/^\/users\/(?<username>[a-zA-Z][a-zA-Z0-9_-]{0,17})\/followees$/, view("followees"));
r.route(/^\/posts\/(?<postID>[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/, view("post"));
r.route(/\//, view("not-found"));
r.subscribe(renderInto(document.querySelector("main")));
r.install();

function view(name) {
  return (...args) =>
    import(`./components/${name}-page.js`).then((m) => m.default(...args));
}

function renderInto(target) {
  return async (result) => {
    if (currentPage instanceof Node) {
      currentPage.dispatchEvent(disconnectEvent);
      target.innerHTML = "";
    }
    currentPage = await result;
    target.appendChild(currentPage);
  };
}
