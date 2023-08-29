import { doGet, doPost, subscribe } from "../http.js";
import renderPost from "./post.js";
import renderComment from "./comment.js";
import { getAuthUser, isAuthenticated } from "../auth.js";

const PAGE_SIZE = 5;

const authenticated = isAuthenticated();
const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <div id="post-outlet"></div>
  </div>
  <br>
  <div class="container">
    ${
      authenticated
        ? `
    <form id="comment-form" class="comment-form">
      <textarea placeholder="Say something..." maxlength="480" required></textarea>
      <button>Comment</button>
    </form>`
        : ""
    }
    <ol id="comments-list" class="comments-list"></ol>
    <button id="load-more-button" class="load-more-comments-button">Load more</button>
  </div>
`;

export default async function renderPostPage(params) {
  const postID = params.postID;
  let [post, { items, endCursor }] = await Promise.all([
    fetchPost(postID),
    fetchComments(postID),
  ]);
  const comments = items;
  const page = template.content.cloneNode(true);
  const postOutlet = page.getElementById("post-outlet");
  const commentsList = page.getElementById("comments-list");
  const commentForm = page.getElementById("comment-form");
  const loadMoreButton = page.getElementById("load-more-button");
  let commentsCountEl = null;
  let subscribeBtn = null;

  const incrementCommentsCount = () => {
    if (commentsCountEl === null) {
      commentsCountEl = postOutlet.querySelector(".comments-count");
    }
    commentsCountEl.textContent = String(++post.commentsCount);
  };

  const subscribePost = () => {
    if (subscribeBtn === null) {
      subscribeBtn = postOutlet.querySelector(".subscribe-button");
    }
    subscribeBtn.title = "Unsubscribe";
    subscribeBtn.setAttribute("aria-pressed", "true");
    subscribeBtn.textContent = "Subscribing";
  };

  if (commentForm !== null) {
    const commentFormTextArea = commentForm.querySelector("textarea");
    const commentFormButton = commentForm.querySelector("button");

    const onCommentFormSubmit = async (ev) => {
      ev.preventDefault();
      const content = commentFormTextArea.value;
      commentFormTextArea.disabled = true;
      commentFormButton.disabled = true;
      try {
        const newComment = await createComment(post.id, content);
        comments.unshift(newComment);
        commentsList.insertAdjacentElement(
          "afterbegin",
          renderComment(newComment)
        );
        incrementCommentsCount();
        subscribePost();
        commentForm.reset();
      } catch (err) {
        console.log(err);
        alert(err.message);
        setTimeout(() => {
          commentFormTextArea.focus();
        });
      } finally {
        commentFormTextArea.disabled = false;
        commentFormButton.disabled = false;
      }
    };
    commentForm.addEventListener("submit", onCommentFormSubmit);
  }

  if (items.length == 0) {
    loadMoreButton.remove();
  }
  const loadMoreButtonClick = async () => {
    ({ items, endCursor } = await fetchComments(postID, endCursor));
    comments.push(...items);
    for (const comment of items) {
      commentsList.appendChild(renderComment(comment));
    }
    if (items.length < PAGE_SIZE) {
      loadMoreButton.remove();
    }
  };
  loadMoreButton.addEventListener("click", loadMoreButtonClick);

  for (const comment of comments) {
    commentsList.appendChild(renderComment(comment));
  }

  postOutlet.appendChild(renderPost(post));

  const onCommentArrive = (newComment) => {
    comments.unshift(newComment);
    commentsList.insertAdjacentElement("afterbegin", renderComment(newComment));
    incrementCommentsCount();
  };

  let otp = "";
  if (authenticated) {
    ({ otp } = await doGet("/api/otp"));
  }
  const unsubcribeFromComments = subscribeToComments(
    postID,
    onCommentArrive,
    otp
  );
  page.addEventListener("disconnect", () => {
    console.log("post page disconnected");
    unsubcribeFromComments();
  });
  return page;
}

function fetchPost(postID) {
  return doGet(`/api/posts/${postID}`);
}

function fetchComments(postID, before = "") {
  return doGet(
    `/api/posts/${postID}/comments?before=${before}&last=${PAGE_SIZE}`
  );
}

async function createComment(postID, content) {
  const comment = await doPost(`/api/posts/${postID}/comments`, { content });
  comment.user = getAuthUser();
  return comment;
}

function subscribeToComments(postID, cb, otp) {
  return subscribe(`/api/posts/${postID}/comments`, cb, otp);
}
