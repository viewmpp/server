(function () {
  'use strict';

  var KEY = 'mpp-theme';
  var MODES = ['light', 'dark', 'system'];

  var root = document.documentElement;
  var media = window.matchMedia('(prefers-color-scheme: dark)');

  function stored() {
    var value;
    try {
      value = localStorage.getItem(KEY);
    } catch (e) {
      value = null;
    }
    return MODES.indexOf(value) === -1 ? 'system' : value;
  }

  function resolve(mode) {
    if (mode === 'system') {
      return media.matches ? 'dark' : 'light';
    }
    return mode;
  }

  function paint(mode) {
    var theme = resolve(mode);

    root.setAttribute('data-theme', theme);
    root.setAttribute('data-gantt-theme', theme === 'dark' ? 'dark' : 'terrace');
  }

  window.MppTheme = {
    get: stored,
    set: function (mode) {
      if (MODES.indexOf(mode) === -1) { return; }

      try {
        localStorage.setItem(KEY, mode);
      } catch (e) {}

      apply(mode);
    }
  };

  function apply(mode) {
    var still = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    if (!document.startViewTransition || still) {
      paint(mode);
      mark(mode);
      return;
    }

    document.startViewTransition(function () {
      paint(mode);
      mark(mode);
    });
  }

  function mark(mode) {
    var buttons = document.querySelectorAll('[data-theme-set]');

    for (var i = 0; i < buttons.length; i++) {
      var button = buttons[i];
      var active = button.dataset.themeSet === mode;

      button.classList.toggle('is-active', active);
      button.setAttribute('aria-pressed', active ? 'true' : 'false');
    }
  }

  media.addEventListener('change', function () {
    if (stored() === 'system') { paint('system'); }
  });

  document.addEventListener('DOMContentLoaded', function () {
    mark(stored());

    document.addEventListener('click', function (event) {
      if (!event.target || !event.target.closest) { return; }

      var button = event.target.closest('[data-theme-set]');
      if (button) { window.MppTheme.set(button.dataset.themeSet); }
    });
  });
})();
