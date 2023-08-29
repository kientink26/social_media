import { getAuthUser } from "../auth.js";
import { doGet, doPost, subscribe } from "../http.js";
import renderPost from "./post.js";

const PAGE_SIZE = 5;

const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <h1>Timeline</h1>
    <form id="post-form">
      <textarea placeholder="Write something..." maxlength="480" required></textarea>
      <button>Publish</button>
    </form>
    <ol id="timeline-list" class="post-list"></ol>
    <button id="load-more-button">Load more</button>
  </div>
`;

export default async function renderHomePage() {
  let { items, endCursor } = await http.timeline();
  const timeline = items;

  const page = template.content.cloneNode(true);
  const postForm = page.getElementById("post-form");
  const postFormTextArea = postForm.querySelector("textarea");
  const postFormButton = postForm.querySelector("button");
  const timelineList = page.getElementById("timeline-list");
  const loadMoreButton = page.getElementById("load-more-button");

  const onPostFormSubmit = async (ev) => {
    ev.preventDefault();
    const content = postFormTextArea.value;
    postFormTextArea.disabled = true;
    postFormButton.disabled = true;
    try {
      const timelineItem = await http.publishPost({ content });
      timeline.unshift(timelineItem);
      timelineList.insertAdjacentElement(
        "afterbegin",
        renderPost(timelineItem.post)
      );
      postForm.reset();
    } catch (err) {
      console.error(err);
      alert(err.message);
      setTimeout(() => {
        postFormTextArea.focus();
      });
    } finally {
      postFormTextArea.disabled = false;
      postFormButton.disabled = false;
    }
  };

  if (items.length == 0) {
    loadMoreButton.remove();
  }
  const loadMoreButtonClick = async () => {
    ({ items, endCursor } = await http.timeline(endCursor));
    timeline.push(...items);
    for (const timelineItem of items) {
      timelineList.appendChild(renderPost(timelineItem.post));
    }
    if (items.length < PAGE_SIZE) {
      loadMoreButton.remove();
    }
  };
  loadMoreButton.addEventListener("click", loadMoreButtonClick);

  const onTimelineItemArrive = (timelineItem) => {
    timeline.unshift(timelineItem);
    timelineList.insertAdjacentElement(
      "afterbegin",
      renderPost(timelineItem.post)
    );
  };

  for (const timelineItem of timeline) {
    timelineList.appendChild(renderPost(timelineItem.post));
  }

  const { otp } = await doGet("/api/otp");
  const timelineUnsubscription =
    http.timelineSubscription(onTimelineItemArrive, otp);

  postForm.addEventListener("submit", onPostFormSubmit);
  page.addEventListener("disconnect", () => {
    console.log("home page disconnected");
    timelineUnsubscription();
  });

  return page;
}

const http = {
  publishPost: (input) =>
    doPost("/api/timeline", input).then((timelineItem) => {
      timelineItem.post.user = getAuthUser();
      return timelineItem;
    }),
  timeline: (before = "") =>
    doGet(`api/timeline?before=${before}&last=${PAGE_SIZE}`),

  timelineSubscription: (cb, otp) => subscribe("/api/timeline", cb, otp),
};
