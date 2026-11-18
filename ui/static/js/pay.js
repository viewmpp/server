(function () {
  'use strict';

  var box = document.getElementById('pay');
  if (!box) { return; }

  var status = document.getElementById('pay-status');
  var data = box.dataset;
  var paid = false;

  if (!window.Paddle) {
    status.textContent = 'The payment form could not be loaded. Check that nothing in the browser blocks paddle.com, then try again.';
    return;
  }

  if (data.sandbox === 'true') { window.Paddle.Environment.set('sandbox'); }

  window.Paddle.Initialize({
    token: data.token,
    checkout: {
      settings: { displayMode: 'overlay', theme: 'light', locale: 'en', successUrl: data.success }
    },
    eventCallback: function (event) {
      if (!event) { return; }
      if (event.name === 'checkout.completed') { paid = true; }
      if (event.name === 'checkout.closed' && !paid) { window.location.assign(data.back); }
    }
  });

  if (data.transaction) { return; }

  window.Paddle.Checkout.open({
    items: [{ priceId: data.price, quantity: 1 }],
    customer: { email: data.email },
    customData: { user_id: data.user, site: data.site }
  });
})();
