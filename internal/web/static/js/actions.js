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
      dialog: backdrop.querySelector(".modal-content"),
      title: backdrop.querySelector("[data-confirm-title]"),
      message: backdrop.querySelector("[data-confirm-message]"),
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
    parts.backdrop.classList.remove("show", "d-block");
    parts.backdrop.hidden = true;
    parts.backdrop.setAttribute("aria-hidden", "true");
    document.body.classList.remove("modal-open");
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
    markFormSubmitting(form, submitter);
    form.submit();
  }

  function markFormSubmitting(form, submitter) {
    if (!(form instanceof HTMLFormElement)) {
      return;
    }

    const button = submitter instanceof HTMLButtonElement
      ? submitter
      : form.querySelector("button[type='submit']");
    if (!button) {
      return;
    }

    const label = button.dataset.submittingLabel || form.dataset.submittingLabel;
    if (!label) {
      return;
    }

    button.disabled = true;
    button.setAttribute("aria-busy", "true");
    button.textContent = label;

    const busyText = form.querySelector("[data-submit-status]");
    if (busyText) {
      busyText.hidden = false;
    }
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
    parts.accept.textContent = actionLabel(tone, submitter);
    parts.accept.className = tone === "danger" ? "btn btn-danger" : tone === "warning" ? "btn btn-warning" : "btn btn-primary";

    parts.backdrop.classList.add("show", "d-block");
    parts.backdrop.hidden = false;
    parts.backdrop.setAttribute("aria-hidden", "false");
    document.body.classList.add("modal-open");
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
      markFormSubmitting(form, event.submitter);
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
        markFormSubmitting(form, event.submitter);
        form.submit();
      }
    }
  });

  document.addEventListener("click", function (event) {
    const row = event.target.closest("tr[data-row-href]");
    if (row && !event.target.closest("[onclick], a, button, .dropdown, form")) {
      window.location.href = row.dataset.rowHref;
      return;
    }

    const copyButton = event.target.closest("[data-copy-target]");
    if (copyButton) {
      copyTextFromTarget(copyButton);
      return;
    }

    const parts = getDialogParts();
    if (!parts || parts.backdrop.hidden) {
      return;
    }

    if (event.target === parts.backdrop || event.target.closest("[data-confirm-cancel]")) {
      closeDialog(parts);
      return;
    }

    if (event.target === parts.accept) {
      submitConfirmed(parts);
    }
  });

  function copyTextFromTarget(button) {
    if (!(button instanceof HTMLElement)) {
      return;
    }

    const selector = button.dataset.copyTarget;
    const target = selector ? document.querySelector(selector) : null;
    if (!target) {
      return;
    }

    const text = target.textContent || "";
    const original = button.textContent || "Copy";

    function markCopied() {
      button.textContent = "Copied";
      window.setTimeout(function () {
        button.textContent = original;
      }, 1600);
    }

    if (navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
      navigator.clipboard.writeText(text).then(markCopied).catch(function () {
        fallbackCopy(text, markCopied);
      });
      return;
    }

    fallbackCopy(text, markCopied);
  }

  function fallbackCopy(text, onCopied) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "");
    textarea.style.position = "fixed";
    textarea.style.left = "-9999px";
    document.body.appendChild(textarea);
    textarea.select();
    try {
      if (document.execCommand("copy")) {
        onCopied();
      }
    } finally {
      textarea.remove();
    }
  }

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

(function () {
  function initFlashToast() {
    var toast = document.querySelector("[data-flash-toast]");
    if (!toast) return;

    var dismissed = false;
    var dismissButton = toast.querySelector("[data-flash-dismiss]");

    function dismiss() {
      if (dismissed) return;
      dismissed = true;
      toast.classList.remove("show");
      window.setTimeout(function () {
        var region = toast.closest("[data-flash-region]");
        if (region) {
          region.remove();
          return;
        }
        toast.remove();
      }, 150);
    }

    if (dismissButton) {
      dismissButton.addEventListener("click", dismiss);
    }

    window.setTimeout(dismiss, 4500);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initFlashToast);
  } else {
    initFlashToast();
  }
})();

(function () {
  function initAutoRenewPolicy() {
    var toggle = document.querySelector("[data-auto-renew-toggle]");
    var days = document.querySelector("[data-auto-renew-days]");
    if (!toggle || !days) return;

    function sync() {
      days.disabled = !toggle.checked;
    }

    toggle.addEventListener("change", sync);
    sync();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initAutoRenewPolicy);
  } else {
    initAutoRenewPolicy();
  }
})();

(function () {
  function initTagInputs() {
    document.querySelectorAll("[data-tag-for]").forEach(function (container) {
      var name = container.dataset.tagFor;
      var textarea = document.getElementById(name);
      if (!textarea) return;
      var locked = container.hasAttribute("data-tag-locked");

      var input = document.createElement("input");
      input.type = "text";
      input.id = name + "-input";
      input.className = "app-tag-text";
      input.placeholder = locked ? "" : "type and press Enter";
      if (locked) input.disabled = true;
      if (container.dataset.tagLabel) input.setAttribute("aria-label", container.dataset.tagLabel);
      if (container.dataset.tagDescribedby) input.setAttribute("aria-describedby", container.dataset.tagDescribedby);
      if (container.dataset.tagInvalid) input.setAttribute("aria-invalid", container.dataset.tagInvalid);
      
      container.appendChild(input);

      function sync() {
        textarea.value = Array.from(container.querySelectorAll(".app-tag"))
          .map(function (t) { return t.dataset.value; })
          .join("\n");
      }

      function addTag(value) {
        value = value.trim();
        if (!value) return;
        var exists = Array.from(container.querySelectorAll(".app-tag"))
          .some(function (t) { return t.dataset.value === value; });
        if (exists) return;

        var tag = document.createElement("span");
        tag.className = "app-tag";
        tag.dataset.value = value;
        tag.textContent = value;

        if (!locked) {
          var btn = document.createElement("button");
          btn.type = "button";
          btn.className = "app-tag-remove";
          btn.setAttribute("aria-label", "Remove " + value);
          btn.textContent = "\u00d7";
          btn.addEventListener("click", function () {
            tag.remove();
            sync();
          });
          tag.appendChild(btn);
        }

        container.insertBefore(tag, input);
        sync();
      }

      (textarea.value || "").split("\n").forEach(function (v) {
        if (v.trim()) addTag(v.trim());
      });

      if (!locked) {
        input.addEventListener("keydown", function (e) {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            addTag(input.value);
            input.value = "";
          }
          if (e.key === "Backspace" && input.value === "") {
            var tags = container.querySelectorAll(".app-tag");
            if (tags.length) {
              tags[tags.length - 1].remove();
              sync();
            }
          }
        });

        container.addEventListener("click", function (e) {
          if (!e.target.closest(".app-tag-remove")) {
            input.focus();
          }
        });
      }
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initTagInputs);
  } else {
    initTagInputs();
  }
})();
