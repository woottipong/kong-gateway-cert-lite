(function () {
  var storageKey = "kong-cert-lite-theme";
  var toggle = document.querySelector("[data-theme-toggle]");
  var icon = document.querySelector("[data-theme-toggle-icon]");

  function currentTheme() {
    return document.documentElement.dataset.theme === "light" ? "light" : "dark";
  }

  function applyTheme(theme) {
    if (theme === "light") {
      document.documentElement.dataset.theme = "light";
      if (toggle) {
        toggle.setAttribute("aria-pressed", "true");
        toggle.setAttribute("aria-label", "Use dark theme");
      }
      if (icon) {
        icon.textContent = "☀";
      }
      return;
    }

    delete document.documentElement.dataset.theme;
    if (toggle) {
      toggle.setAttribute("aria-pressed", "false");
      toggle.setAttribute("aria-label", "Use light theme");
    }
    if (icon) {
      icon.textContent = "☾";
    }
  }

  applyTheme(currentTheme());

  if (!toggle) {
    return;
  }

  toggle.addEventListener("click", function () {
    var nextTheme = currentTheme() === "light" ? "dark" : "light";
    localStorage.setItem(storageKey, nextTheme);
    applyTheme(nextTheme);
  });
})();
