// Nav drawer toggle for huginn AppLayout.
(function () {
  const drawer  = document.getElementById('nav-drawer');
  const overlay = document.getElementById('nav-overlay');
  if (!drawer) return;

  function open()  { drawer.classList.remove('-translate-x-full'); overlay.removeAttribute('hidden'); }
  function close() { drawer.classList.add('-translate-x-full');    overlay.setAttribute('hidden', ''); }

  document.querySelectorAll('[data-nav-toggle]').forEach(el => el.addEventListener('click', open));
  document.querySelectorAll('[data-nav-close]').forEach(el  => el.addEventListener('click', close));
  overlay.addEventListener('click', close);

  // Close on nav link click (SPA feel with HTMX).
  document.querySelectorAll('[data-nav-link]').forEach(el => el.addEventListener('click', close));
})();

// Calendars drawer (mobile). Toggle button lives inside the htmx-swapped grid,
// so open is delegated on document; drawer + overlay persist at page level.
(function () {
  const drawer  = document.getElementById('cal-sidebar');
  const overlay = document.getElementById('cal-overlay');
  if (!drawer || !overlay) return;

  function open()  { drawer.classList.remove('-translate-x-full'); overlay.removeAttribute('hidden'); }
  function close() { drawer.classList.add('-translate-x-full');    overlay.setAttribute('hidden', ''); }

  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-cal-drawer-open]'))  return open();
    if (e.target.closest('[data-cal-drawer-close]')) return close();
  });
})();

// Modal backdrop click-to-close. The modal is htmx-swapped into #modal-root, so
// the handler is delegated on document. Only a click on the backdrop itself
// (not its children) closes — matches the old hx-on:click, without needing
// 'unsafe-eval' in the CSP.
(function () {
  document.addEventListener('click', function (e) {
    if (e.target.matches('[data-modal-backdrop]')) {
      htmx.ajax('GET', '/events/close', { target: '#modal-root', swap: 'innerHTML' });
    }
  });
})();
