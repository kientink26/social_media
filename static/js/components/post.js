import renderAvatarHTML from "./avatar.js";
import { escapeHTML } from "../utils.js";
import { isAuthenticated } from "../auth.js";
import { doPost } from "../http.js";

export default function renderPost(post) {
  const authenticated = isAuthenticated();
  const { user } = post;
  const ago = new Date(post.createdAt).toLocaleString();
  const li = document.createElement("li");
  li.className = "post-item";
  li.innerHTML = `
    <article class="post">
      <div class="post-header">
        <a href="/users/${user.username}">
          ${renderAvatarHTML(user)}
          <span class="username">${user.username}</span>
        </a>
        <a href="/posts/${post.id}">
          <time datetime="${post.createdAt}">${ago}</time>
        </a>
      </div>
      <div class="post-grid">
        <div>
          <div class="post-content">${escapeHTML(post.content)}</div>
          <div class="post-controls">
            ${
              authenticated
                ? `
              <button class="like-button"
                title="${post.liked ? "Unlike" : "Like"}"
                aria-label="${post.likesCount} likes">
                <span class="likes-count">${post.likesCount}</span>
                <span class="like-emoji">${
                  post.liked ? `&#128150;` : `&#9825;`
                }<span>
              </button>
            `
                : `
              <span class="brick" aria-label="${post.likesCount} likes">
                <span class="likes-count">${post.likesCount}</span>
                &#9825;
              </span>
            `
            }
            <a class="comments-link" href="/posts/${post.id}">
              <span class="comments-count">${post.commentsCount}</span>
              &#128172;
            </a>
          </div>
        </div>
        <div>
        ${
          authenticated
            ? `
          <button class="subscribe-button" 
              title="${post.subscribed ? "Unsubscribe" : "Subscribe"}"
              aria-pressed="${post.subscribed}">
            ${post.subscribed ? "Subscribing" : "Subscribe"}
          </button>
        `
            : ""
        }
        </div>
      <div>
    </article>
  `;

  const likeButton = li.querySelector(".like-button");
  if (likeButton !== null) {
    const likesCountEl = likeButton.querySelector(".likes-count");
    const likeEmojiEl = likeButton.querySelector(".like-emoji");

    const onLikeButtonClick = async () => {
      likeButton.disabled = true;
      try {
        const out = await togglePostLike(post.id);

        post.likesCount = out.likesCount;
        post.liked = out.liked;
        likeButton.title = post.liked ? "Unlike" : "Like";
        likeButton.setAttribute("aria-label", post.likesCount + "likes");
        likesCountEl.textContent = String(post.likesCount);
        likeEmojiEl.innerHTML = post.liked ? `&#128150;` : `&#9825;`;
      } catch (err) {
        console.log(err);
        alert(err.message);
      } finally {
        likeButton.disabled = false;
      }
    };
    likeButton.addEventListener("click", onLikeButtonClick);
  }

  const subscribeButton = li.querySelector(".subscribe-button");
  if (subscribeButton !== null) {
    const onSubscribeButtonClick = async () => {
      subscribeButton.disabled = true;
      try {
        const out = await togglePostSubscription(post.id);
        post.subscribed = out.subscribed;
        subscribeButton.title = post.subscribed ? "Unsubscribe" : "Subscribe";
        subscribeButton.setAttribute("aria-pressed", String(post.subscribed));
        subscribeButton.textContent = post.subscribed
          ? "Subscribing"
          : "Subscribe";
      } catch (err) {
        console.log(err);
        alert(err.message);
      } finally {
        subscribeButton.disabled = false;
      }
    };
    subscribeButton.addEventListener("click", onSubscribeButtonClick);
  }
  return li;
}

function togglePostLike(postID) {
  return doPost(`/api/posts/${postID}/toggle_like`);
}

function togglePostSubscription(postID) {
  return doPost(`/api/posts/${postID}/toggle_subscription`);
}
