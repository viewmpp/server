(function () {
  'use strict';

  var box = document.getElementById('send-cta');
  if (!box) { return; }

  var KEY = 'mpp-send-dismissed';
  var DELAY = 20000;

  try {
    if (localStorage.getItem(KEY) === '1') { return; }
  } catch (e) { /* private mode: the prompt simply reappears */ }

  var shown = false;

  function show() {
    if (shown) { return; }
    shown = true;

    box.classList.remove('is-hidden');
    document.removeEventListener('mpp:task', show);
  }

  function dismiss() {
    box.classList.add('is-hidden');

    try {
      localStorage.setItem(KEY, '1');
    } catch (e) { /* nothing to remember it in */ }
  }

  document.getElementById('send-cta-close').addEventListener('click', dismiss);

  document.addEventListener('mpp:task', show);
  setTimeout(show, DELAY);
})();
