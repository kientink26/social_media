import { doGet, doPost } from "../http.js";
import { updateHasUnreadNotification } from "../header.js"

const PAGE_SIZE = 6;

const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <h1>Notifications</h1>
    <div id="notifications-outlet" class="notifications-wrapper"></div>
    <button id="load-more-button">Load more</button>
  </div>
`;
export default async function renderNotificationsPage() {
  const page = template.content.cloneNode(true);
  let { items, endCursor } = await fetchNotifications();
  const notifications = items;

  const notificationsOutlet = page.getElementById("notifications-outlet");

  const loadMoreButton = page.getElementById("load-more-button");
  if (items.length == 0) {
    loadMoreButton.remove();
  }
  const loadMoreButtonClick = async () => {
    ({ items, endCursor } = await fetchNotifications(endCursor));
    notifications.push(...items);
    for (const notification of items) {
      notificationsOutlet.appendChild(renderNotification(notification));
    }
    if (items.length < PAGE_SIZE) {
      loadMoreButton.remove();
    }
  };
  loadMoreButton.addEventListener("click", loadMoreButtonClick);

  for (const notification of notifications) {
    notificationsOutlet.appendChild(renderNotification(notification));
  }

  const onNotificationArrive = (newNotification) => {
    const oldNotificationEl = notificationsOutlet.querySelector('.' + newNotification.type + newNotification.postID)
    if (oldNotificationEl !== null) {
      oldNotificationEl.remove();
    }
    notifications.unshift(newNotification);
    notificationsOutlet.insertAdjacentElement(
      "afterbegin",
      renderNotification(newNotification)
    );
  };
  const unsubscribeFromNotifications =
    subscribeToNotifications(onNotificationArrive);

  page.addEventListener("disconnect", () => {
    console.log("notification page disconnected");
    unsubscribeFromNotifications();
  });

  return page;
}

function fetchNotifications(before = "") {
  return doGet(`/api/notifications?before=${before}&last=${PAGE_SIZE}`);
}

function renderNotification(notification) {
  const article = document.createElement("article");
  article.className = "notification";
  if (notification.read) {
    article.classList.add("read");
  } else {
    article.classList.add(notification.type + notification.postID)
  }
  let content = joinActors(
    notification.actors.map(
      (s) => `<a href="/users/${encodeURIComponent(s)}">${s}</a>`
    )
  );
  switch (notification.type) {
    case "follow":
      content += ` followed you`;
      break;
    case "comment":
      content += ` commented on a <a href="/posts/${encodeURIComponent(
        notification.postID
      )}">post</a>`;
      break;
    case "post_mention":
      content += ` mentioned you on a <a href="/posts/${encodeURIComponent(
        notification.postID
      )}">post</a>`;
      break;
    case "comment_mention":
      content += ` mentioned you on <a href="/posts/${encodeURIComponent(
        notification.postID
      )}">comment(s)</a> of a post`;
      break;
    default:
      content += " did something";
  }
  const ago = new Date(notification.issuedAt).toLocaleString();
  article.innerHTML = `
    <p>${content}</p>
    <time datetime="${notification.issuedAt}">${ago}</time>
  `;
  if (!notification.read) {
    const onNotificationClick = async () => {
      await markNotificationAsRead(notification.id);
      notification.read = true;
      article.classList.add("read");
      article.removeEventListener("click", onNotificationClick);
      await updateHasUnreadNotification();
    };

    article.addEventListener("click", onNotificationClick);
  }
  return article;
}

function markNotificationAsRead(notificationID) {
  return doPost(
    `/api/notifications/${encodeURIComponent(notificationID)}/mark_as_read`
  );
}

function joinActors(actors) {
  switch (actors.length) {
    case 0:
      return "Somebody";
    case 1:
      return actors[0];
    case 2:
      return `${actors[0]} and ${actors[1]}`;
    default:
      return `${actors[0]} and ${actors.length - 1} others`;
  }
}

function subscribeToNotifications(cb) {
  const onNotificationArrive = (ev) => {
    cb(ev.detail);
  };
  addEventListener("notificationarrive", onNotificationArrive);
  return () => {
    removeEventListener("notificationarrive", onNotificationArrive);
  };
}
