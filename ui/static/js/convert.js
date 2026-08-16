(function () {
  'use strict';

  var panel = document.getElementById('convert');
  if (!panel) { return; }

  var button = document.getElementById('convert-download');
  var offer = document.getElementById('convert-offer');
  var signedIn = panel.dataset.signedIn === '1';

  var loaded = null;
  var loadedName = '';

  var toolbarExport = document.getElementById('export-xlsx');
  if (toolbarExport) { toolbarExport.remove(); }

  var toolbarOffer = document.getElementById('save-cta');
  if (toolbarOffer) { toolbarOffer.remove(); }

  document.addEventListener('mpp:loaded', function (event) {
    loaded = event.detail.contract;
    loadedName = event.detail.fileName;

    panel.classList.remove('is-hidden');
    offer.classList.add('is-hidden');
  });

  button.addEventListener('click', function () {
    if (!loaded) { return; }

    button.disabled = true;

    window.MppExport.xlsx(loaded, loadedName)
      .then(showOffer)
      .catch(function (err) { offer.textContent = err.message; offer.classList.remove('is-hidden'); })
      .then(function () { button.disabled = false; });
  });

  function showOffer() {
    offer.textContent = '';
    offer.classList.remove('is-hidden');

    if (!signedIn) {
      offer.innerHTML = 'Want to keep this plan and share it by link? ' +
        '<a href="/signup">Create a free account</a>';
      return;
    }

    var ask = document.createElement('button');
    ask.className = 'linkish';
    ask.type = 'button';
    ask.textContent = 'Save this plan to your projects';

    ask.addEventListener('click', function () {
      ask.disabled = true;

      window.MppExport.save(loaded, loadedName)
        .then(function (result) {
          offer.textContent = 'Saved. ';
          var link = document.createElement('a');
          link.href = '/p/' + result.id;
          link.textContent = 'Open it';
          offer.appendChild(link);
        })
        .catch(function (err) { offer.textContent = err.message; });
    });

    offer.appendChild(document.createTextNode('Keep it for later? '));
    offer.appendChild(ask);
  }
})();
