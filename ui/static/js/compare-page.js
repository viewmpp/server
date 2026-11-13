(function () {
  'use strict';

  var root = document.getElementById('pair');
  if (!root || !window.MppCompare) { return; }

  var max = Number(root.dataset.maxUpload) || 0;
  var error = document.getElementById('pair-error');
  var chosen = { previous: null, current: null };
  var busy = false;

  Array.prototype.forEach.call(root.querySelectorAll('[data-slot]'), function (slot) {
    var key = slot.dataset.slot;
    var input = slot.querySelector('input[type="file"]');

    input.addEventListener('change', function () {
      pick(slot, key, input.files[0]);
    });

    slot.addEventListener('dragover', function (e) {
      e.preventDefault();
      slot.classList.add('is-over');
    });

    slot.addEventListener('dragleave', function () {
      slot.classList.remove('is-over');
    });

    slot.addEventListener('drop', function (e) {
      e.preventDefault();
      slot.classList.remove('is-over');
      if (e.dataTransfer && e.dataTransfer.files.length) {
        pick(slot, key, e.dataTransfer.files[0]);
      }
    });
  });

  document.querySelectorAll('[data-compare-example]').forEach(function (button) {
    button.addEventListener('click', function () {
      var pair = JSON.parse(button.dataset.compareExample);
      hideError();
      window.MppCompare.example(pair.previous, pair.previous_name, pair.current, pair.current_name);
    });
  });

  function pick(slot, key, file) {
    if (!file) { return; }
    hideError();

    if (max && file.size > max) {
      showError(file.name + ' is larger than ' + Math.round(max / 1048576) + ' MB.');
      return;
    }

    chosen[key] = file;
    slot.classList.add('is-chosen');
    slot.querySelector('[data-name]').textContent = file.name;

    if (chosen.previous && chosen.current) { start(); }
  }

  function start() {
    if (busy) { return; }
    busy = true;

    window.MppSignal('compare_from:page');
    window.MppCompare.pairFiles(chosen.previous, chosen.current).then(function () {
      busy = false;
    });
  }

  function showError(text) {
    error.textContent = text;
    error.classList.remove('is-hidden');
  }

  function hideError() {
    error.textContent = '';
    error.classList.add('is-hidden');
  }
})();
