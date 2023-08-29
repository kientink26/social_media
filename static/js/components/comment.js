import renderAvatarHTML from "./avatar.js";
import { escapeHTML } from "../utils.js";
import { isAuthenticated } from "../auth.js";
import { doPost } from "../http.js";

export default function renderComment(comment) {
  const authenticated = isAuthenticated();
  const { user } = comment;
  const ago = new Date(comment.createdAt).toLocaleString();
  const li = document.createElement("li");
  li.className = "comment-item";
  li.innerHTML = `
    <article class="comment">
      <div class="comment-header">
        <a href="/users/${user.username}">
          ${renderAvatarHTML(user)}
          <span class="username">${user.username}</span>
        </a>
        <time datetime="${comment.createdAt}">${ago}</time>
      </div>
      <div class="comment-content">${escapeHTML(comment.content)}</div>
      <div class="comment-controls">
        ${
          authenticated
            ? `
          <button class="like-button"
            title="${comment.liked ? "Unlike" : "Like"}"
            aria-label="${comment.likesCount} likes">
            <span class="likes-count">${comment.likesCount}</span>
            <span class="like-emoji">${
              comment.liked ? `&#128150;` : `&#9825;`
            }<span>
          </button>
        `
            : `
          <span class="brick" aria-label="${comment.likesCount} likes">
            <span class="likes-count">${comment.likesCount}</span>
            &#9825;
          </span>
        `
        }
      </div>
    </article>
  `;
  const likeButton = li.querySelector(".like-button");
  if (likeButton !== null) {
    const likesCountEl = likeButton.querySelector(".likes-count");
    const likeEmojiEl = likeButton.querySelector(".like-emoji");

    const onLikeButtonClick = async () => {
      likeButton.disabled = true;
      try {
        const out = await toggleCommentLike(comment.id);

        comment.likesCount = out.likesCount;
        comment.liked = out.liked;
        likeButton.title = comment.liked ? "Unlike" : "Like";
        likeButton.setAttribute("aria-label", comment.likesCount + "likes");
        likesCountEl.textContent = String(comment.likesCount);
        likeEmojiEl.innerHTML = comment.liked ? `&#128150;` : `&#9825;`;
      } catch (err) {
        console.log(err);
        alert(err.message);
      } finally {
        likeButton.disabled = false;
      }
    };
    likeButton.addEventListener("click", onLikeButtonClick);
  }
  return li;
}

function toggleCommentLike(commentID) {
  return doPost(`/api/comments/${commentID}/toggle_like`);
}
