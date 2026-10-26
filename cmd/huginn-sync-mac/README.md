# huginn-sync-mac

One-way push of all Calendar.app calendars (local, iCloud, Google, …) to huginn.

1. In huginn: Settings → Devices → create an app password.
2. Build (needs Xcode Command Line Tools):
   ```
   swiftc -O sync.swift -o huginn-sync \
     -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist -Xlinker Info.plist
   sudo cp huginn-sync /usr/local/bin/
   ```
3. Run once from Terminal to grant calendar access (macOS prompts once):
   ```
   HUGINN_URL=https://huginn.example.com HUGINN_TOKEN=... huginn-sync
   ```
4. Schedule every 15 min: edit URL/token in `com.fyrmforge.huginn-sync.plist`, then
   ```
   cp com.fyrmforge.huginn-sync.plist ~/Library/LaunchAgents/
   launchctl load ~/Library/LaunchAgents/com.fyrmforge.huginn-sync.plist
   ```
   Log: `/tmp/huginn-sync.log`.

Window: 1 year back, 2 years ahead (`HUGINN_PAST_DAYS` / `HUGINN_FUTURE_DAYS`).
Synced calendars show up in huginn read-only; events removed on the Mac are removed on huginn.
