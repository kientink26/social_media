import { doGet } from "../http.js";
import renderUserProfile from "./user.js";
import { navigate } from "../lib/router.js"

const template = document.createElement("template");
template.innerHTML = `
  <div class="container">
    <h1>Search</h1>
    <form id="search-form" class="search-form">
      <input type="search" name="q" placeholder="Search..." autocomplete="off" autofocus>
    </form>
    <div id="search-results-outlet" class="search-results-wrapper users-wrapper"></div>
  </div>
`;
export default async function renderSearchPage() {
  const url = new URL(location.toString());
  const searchQuery = url.searchParams.has("q")
    ? decodeURIComponent(url.searchParams.get("q")).trim()
    : "";

  const page = template.content.cloneNode(true);

  const users = await fetchUsers(searchQuery);
  const searchForm = page.getElementById("search-form");
  const searchInput = searchForm.querySelector("input");
  const searchResultsOutlet = page.getElementById("search-results-outlet");

  const onSearchFormSubmit = (ev) => {
    ev.preventDefault();
    const searchQuery = searchInput.value.trim();
    navigate("/search?q=" + encodeURIComponent(searchQuery));
  };

  searchForm.addEventListener("submit", onSearchFormSubmit);
  searchInput.value = searchQuery;
  setTimeout(() => {
    searchInput.focus();
  });
  for (const user of users) {
    searchResultsOutlet.appendChild(renderUserProfile(user));
  }

  return page;
}

function fetchUsers(search) {
  return doGet(`/api/users?search=${search}`);
}
