import { isAuthenticated } from "./auth.js";
import { isOject } from "./utils.js";

export function doGet(url, headers) {
  return fetch(url, {
    headers: Object.assign(defaultHeaders(), headers),
  }).then(parseResponse);
}

export function doPost(url, body, headers) {
  const init = {
    method: "POST",
    headers: defaultHeaders(),
  };
  if (isOject(body)) {
    init["body"] = JSON.stringify(body);
    init.headers["content-type"] = "application/json; charset=utf-8";
  }
  Object.assign(init.headers, headers);
  return fetch(url, init).then(parseResponse);
}

function defaultHeaders() {
  return isAuthenticated()
    ? {
        authorization: "Bearer " + localStorage.getItem("token"),
      }
    : {};
}

async function parseResponse(resp) {
  return resp
    .clone()
    .json()
    .catch(() => resp.text())
    .then((body) => {
      if (!resp.ok) {
        const err = new Error();
        if (typeof body === "string" && body.trim() !== "") {
          err.message = body.trim();
        } else if (
          typeof body === "object" &&
          body !== null &&
          typeof body.error === "string"
        ) {
          err.message = body.error;
        } else {
          err.message = resp.statusText;
        }
        err.name = err.message
          .split(" ")
          .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
          .join("");
        if (!err.name.endsWith("Error")) {
          err.name = err.name + "Error";
        }
        err["headers"] = resp.headers;
        err["statusCode"] = resp.status;
        throw err;
      }
      return body;
    });
}

export function subscribe(url, cb, otp) {
  if (isAuthenticated()) {
    const _url = new URL(url, location.origin);
    _url.searchParams.set("otp", otp);
    url = _url.toString();
  }
  const eventSource = new EventSource(url);
  eventSource.onmessage = (ev) => {
    try {
      cb(JSON.parse(ev.data));
    } catch (_) {}
  };
  return () => {
    eventSource.close();
  };
}

export default {
  get: doGet,
  post: doPost,
  subscribe: subscribe,
};
