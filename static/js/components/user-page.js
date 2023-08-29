import { doGet } from "../http.js";
import renderPost from "./post.js";
import renderUserProfile from "./user.js";

const PAGE_SIZE = 3;

const template = document.createElement("template");
template.innerHTML = `
  <div class="user-wrapper">
    <div class="container wide">
      <div id="user-div"></div>
    </div>
  </div>
  <div class="container">
    <h2>Posts</h2>
    <ol id="posts-list"></ol>
    <button id="load-more-button" class="load-more-posts-button">Load more</button>
  </div>
`;

export default async function renderUserPage(params) {
  let [user, { items, endCursor }] = await Promise.all([
    http.fetchUser(params.username),
    http.fetchPosts(params.username),
  ]);
  const posts = items;
  const page = template.content.cloneNode(true);
  const userDiv = page.getElementById("user-div");
  const postsList = page.getElementById("posts-list");
  const loadMoreButton = page.getElementById("load-more-button");

  userDiv.appendChild(renderUserProfile(user));

  if (items.length == 0) {
    loadMoreButton.remove();
  }
  const loadMoreButtonClick = async () => {
    ({ items, endCursor } = await http.fetchPosts(params.username, endCursor));
    posts.push(...items);
    for (const post of items) {
      post.user = user;
      postsList.appendChild(renderPost(post));
    }
    if (items.length < PAGE_SIZE) {
      loadMoreButton.remove();
    }
  };
  loadMoreButton.addEventListener("click", loadMoreButtonClick);

  for (const post of posts) {
    post.user = user;
    postsList.appendChild(renderPost(post));
  }
  return page;
}

const http = {
  fetchUser: (username) => doGet("/api/users/" + username),

  fetchPosts: (username, before = "") =>
    doGet(`/api/users/${username}/posts?before=${before}&last=${PAGE_SIZE}`),
};
