import { getAuthUser } from "./auth.js";
import { doGet, subscribe } from "./http.js";

const authUser = getAuthUser();
const authenticated = authUser != null;
const rePostRoute =
  /^\/posts\/(?<postID>[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/;

const header = document.querySelector("header");

void (async function updateHeaderView() {
  header.innerHTML = `
  <nav>
    <a href="/">Home</a>
    ${
      authenticated
        ? `
      <a href="/users/${authUser.username}">Profile</a>
      <a href="/notifications" id="notifications-link">Notifications</a>
      <a href="/search">Search</a>
      <button id="logout-button">Logout</button>
    `
        : ""
    }
  </nav>
`;

  if (authenticated) {
    const logoutButton = header.querySelector("#logout-button");
    logoutButton.addEventListener("click", onLogoutButtonClick);
    const notificationsLink = header.querySelector("#notifications-link");

    updateHasUnreadNotification();

    const onNotificationArrive = async (notification) => {
      notificationsLink.classList.add("has-unread-notifications");
      dispatchEvent(
        new CustomEvent("notificationarrive", { detail: notification })
      );

      const match = rePostRoute.exec(location.pathname);
      if (match !== null) {
        const postID = decodeURIComponent(match.groups["postID"]);
        if (postID == notification.postID) {
          return;
        }
      }

      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        return;
      }

      const sysNotification = new Notification("New notification", {
        tag: notification.id,
        body: getNotificationBody(notification),
      });
    };

    let otp = "";
    if (authenticated) {
      ({ otp } = await doGet("/api/otp"));
    }
    subscribeToNotifications(onNotificationArrive, otp);
  }
})();

export async function updateHasUnreadNotification() {
  const notificationsLink = document.querySelector("#notifications-link");
  let hasUnreadNotifications = false;
  if (authenticated) {
    hasUnreadNotifications = await fetchHasUnreadNotifications();
  }
  if (hasUnreadNotifications) {
    notificationsLink.classList.add("has-unread-notifications");
  } else {
    notificationsLink.classList.remove("has-unread-notifications");
  }
}

function getNotificationBody(notification) {
  const actorsText = joinActors(notification.actors);
  switch (notification.type) {
    case "follow":
      return actorsText + " followed you";
    case "comment":
      return actorsText + " commented on a post";
    case "post_mention":
      return actorsText + " mentioned you on a post";
    case "comment_mention":
      return actorsText + " mentioned you on comment(s) of a post";
    default:
      return actorsText + " did something";
  }
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

function onLogoutButtonClick(ev) {
  const button = ev.currentTarget;
  button.disabled = true;
  localStorage.clear();
  location.reload();
}

function fetchHasUnreadNotifications() {
  return doGet("/api/has_unread_notifications");
}

function subscribeToNotifications(cb, otp) {
  return subscribe("/api/notifications", cb, otp);
}