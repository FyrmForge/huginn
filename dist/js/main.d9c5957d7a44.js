// HTMX event handlers for huginn.

// Log HTMX errors to the console.
document.addEventListener('htmx:responseError', function(evt) {
    console.error('HTMX request failed:', evt.detail.xhr.status, evt.detail.xhr.statusText);
});

// Re-initialize components after HTMX swaps.
document.addEventListener('htmx:afterSwap', function(evt) {
    // Scroll to the first validation error if present.
    var firstError = evt.detail.target.querySelector('.field-error');
    if (firstError && firstError.textContent.trim() !== '') {
        firstError.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
});

// Handle HTMX send errors (network failures).
document.addEventListener('htmx:sendError', function(evt) {
    console.error('HTMX network error for:', evt.detail.elt);
});

// Scroll week time grid to the earliest event of the week (8am fallback), on
// load and after HTMX swaps.
function scrollWeekGrid() {
    var el = document.getElementById('week-time-grid');
    if (!el) return;
    var min = Infinity;
    el.querySelectorAll('[data-event-cal]').forEach(function(ev) {
        var t = parseInt(ev.style.top, 10);
        if (!isNaN(t) && t < min) min = t;
    });
    el.scrollTop = (min === Infinity) ? 480 : Math.max(0, min - 24);
}
document.addEventListener('DOMContentLoaded', scrollWeekGrid);
document.addEventListener('htmx:afterSwap', scrollWeekGrid);

// Open the native month/date picker when its transparent overlay is clicked.
document.addEventListener('click', function(evt) {
    var inp = evt.target.closest('input[data-jump-picker]');
    if (inp && typeof inp.showPicker === 'function') {
        try { inp.showPicker(); } catch (e) { /* unsupported browser: falls back to default */ }
    }
});

// Grafana-style time crosshair: a horizontal line across the whole week that
// follows the cursor, with the time shown in the hour gutter.
document.addEventListener('mousemove', function(evt) {
    var grid = document.getElementById('week-time-grid');
    var cross = document.getElementById('week-crosshair');
    if (!grid || !cross) return;
    if (!grid.contains(evt.target)) { cross.classList.add('hidden'); return; }
    var rect = grid.getBoundingClientRect();
    var y = evt.clientY - rect.top + grid.scrollTop;
    if (y < 0 || y > 1440) { cross.classList.add('hidden'); return; }
    cross.classList.remove('hidden');
    cross.style.top = y + 'px';
    var label = document.getElementById('week-crosshair-label');
    if (label) {
        var h = Math.floor(y / 60), m = Math.floor(y % 60);
        label.textContent = (h < 10 ? '0' : '') + h + ':' + (m < 10 ? '0' : '') + m;
    }
});

// All-day toggle: switch datetime-local ↔ date inputs.
document.addEventListener('change', function(evt) {
    var cb = evt.target.closest('[data-allday-toggle]');
    if (!cb) return;
    var allDay = cb.checked;
    var fields = cb.closest('form').querySelectorAll('[name="start_at"], [name="end_at"]');
    fields.forEach(function(input) {
        var val = input.value;
        if (allDay) {
            input.type = 'date';
            input.value = val.length >= 10 ? val.slice(0, 10) : val;
        } else {
            input.type = 'datetime-local';
            input.value = val.length === 10 ? val + 'T00:00' : val;
        }
    });
});

// Calendar visibility toggles.
document.addEventListener('change', function(evt) {
    var cb = evt.target.closest('[data-cal-toggle]');
    if (!cb) return;
    var calID = cb.dataset.calToggle;
    var visible = cb.checked;
    document.querySelectorAll('[data-event-cal="' + calID + '"]').forEach(function(el) {
        el.style.display = visible ? '' : 'none';
    });
    var dot = cb.closest('label').querySelector('span');
    if (dot) dot.style.opacity = visible ? '1' : '0.3';
});

// Recurrence picker: toggle panel.
document.addEventListener('change', function(evt) {
    var cb = evt.target.closest('[data-recur-toggle]');
    if (!cb) return;
    var panel = document.getElementById('recur-fields');
    if (panel) panel.classList.toggle('hidden', !cb.checked);
});

// Recurrence picker: freq change shows/hides byday row.
document.addEventListener('change', function(evt) {
    var sel = evt.target.closest('[data-recur-freq]');
    if (!sel) return;
    var byday = document.getElementById('recur-byday');
    if (byday) byday.classList.toggle('hidden', sel.value !== 'WEEKLY');
});

// Recurrence picker: day pill toggle (updates hidden input).
document.addEventListener('click', function(evt) {
    var pill = evt.target.closest('[data-day]');
    if (!pill) return;
    var active = pill.dataset.dayActive === 'true';
    active = !active;
    pill.dataset.dayActive = active ? 'true' : 'false';
    if (active) {
        pill.classList.replace('bg-huginn-panel', 'bg-huginn-accent');
        pill.classList.replace('text-huginn-mute', 'text-huginn-bg');
    } else {
        pill.classList.replace('bg-huginn-accent', 'bg-huginn-panel');
        pill.classList.replace('text-huginn-bg', 'text-huginn-mute');
    }
    // Rebuild hidden input from active pills.
    var days = [];
    document.querySelectorAll('[data-day][data-day-active="true"]').forEach(function(p) {
        days.push(p.dataset.day);
    });
    var inp = document.getElementById('recur-byday-input');
    if (inp) inp.value = days.join(',');
});

// Recurrence picker: end-type radio enables/disables associated inputs.
document.addEventListener('change', function(evt) {
    var radio = evt.target.closest('[data-recur-end]');
    if (!radio) return;
    var val = radio.value;
    var untilInp = document.getElementById('recur-until-input');
    var countInp = document.getElementById('recur-count-input');
    if (untilInp) untilInp.classList.toggle('opacity-50', val !== 'until');
    if (untilInp) untilInp.classList.toggle('pointer-events-none', val !== 'until');
    if (countInp) countInp.classList.toggle('opacity-50', val !== 'count');
    if (countInp) countInp.classList.toggle('pointer-events-none', val !== 'count');
});
