(function () {
  'use strict';

  var app = document.getElementById('app');
  if (!app) { return; }

  var VOTED = 'mpp-wish-voted';
  var ASKED = 'mpp-export-asked';

  var viewer = app.dataset.viewer || 'file';

  function read(key) {
    try {
      return localStorage.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function write(key, value) {
    try {
      localStorage.setItem(key, value);
    } catch (e) {}
  }

  var signal = window.MppSignal;

  var voted = (read(VOTED) || '').split(',').filter(Boolean);

  document.querySelectorAll('[data-wish]').forEach(function (button) {
    var wish = button.dataset.wish;
    var dialog = document.getElementById(button.getAttribute('aria-controls'));

    button.addEventListener('click', function () {
      if (voted.indexOf(wish) === -1) {
        voted.push(wish);
        write(VOTED, voted.join(','));
        signal(wish + ':' + viewer);
      }

      if (dialog && typeof dialog.showModal === 'function') { dialog.showModal(); }
    });
  });

  var ask = document.getElementById('export-ask');
  if (!ask) { return; }

  var ANSWERED = 'answered';
  var MAX_SKIPS = 3;

  var asked = read(ASKED);
  var skips = parseInt(asked, 10) || 0;
  var done = asked === ANSWERED || skips >= MAX_SKIPS;

  document.addEventListener('mpp:exported', function () {
    if (done || typeof ask.showModal !== 'function') { return; }
    done = true;

    ask.returnValue = '';
    ask.showModal();
    signal('export_asked');
  });

  document.querySelectorAll('[data-export-link]').forEach(function (link) {
    link.addEventListener('click', function () {
      document.dispatchEvent(new CustomEvent('mpp:exported'));
    });
  });

  ask.addEventListener('close', function () {
    if (!ask.returnValue) {
      write(ASKED, String(skips + 1));
      return;
    }

    write(ASKED, ANSWERED);
    signal('export_purpose:' + ask.returnValue);
  });
})();
