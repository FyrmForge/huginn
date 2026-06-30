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

// --ppm = rendered pixels per minute on the week timeline (1 = default 60px/hr).
function weekPpm() {
    var el = document.getElementById('week-time-grid');
    if (!el) return 1;
    var v = parseFloat(getComputedStyle(el).getPropertyValue('--ppm'));
    return (v && v > 0) ? v : 1;
}

// Apply the stored timeline scale (drag-to-compress) to the grid.
function applyWeekScale() {
    var el = document.getElementById('week-time-grid');
    if (!el) return;
    var stored = parseFloat(sessionStorage.getItem('weekPpm'));
    if (stored && stored > 0) el.style.setProperty('--ppm', stored);
}

// Scroll week time grid to the earliest event of the week (8am fallback), on
// load and after HTMX swaps. Uses rendered geometry so it works at any scale.
function scrollWeekGrid() {
    var el = document.getElementById('week-time-grid');
    if (!el) return;
    var gridTop = el.getBoundingClientRect().top;
    var min = Infinity;
    el.querySelectorAll('[data-event-cal]').forEach(function(ev) {
        var top = ev.getBoundingClientRect().top - gridTop + el.scrollTop;
        if (top < min) min = top;
    });
    el.scrollTop = (min === Infinity) ? 480 * weekPpm() : Math.max(0, min - 24);
}
function initWeekGrid() { applyWeekScale(); scrollWeekGrid(); }
document.addEventListener('DOMContentLoaded', initWeekGrid);
document.addEventListener('htmx:afterSwap', initWeekGrid);

// Month cells render every chip; collapse only the ones that actually overflow
// the cell's height into a "+N more" indicator. Re-runs on load, swap, resize.
function chipData(c) {
    return {
        title: c.getAttribute('data-ev-title'),
        time: c.getAttribute('data-ev-time'),
        loc: c.getAttribute('data-ev-loc'),
        desc: c.getAttribute('data-ev-desc'),
        cal: c.getAttribute('data-ev-cal-name'),
        color: c.getAttribute('data-ev-cal-color')
    };
}
function fitDayCells() {
    var grid = document.getElementById('cal-grid');
    if (!grid) return;
    grid.querySelectorAll('[data-chips]').forEach(function(wrap) {
        var chips = Array.prototype.filter.call(wrap.children, function(c) { return c.hasAttribute('data-ev-card'); });
        chips.forEach(function(c) { c.classList.remove('hidden'); });
        var old = wrap.querySelector('[data-more-card]');
        if (old) old.remove();
        if (chips.length < 2) return;
        var avail = wrap.clientHeight;
        var chipH = chips[0].offsetHeight + 2; // include the row gap
        if (chipH <= 2) return;
        var fit = Math.floor(avail / chipH);
        if (chips.length <= fit) return;       // everything fits — no "+N more"
        var show = Math.max(1, fit - 1);       // leave one row for the indicator
        var hidden = chips.slice(show);
        hidden.forEach(function(c) { c.classList.add('hidden'); });
        var btn = document.createElement('button');
        btn.className = 'shrink-0 text-[10px] text-left text-huginn-mute hover:text-huginn-accent pl-1.5 truncate transition-colors';
        btn.textContent = '+' + hidden.length + ' more';
        btn.setAttribute('data-more-card', '');
        btn.setAttribute('data-more', JSON.stringify(hidden.map(chipData)));
        var cell = wrap.closest('[data-date]');
        if (cell) btn.setAttribute('data-date', cell.getAttribute('data-date'));
        wrap.appendChild(btn);
    });
}
var fitTimer = null;
document.addEventListener('DOMContentLoaded', fitDayCells);
document.addEventListener('htmx:afterSwap', fitDayCells);
window.addEventListener('resize', function() { clearTimeout(fitTimer); fitTimer = setTimeout(fitDayCells, 80); });

// Clicking "+N more" jumps to that week (capture phase stops the cell's new-event).
document.addEventListener('click', function(e) {
    var b = e.target.closest('[data-more-card]');
    if (!b || !b.getAttribute('data-date')) return;
    e.preventDefault();
    e.stopPropagation();
    var d = new Date(b.getAttribute('data-date') + 'T00:00:00');
    var wd = d.getDay() || 7;
    d.setDate(d.getDate() - (wd - 1));
    var monday = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
    if (window.htmx) {
        window.htmx.ajax('GET', '/week/grid?week=' + monday, { target: '#cal-grid', swap: 'outerHTML' });
        history.pushState(null, '', '/week?week=' + monday);
    }
}, true);

// Drag the hour gutter vertically to expand/compress the timeline.
(function() {
    var dragging = false, startY = 0, startPpm = 1, grid = null;
    function clamp(v) { return Math.max(0.25, Math.min(3, v)); }
    document.addEventListener('mousedown', function(e) {
        if (!e.target.closest('#week-hour-gutter')) return;
        grid = document.getElementById('week-time-grid');
        if (!grid) return;
        dragging = true;
        startY = e.clientY;
        startPpm = weekPpm();
        document.body.style.userSelect = 'none';
        e.preventDefault();
    });
    document.addEventListener('mousemove', function(e) {
        if (!dragging || !grid) return;
        var ppm = clamp(startPpm + (e.clientY - startY) * 0.004); // drag down = expand
        grid.style.setProperty('--ppm', ppm);
    });
    document.addEventListener('mouseup', function() {
        if (!dragging) return;
        dragging = false;
        document.body.style.userSelect = '';
        sessionStorage.setItem('weekPpm', weekPpm());
    });
})();

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
    var ppm = weekPpm();
    if (y < 0 || y > 1440 * ppm) { cross.classList.add('hidden'); return; }
    cross.classList.remove('hidden');
    cross.style.top = y + 'px';
    var label = document.getElementById('week-crosshair-label');
    if (label) {
        var mins = y / ppm;
        var h = Math.floor(mins / 60), m = Math.floor(mins % 60);
        label.textContent = (h < 10 ? '0' : '') + h + ':' + (m < 10 ? '0' : '') + m;
    }
});

// Hover detail cards (single event + "+N more" stack share one card design).
(function() {
    // Build one event "card block" (title, time, location, description, calendar).
    function buildEventBlock(d) {
        var block = document.createElement('div');
        function line(cls, val) {
            if (!val) return;
            var el = document.createElement('div');
            el.className = cls;
            el.textContent = val;
            block.appendChild(el);
        }
        line('text-xs font-medium text-huginn-fg mb-1 break-words', d.title);
        line('text-[10px] text-huginn-accent mb-1', d.time);
        line('text-[10px] text-huginn-dim mb-1 break-words', d.loc);
        line('text-[10px] text-huginn-mute break-words whitespace-pre-line', d.desc);
        if (d.cal) {
            var row = document.createElement('div');
            row.className = 'flex items-center gap-1.5 text-[10px] text-huginn-dim mt-2 pt-2 border-t border-huginn-line';
            var dot = document.createElement('span');
            dot.className = 'w-2 h-2 rounded-full shrink-0';
            dot.style.backgroundColor = d.color || '';
            var name = document.createElement('span');
            name.className = 'truncate';
            name.textContent = d.cal;
            row.appendChild(dot);
            row.appendChild(name);
            block.appendChild(row);
        }
        return block;
    }
    function position(card, e) {
        var pad = 14, w = card.offsetWidth, h = card.offsetHeight;
        var x = e.clientX + pad, y = e.clientY + pad;
        if (x + w > window.innerWidth - 8) x = e.clientX - w - pad;
        if (y + h > window.innerHeight - 8) y = e.clientY - h - pad;
        card.style.left = Math.max(8, x) + 'px';
        card.style.top = Math.max(8, y) + 'px';
    }

    // Single event card.
    var evCard = null, evCur = null;
    // "+N more" stacked card.
    var moreCard = null, moreCur = null;

    document.addEventListener('mousemove', function(e) {
        // "+N more" stack takes priority (the button sits among event chips).
        var more = e.target.closest('[data-more-card]');
        if (more) {
            if (evCard && evCur) { evCard.classList.add('hidden'); evCur = null; }
            moreCard = moreCard || document.getElementById('more-card');
            if (!moreCard) return;
            if (more !== moreCur) {
                moreCur = more;
                var items = [];
                try { items = JSON.parse(more.getAttribute('data-more') || '[]'); } catch (err) {}
                moreCard.innerHTML = '';
                items.forEach(function(it, i) {
                    var block = buildEventBlock(it);
                    if (i > 0) block.className = 'mt-3 pt-3 border-t border-huginn-line';
                    moreCard.appendChild(block);
                });
                moreCard.classList.remove('hidden');
            }
            position(moreCard, e);
            return;
        }
        if (moreCard && moreCur) { moreCard.classList.add('hidden'); moreCur = null; }

        var el = e.target.closest('[data-ev-card]');
        if (!el) {
            if (evCard && evCur) { evCard.classList.add('hidden'); evCur = null; }
            return;
        }
        evCard = evCard || document.getElementById('ev-card');
        if (!evCard) return;
        if (el !== evCur) {
            evCur = el;
            evCard.innerHTML = '';
            evCard.appendChild(buildEventBlock({
                title: el.getAttribute('data-ev-title'),
                time: el.getAttribute('data-ev-time'),
                loc: el.getAttribute('data-ev-loc'),
                desc: el.getAttribute('data-ev-desc'),
                cal: el.getAttribute('data-ev-cal-name'),
                color: el.getAttribute('data-ev-cal-color')
            }));
            evCard.classList.remove('hidden');
        }
        position(evCard, e);
    });
})();

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
