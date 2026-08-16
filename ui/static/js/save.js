(function () {
  'use strict';

  var box = document.getElementById('save-cta');
  if (!box) { return; }

  var text = document.getElementById('save-cta-text');
  var signup = document.getElementById('save-cta-signup');
  var save = document.getElementById('save-cta-save');

  var loaded = null;
  var loadedName = '';

  var KEEP = 'Keep this plan to open it later or share a link';
  var BACK = 'Created your account? Now save it';
  var AGAIN = 'Finish creating your account, then save';

  document.addEventListener('mpp:loaded', function (event) {
    loaded = event.detail.contract;
    loadedName = event.detail.fileName;
    offer();
  });

  signup.addEventListener('click', function () {
    text.textContent = BACK;
    signup.classList.add('is-hidden');
    save.classList.remove('is-hidden');
  });

  save.addEventListener('click', function () {
    if (!loaded) { return; }

    save.disabled = true;

    window.MppExport.save(loaded, loadedName)
      .then(saved)
      .catch(function (err) {
        save.disabled = false;

        if (err.status === 401) {
          text.textContent = AGAIN;
          signup.classList.remove('is-hidden');
          return;
        }

        text.textContent = err.message;
      });
  });

  function offer() {
    text.textContent = KEEP;
    signup.classList.remove('is-hidden');
    save.classList.add('is-hidden');
    save.disabled = false;
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
  }
})();
