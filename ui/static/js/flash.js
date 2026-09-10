(function () {
  'use strict';

  var LIFETIME = 15000;

  var stack = document.getElementById('notifications');
  if (!stack) { return; }

  var app = document.getElementById('app');
  var workspace = app && app.querySelector('.workspace');
  var notices = [];

  document.querySelectorAll('[data-viewer-notice]').forEach(function (box) {
    stack.appendChild(box);
  });

  function place() {
    var viewing = !!(app && !app.classList.contains('is-hidden'));
    var parent = viewing ? workspace : document.body;
    if (stack.parentNode !== parent) { parent.appendChild(stack); }
    stack.classList.toggle('is-viewer', viewing);
    notices.forEach(function (sync) { sync(false); });
  }

  place();

  stack.querySelectorAll('.toast').forEach(function (box) {
    var progress = box.querySelector('[data-flash-progress]');
    var remaining = LIFETIME;
    var started = null;
    var frame = null;
    var hovered = box.matches(':hover');
    var focused = box.contains(document.activeElement);
    var wasVisible = false;

    function stop() {
      cancelAnimationFrame(frame);
      started = null;
    }

    function dismiss() {
      stop();
      box.classList.add('is-hidden');
    }

    function update() {
      if (started !== null) {
        var now = performance.now();
        remaining = Math.max(0, remaining - (now - started));
        started = now;
      }
      progress.style.transform = 'scaleX(' + remaining / LIFETIME + ')';
    }

    function tick() {
      update();
      if (remaining === 0) { dismiss(); }
      else if (started !== null) { frame = requestAnimationFrame(tick); }
    }

    function sync(reset) {
      update();
      stop();
      var visible = !box.classList.contains('is-hidden') &&
        (!box.hasAttribute('data-viewer-notice') || stack.classList.contains('is-viewer'));
      if (visible && (!wasVisible || reset)) {
        remaining = LIFETIME;
        progress.style.transform = 'scaleX(1)';
      }
      wasVisible = visible;
      var hasAction = !!box.querySelector('.toast__surface .btn:not(.is-hidden)');
      progress.parentElement.hidden = hasAction;
      if (hasAction) {
        remaining = LIFETIME;
        progress.style.transform = 'scaleX(1)';
        return;
      }
      if (visible && remaining === 0) { dismiss(); return; }
      if (!visible || hovered || focused || document.hidden || box.querySelector('[aria-busy="true"]:not(.is-hidden)')) { return; }
      started = performance.now();
      frame = requestAnimationFrame(tick);
    }

    box.addEventListener('mouseenter', function () { hovered = true; sync(false); });
    box.addEventListener('mouseleave', function () { hovered = false; sync(false); });
    box.addEventListener('focusin', function () { focused = true; sync(false); });
    box.addEventListener('focusout', function (event) {
      focused = box.contains(event.relatedTarget);
      sync(false);
    });
    box.querySelector('.toast__close').addEventListener('click', dismiss);
    document.addEventListener('visibilitychange', function () { sync(false); });

    new MutationObserver(function (records) {
      sync(records.some(function (record) {
        return record.type === 'childList' || record.type === 'characterData';
      }));
    }).observe(box, { attributes: true, attributeFilter: ['class', 'aria-busy'], childList: true, characterData: true, subtree: true });

    notices.push(sync);
    sync(false);
  });

  if (app) {
    new MutationObserver(place).observe(app, { attributes: true, attributeFilter: ['class'] });
  }
})();
