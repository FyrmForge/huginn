# Huginn Design

Huginn is a companion to Muninn. It shares the same visual language — same
base palette, same mono font, same compact terminal feel — but carries its
own accent color so the two apps are visually distinct when open side by side.

---

## Accent color options

Pick one. Everything else stays identical to Muninn.

| Option | Name    | Hex       | Feel                              |
|--------|---------|-----------|-----------------------------------|
| A      | Indigo  | `#4f8ef7` | Classic calendar blue             |
| B      | Teal    | `#2dd4bf` | Fresh, distinct from Muninn purple|
| C      | Amber   | `#f5a623` | Warm, fits the FyrmForge fire theme|

Once chosen, replace every `muninn-accent` reference below with the chosen
hex and name it `huginn-accent`.

---

## Base palette (identical to Muninn)

```
huginn-bg:           #0a0a0a
huginn-surface:      #111114
huginn-panel:        #16161b
huginn-line:         #1f1f26
huginn-mute:         #5a5a66
huginn-dim:          #7c7c87
huginn-fg:           #d6d6d9
huginn-hi:           #f4f4f5
huginn-danger:       #f24d6d
huginn-ok:           #7dd181
huginn-accent:       <chosen above>
huginn-accent-soft:  <darken accent ~30%>
huginn-accent-glow:  <lighten accent ~15%>
```

---

## Typography

- Font: JetBrains Mono (same Google Fonts import as Muninn)
- Base size: `text-xs` / `text-sm` — compact, terminal feel
- All UI text is monospace; no sans-serif body font

---

## App shell

Huginn uses the same two-layout pattern as Muninn:

**Layout** — bare shell for auth pages (login).

**AppLayout** — authenticated shell:
- Fixed topbar (`h-10`) with: nav toggle `≡`, brand crumb `huginn › [user]`
- Slide-in left nav drawer (same structure as Muninn's NavDrawer)
- `<main>` scrolls independently

Nav groups for Huginn:

```
calendar
  /          (month view)
  /week      (week view)
  /day       (day view)
  /agenda    (agenda view)

calendars
  /calendars          (list + manage)
  /calendars/new      (create)

import & export
  /import
  /export

system
  /settings
  /settings/devices   (CalDAV / app passwords)
```

No chat drawer — Huginn has no AI features in scope yet.

---

## Component conventions

Same patterns as Muninn:

- `btn-ghost` — borderless icon/text button, hover reveals accent
- `btn-primary` — accent-background action button
- `chip` — inline `<kbd>`-style label
- `input` — `bg-huginn-panel border-huginn-line` text input
- Active nav link: accent left border + accent text + panel bg
- Flash messages: same position and alert classes

---

## Calendar grid

The calendar month/week/day grid is the one place custom JS is expected
(per plan). Keep it minimal — CSS grid for layout, JS only for drag/resize
if added later (Phase 7).

Grid color conventions:
- Today highlight: accent at low opacity (`/15`)
- Event chips: accent bg at `/20`, accent text
- Other-month days: `huginn-mute` text
- Weekend columns: subtle `huginn-panel` bg tint
