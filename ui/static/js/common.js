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
            return opts.onDone(contract, file, res && res.headers.get('X-Project-Id'));
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

    navigator.clipboard.writeText(link).then(function () {
      button.textContent = 'Link copied';
      setTimeout(function () { button.textContent = 'Copy link'; }, 2000);
    }, function () {
      window.prompt('Copy link', link);
    });
  });
})();
