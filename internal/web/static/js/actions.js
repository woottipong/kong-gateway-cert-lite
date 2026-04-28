(function () {
  const confirmedForms = new WeakSet();
  const state = {
    form: null,
    submitter: null,
    previousFocus: null,
  };

  function getDialogParts() {
    const backdrop = document.querySelector("[data-confirm-backdrop]");
    if (!backdrop) {
      return null;
    }

    return {
      backdrop,
      dialog: backdrop.querySelector(".app-confirm-dialog"),
      title: backdrop.querySelector("[data-confirm-title]"),
      kicker: backdrop.querySelector("[data-confirm-kicker]"),
      message: backdrop.querySelector("[data-confirm-message]"),
      icon: backdrop.querySelector("[data-confirm-icon]"),
      cancel: backdrop.querySelector("[data-confirm-cancel]"),
      accept: backdrop.querySelector("[data-confirm-accept]"),
    };
  }

  function actionTone(form, submitter) {
    const explicitTone = form.dataset.confirmTone || submitter?.dataset.confirmTone;
    if (explicitTone) {
      return explicitTone;
    }

    const action = form.getAttribute("action") || "";
    const text = [
      form.dataset.confirm,
      submitter?.textContent,
      action,
    ].join(" ").toLowerCase();

    if (text.includes("delete") || text.includes("/delete")) {
      return "danger";
    }
    if (text.includes("issue") || text.includes("sync") || text.includes("test")) {
      return "warning";
    }
    return "primary";
  }

  function actionTitle(tone) {
    if (tone === "danger") {
      return "Confirm destructive action";
    }
    if (tone === "warning") {
      return "Confirm operation";
    }
    return "Confirm changes";
  }

  function actionLabel(tone, submitter) {
    const submitterLabel = submitter?.textContent?.trim();
    if (submitterLabel) {
      return submitterLabel;
    }
    if (tone === "danger") {
      return "Delete";
    }
    return "Continue";
  }

  function closeDialog(parts) {
    parts.backdrop.hidden = true;
    document.documentElement.classList.remove("app-confirm-open");
    parts.backdrop.dataset.confirmTone = "";
    state.form = null;
    state.submitter = null;

    if (state.previousFocus instanceof HTMLElement) {
      state.previousFocus.focus();
    }
    state.previousFocus = null;
  }

  function focusableElements(container) {
    return Array.from(container.querySelectorAll([
      "a[href]",
      "button:not([disabled])",
      "input:not([disabled])",
      "select:not([disabled])",
      "textarea:not([disabled])",
      "[tabindex]:not([tabindex='-1'])",
    ].join(","))).filter(function (element) {
      return element instanceof HTMLElement && element.offsetParent !== null;
    });
  }

  function submitConfirmed(parts) {
    const form = state.form;
    const submitter = state.submitter;
    if (!form) {
      closeDialog(parts);
      return;
    }

    confirmedForms.add(form);
    closeDialog(parts);

    if (typeof form.requestSubmit === "function") {
      form.requestSubmit(submitter instanceof HTMLElement ? submitter : undefined);
      return;
    }
    form.submit();
  }

  function openDialog(form, submitter) {
    const parts = getDialogParts();
    if (!parts || !parts.dialog || !parts.message || !parts.accept || !parts.cancel) {
      return false;
    }

    const tone = actionTone(form, submitter);
    state.form = form;
    state.submitter = submitter;
    state.previousFocus = document.activeElement;

    parts.backdrop.dataset.confirmTone = tone;
    parts.message.textContent = form.dataset.confirm;
    if (parts.title) {
      parts.title.textContent = form.dataset.confirmTitle || actionTitle(tone);
    }
    if (parts.kicker) {
      parts.kicker.textContent = tone === "danger" ? "Requires confirmation" : "Review before continuing";
    }
    if (parts.icon) {
      parts.icon.textContent = tone === "danger" ? "!" : "?";
    }
    parts.accept.textContent = actionLabel(tone, submitter);
    parts.accept.className = tone === "danger" ? "btn btn-danger" : "btn btn-primary";

    parts.backdrop.hidden = false;
    document.documentElement.classList.add("app-confirm-open");
    parts.dialog.focus();
    return true;
  }

  document.addEventListener("submit", function (event) {
    const form = event.target;
    if (!(form instanceof HTMLFormElement)) {
      return;
    }

    if (confirmedForms.has(form)) {
      confirmedForms.delete(form);
      return;
    }

    const message = form.dataset.confirm;
    if (!message) {
      return;
    }

    event.preventDefault();
    if (!openDialog(form, event.submitter)) {
      if (window.confirm(message)) {
        confirmedForms.add(form);
        if (typeof form.requestSubmit === "function") {
          form.requestSubmit(event.submitter);
          return;
        }
        form.submit();
      }
    }
  });

  document.addEventListener("click", function (event) {
    const parts = getDialogParts();
    if (!parts || parts.backdrop.hidden) {
      return;
    }

    if (event.target === parts.backdrop || event.target === parts.cancel) {
      closeDialog(parts);
      return;
    }

    if (event.target === parts.accept) {
      submitConfirmed(parts);
    }
  });

  document.addEventListener("keydown", function (event) {
    const parts = getDialogParts();
    if (!parts || parts.backdrop.hidden) {
      return;
    }
    if (event.key === "Escape") {
      closeDialog(parts);
      return;
    }
    if (event.key !== "Tab" || !parts.dialog) {
      return;
    }

    const focusable = focusableElements(parts.dialog);
    if (focusable.length === 0) {
      event.preventDefault();
      parts.dialog.focus();
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
      return;
    }
    if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });
})();
