import { doPost } from "../http.js";

const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <h1>Access Page</h1>
    <form id="login-form">
      <input type="text" placeholder="Email" autocomplete="email" required>
      <input type="password" placeholder="Password" required>
      <button>Login</button>
    </form>
  </div>
`;

export default function renderAccessPage() {
  const page = template.content.cloneNode(true);
  const loginForm = page.getElementById("login-form");
  loginForm.addEventListener("submit", onLoginFormSubmit);
  return page;
}

async function onLoginFormSubmit(ev) {
  ev.preventDefault();
  const form = ev.currentTarget;
  const input = form.querySelector("input");
  const button = form.querySelector("button");
  const email = input.value;

  input.disabled = true;
  button.disabled = true;

  try {
    saveLogin(await http.login(email));
    location.reload();
  } catch (err) {
    console.error(err);
    if (err.name === "UserNotFoundError") {
      if (confirm("User not found. Do you want to create an account?")) {
        runRegistrationProgram(email);
      }
      return;
    }
    setTimeout(() => {
      input.focus();
    });
  } finally {
    input.disabled = false;
    button.disabled = false;
  }
}

function saveLogin(payLoad) {
  localStorage.setItem("token", payLoad.token);
  localStorage.setItem("expires_at", String(payLoad.expiresAt));
  localStorage.setItem("auth_user", JSON.stringify(payLoad.user));
}

const http = {
  login: (email) => doPost("/api/login", { email }),
  createUser: (email, username) => doPost("/api/users", { email, username }),
};

const rxUsername = /^[a-zA-Z][a-zA-Z0-9_-]{0,17}$/;

async function runRegistrationProgram(email, username) {
  username = prompt("Username:", username);
  if (username === null) {
    return;
  }

  username = username.trim();
  if (!rxUsername.test(username)) {
    alert("invalid username");
    runRegistrationProgram(email, username);
    return;
  }

  try {
    await http.createUser(email, username);
    saveLogin(await http.login(email));
    location.reload();
  } catch (err) {
    console.error(err);
    alert(err.message);
    if (err.name === "UsernameTakenError") {
      runRegistrationProgram(email);
    }
  }
}
