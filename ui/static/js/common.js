(function () {
  'use strict';

  window.MppText = {
    tooLarge:   function (mb) { return 'File is larger than ' + mb + ' MB'; },
    serverSaid: function (status) { return 'server replied ' + status; }
  };

  window.MppExport = {
    xlsx: function (contract, fileName) {
      return fetch('/api/v1/xlsx?name=' + encodeURIComponent(fileName || ''), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(contract)
      }).then(function (response) {
        if (!response.ok) { throw new Error(window.MppText.serverSaid(response.status)); }
        return response.blob();
      }).then(function (blob) {
        var a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = (String(fileName || '').replace(/\.[^.]*$/, '') || 'project') + '.xlsx';
        a.click();
        URL.revokeObjectURL(a.href);
      });
    },

    save: function (contract, fileName) {
      return fetch('/api/v1/projects?name=' + encodeURIComponent(fileName || ''), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(contract)
      }).then(function (response) {
        if (!response.ok) {
          var error = new Error(window.MppText.serverSaid(response.status));
          error.status = response.status;
          throw error;
        }
        return response.json();
      });
    }
  };

  window.MppUpload = {
    bind: function (opts) {
      var drop = opts.drop;
      var input = opts.file;

      if (!drop) { return; }

      drop.addEventListener('dragover', function (e) {
        e.preventDefault();
        drop.classList.add('is-over');
      });

      drop.addEventListener('dragleave', function () {
        drop.classList.remove('is-over');
      });

      drop.addEventListener('drop', function (e) {
        e.preventDefault();
        drop.classList.remove('is-over');
        send(e.dataTransfer.files[0]);
      });

      if (input) {
        input.addEventListener('change', function () { send(input.files[0]); });
      }

      function send(file) {
        if (!file) { return; }

        var limit = Number(drop.dataset.maxUpload);
        if (limit && file.size > limit) {
          opts.onError(window.MppText.tooLarge(Math.round(limit / 1048576)));
          return;
        }

        opts.onStart();
        drop.classList.add('is-busy');

        var res = null;

        fetch('/api/v1/upload?name=' + encodeURIComponent(file.name), {
          method: 'POST',
          headers: { 'Content-Type': 'application/octet-stream' },
          body: file
        })
          .then(function (response) {
            res = response;
            return response.json().then(function (payload) {
              if (!response.ok) { throw new Error(payload || window.MppText.serverSaid(response.status)); }
              return payload;
            });
          })
          .then(function (contract) {
            return opts.onDone(contract, file, res && res.headers.get('X-Project-Id'), res && res.headers.get('X-Save-Refused'));
          })
          .catch(function (err) {
            opts.onError(err.message);
          })
          .then(function () {
            drop.classList.remove('is-busy');
            if (input) { input.value = ''; }
          });
      }
    }
  };

  document.addEventListener('dragover', function (e) { e.preventDefault(); });
  document.addEventListener('drop', function (e) { e.preventDefault(); });

  document.addEventListener('submit', function (event) {
    var message = event.target.getAttribute('data-confirm');
    if (message && !window.confirm(message)) {
      event.preventDefault();
    }
  });

  document.addEventListener('click', function (event) {
    var button = event.target.closest('#copy-link');
    if (!button) { return; }

    var link = location.origin + button.getAttribute('data-link');
    var label = button.querySelector('[data-label]') || button;

    navigator.clipboard.writeText(link).then(function () {
      label.textContent = 'Link copied';
      setTimeout(function () { label.textContent = 'Copy link'; }, 2000);
    }, function () {
      window.prompt('Copy link', link);
    });
  });

  function closeMenus(except) {
    document.querySelectorAll('[data-menu]').forEach(function (menu) {
      if (menu === except) { return; }
      menu.querySelector('[data-menu-panel]').classList.add('is-hidden');
      menu.querySelector('[data-menu-toggle]').setAttribute('aria-expanded', 'false');
    });
  }

  document.addEventListener('click', function (event) {
    var toggle = event.target.closest('[data-menu-toggle]');

    if (!toggle) {
      if (!event.target.closest('[data-menu-panel]')) { closeMenus(null); }
      return;
    }

    var menu = toggle.closest('[data-menu]');
    var panel = menu.querySelector('[data-menu-panel]');
    var open = panel.classList.contains('is-hidden');

    closeMenus(menu);
    panel.classList.toggle('is-hidden', !open);
    toggle.setAttribute('aria-expanded', String(open));
  });

  document.addEventListener('keydown', function (event) {
    if (event.key === 'Escape') { closeMenus(null); }
  });

  document.querySelectorAll('[data-share-form]').forEach(function (form) {
    var protect = form.querySelector('[data-share-protect]');
    var box = form.querySelector('[data-share-pass]');
    var field = box.querySelector('input');
    var access = form.querySelector('[data-share-access]');

    function sync() {
      var on = protect.checked;

      box.classList.toggle('is-hidden', !on);
      access.value = on ? 'protected' : 'public';

      if (on) {
        field.setAttribute('required', 'required');
        field.focus();
      } else {
        field.removeAttribute('required');
      }
    }

    protect.addEventListener('change', sync);

    if (protect.checked) { field.setAttribute('required', 'required'); }
  });

  document.querySelectorAll('[data-share-url]').forEach(function (input) {
    input.value = location.origin + input.value;
    input.addEventListener('focus', function () { input.select(); });
  });

  document.addEventListener('click', function (event) {
    var close = event.target.closest('[data-toast-close]');
    if (close) { close.parentNode.classList.add('is-hidden'); }
  });

  document.querySelectorAll('[data-reveal]').forEach(function (form) {
    var body = form.querySelector('[data-reveal-body]');
    var field = body.querySelector('input');
    var toggle = form.querySelector('[data-reveal-toggle]');
    var cancel = form.querySelector('[data-reveal-cancel]');
    var submit = form.querySelector('[data-reveal-submit]');

    function open(on) {
      body.classList.toggle('is-hidden', !on);
      toggle.classList.toggle('is-hidden', on);
      cancel.classList.toggle('is-hidden', !on);
      submit.classList.toggle('is-hidden', !on);

      if (on) {
        field.setAttribute('required', 'required');
        field.focus();
      } else {
        field.removeAttribute('required');
        field.value = '';
      }
    }

    toggle.addEventListener('click', function () { open(true); });
    cancel.addEventListener('click', function () { open(false); });
  });
})();
