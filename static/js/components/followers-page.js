import { doGet } from "../http.js";
import renderUserProfile from "./user.js";

const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <h1><span id="username-outlet"></span>'s followers</h1>
    <div id="followers-outlet" class="followers-wrapper users-wrapper"></div>
  </div>
`;
export default async function renderFollowersPage(params) {
  const username = params.username;
  const page = template.content.cloneNode(true);
  const followers = await fetchFollowers(username);

  const usernameOutlet = page.getElementById("username-outlet");
  usernameOutlet.innerHTML = `<a href="/users/${username}">${username}</a>`;
  const followersOutlet = page.getElementById("followers-outlet");

  for (const user of followers) {
    followersOutlet.appendChild(renderUserProfile(user));
  }

  return page;
}

function fetchFollowers(username) {
  return doGet(`/api/users/${username}/followers`);
}
