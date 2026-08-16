(function () {
  'use strict';

  var box = document.getElementById('save-cta');
  if (!box) { return; }

  var text = document.getElementById('save-cta-text');
  var signup = document.getElementById('save-cta-signup');
  var save = document.getElementById('save-cta-save');
  var manage = document.getElementById('save-cta-manage');

  var signedIn = box.dataset.signedIn === '1';

  var loaded = null;
  var loadedName = '';

  var KEEP = 'Keep this plan to open it later or share a link';
  var BACK = 'Created your account? Now save it';
  var AGAIN = 'Finish creating your account, then save';
  var FULL = 'Not saved - your free account is full. Delete a plan, or go Pro to save without a limit.';

  document.addEventListener('mpp:loaded', function (event) {
    loaded = event.detail.contract;
    loadedName = event.detail.fileName;

    if (signedIn) {
      if (event.detail.refused === 'limit') {
        only(manage, FULL);
      } else {
        box.classList.add('is-hidden');
      }
      return;
    }

    only(signup, KEEP);
  });

  signup.addEventListener('click', function () {
    only(save, BACK);
  });

  save.addEventListener('click', function () {
    if (!loaded) { return; }

    save.disabled = true;

    window.MppExport.save(loaded, loadedName)
      .then(saved)
      .catch(function (err) {
        save.disabled = false;

        if (err.status === 401) {
          only(signup, AGAIN);
          return;
        }

        if (err.status === 409) {
          only(manage, FULL);
          return;
        }

        text.textContent = err.message;
      });
  });

  function only(action, message) {
    text.textContent = message;

    [signup, save, manage].forEach(function (el) {
      el.classList.toggle('is-hidden', el !== action);
    });

    box.classList.remove('is-hidden');
  }

  function saved(result) {
    box.textContent = '';

    var done = document.createElement('span');
    done.className = 'offer__text';
    done.textContent = 'Saved to your projects.';

    var open = document.createElement('a');
    open.className = 'btn';
    open.href = '/p/' + result.id;
    open.textContent = 'Open it';

    box.appendChild(done);
    box.appendChild(open);
    box.classList.remove('is-hidden');
  }
})();
