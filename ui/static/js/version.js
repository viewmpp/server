(function () {
  'use strict';

  var app = document.getElementById('app');
  if (!app || !app.dataset.projectId) { return; }

  var project = app.dataset.projectId;
  var viewer = app.dataset.viewer || 'file';

  var upload = document.getElementById('version-upload');
  var input = document.getElementById('version-file');
  var banner = document.getElementById('version-banner');
  var changes = document.getElementById('version-changes');

  var UPLOAD_LABEL = upload ? upload.textContent : '';
  var DIFFERENT = 'This file shares few tasks with the plan behind this link. ' +
    'Replace the plan with it anyway? Everyone with the link will see the new file.';

  if (upload && input) {
    upload.addEventListener('click', function () {
      input.value = '';
      input.click();
    });

    input.addEventListener('change', function () {
      var file = input.files[0];
      if (!file) { return; }

      upload.disabled = true;
      upload.textContent = 'Uploading ' + file.name + '...';

      parse(file)
        .then(function (contract) { return send(contract, file.name, false); })
        .then(function () { window.location.reload(); })
        .catch(function (err) {
          upload.disabled = false;
          upload.textContent = UPLOAD_LABEL;
          if (err) { window.alert(err.message); }
        });
    });
  }

  if (banner && changes) {
    changes.addEventListener('click', function () {
      window.MppSignal('version_changes_open:' + viewer);
      changes.disabled = true;

      fetch('/api/v1/projects/' + encodeURIComponent(project) + '/previous')
        .then(function (response) {
          if (!response.ok) { return window.MppRefusal(response); }
          return response.json();
        })
        .then(function (previous) {
          return window.MppCompare.show(previous, banner.dataset.previousName);
        })
        .catch(function (err) { window.alert(err.message); })
        .then(function () { changes.disabled = false; });
    });
  }

  function parse(file) {
    return fetch('/api/v1/upload?store=no&for=version&name=' + encodeURIComponent(file.name), {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream', 'X-Requested-With': 'viewmpp' },
      body: file
    }).then(function (response) {
      if (!response.ok) { return window.MppRefusal(response); }
      return response.json();
    });
  }

  function send(contract, name, force) {
    var url = '/api/v1/projects/' + encodeURIComponent(project) + '/versions?name=' + encodeURIComponent(name) + (force ? '&force=1' : '');

    return fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'viewmpp' },
      body: JSON.stringify(contract)
    }).then(function (response) {
      if (response.status === 409 && !force) {
        if (window.confirm(DIFFERENT)) { return send(contract, name, true); }
        return Promise.reject(null);
      }
      if (!response.ok) { return window.MppRefusal(response); }
      return null;
    });
  }
})();
