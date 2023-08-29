import { isAuthenticated } from "../auth.js";
import renderAvatarHTML from "./avatar.js";
import { doPost } from "../http.js";

export default function renderUserProfile(user) {
  const authenticated = isAuthenticated();
  const div = document.createElement("div");
  div.className = "user-profile";
  div.innerHTML = `
      <div>
        ${renderAvatarHTML(user)}
        <a href="/users/${user.username}">
          <h1>${user.username}</h1>
        </a>
        ${
          user.followeed
            ? `
        <span class="badge">Follows you</span>
        `
            : ""
        }
        <br><br>
        <a href="/users/${user.username}/followers">
          <span class="followers-count-span">${user.followersCount}</span> 
          followers
        </a>
        <a href="/users/${user.username}/followees">${
    user.followeesCount
  } followees</a>
      </div>
      <div>
    ${
      authenticated && !user.me
        ? `
      <button class="follow-button" aria-pressed="${user.following}">
        ${user.following ? "Following" : "Follow"}
      </button>
    `
        : ""
    }
      </div>
  `;

  const followersCountSpan = div.querySelector(".followers-count-span");
  const followButton = div.querySelector(".follow-button");
  if (followButton !== null) {
    const onFollowButtonClick = async () => {
      followButton.disabled = true;
      try {
        const out = await http.toggleFollow(user.username);
        followersCountSpan.textContent = String(out.followersCount);
        followButton.setAttribute("aria-pressed", String(out.following));
        followButton.textContent = out.following ? "Following" : "Follow";
      } catch (err) {
        console.log(err);
        alert(err.message);
      } finally {
        followButton.disabled = false;
      }
    };
    followButton.addEventListener("click", onFollowButtonClick);
  }
  return div;
}

const http = {
  toggleFollow: (username) => doPost(`/api/users/${username}/toggle_follow`),
};
