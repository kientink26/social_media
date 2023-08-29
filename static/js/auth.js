export function getAuthUser() {
  const authUserRaw = localStorage.getItem("auth_user");
  if (authUserRaw === null) {
    return null;
  }
  if (localStorage.getItem("token") === null) {
    return null;
  }
  const expiresAtRaw = localStorage.getItem("expires_at");
  if (expiresAtRaw === null) {
    return null;
  }
  const expiresAt = new Date(expiresAtRaw);
  if (isNaN(expiresAt.valueOf()) || expiresAt <= new Date()) {
    return null;
  }
  return JSON.parse(authUserRaw);
}

export function isAuthenticated() {
  return getAuthUser() !== null;
}

export function guard(fn1, fn2) {
  return (...args) => (isAuthenticated() ? fn1(...args) : fn2(...args));
}
