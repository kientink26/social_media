import { doGet } from "../http.js";
import renderUserProfile from "./user.js";

const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <h1><span id="username-outlet"></span>'s followees</h1>
    <div id="followees-outlet" class="followees-wrapper users-wrapper"></div>
  </div>
`;
export default async function renderFolloweesPage(params) {
  const username = params.username;
  const page = template.content.cloneNode(true);
  const followees = await fetchFollowees(username);

  const usernameOutlet = page.getElementById("username-outlet");
  usernameOutlet.innerHTML = `<a href="/users/${username}">${username}</a>`;
  const followeesOutlet = page.getElementById("followees-outlet");

  for (const user of followees) {
    followeesOutlet.appendChild(renderUserProfile(user));
  }

  return page;
}

function fetchFollowees(username) {
  return doGet(`/api/users/${username}/followees`);
}
