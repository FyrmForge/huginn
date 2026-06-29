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
