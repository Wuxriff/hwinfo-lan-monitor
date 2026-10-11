# HWiNFO LAN Monitor

**English** | [Русский](README.ru.md)

Watch your PC sensors from your phone. HWiNFO LAN Monitor reads sensor data from [HWiNFO](https://www.hwinfo.com/) and serves it as a fast web page on your local network. Glance at temperatures, clocks and fan speeds on a phone or tablet without leaving your game or minimizing anything.

![Dashboard on a phone: dark and light themes, English and Russian, settings](assets/screenshots/phones.png)

## Why

- **Made for phones.** A dense HWiNFO-style table that fits a 6" screen in portrait. Rotate to landscape to see all columns.
- **Looks like HWiNFO.** Sensors, collapsible groups (per-core clocks, usage, power...), Current / Minimum / Maximum / Average.
- **Light on resources.** One small `.exe`, no installer, no runtime. The page only updates changed cells and pauses when the phone screen is off.
- **Nothing to configure.** Start it, scan the QR code, done.

## Quick start

1. In HWiNFO open **Settings** and enable **Shared Memory Support**. Keep the sensors window open (minimized is fine).
2. Download `hwinfo-lan-monitor.exe` from [Releases](../../releases) and run it.
3. Scan the QR code in the window with your phone, or open one of the listed addresses in any browser on the same network.

![Host app window, light and dark](assets/screenshots/app.png)

Closing the window hides it to the tray. Exit from the tray icon menu.

### The phone cannot open the page

Windows Firewall blocks incoming connections by default. Allow the port once from PowerShell **as administrator**:

```powershell
.\scripts\firewall-rule.ps1
```

The rule allows TCP 8086 on private (home) networks only. Remove it with `-Remove`, use another port with `-Port <n>`.

## Features

**Dashboard** (settings under the gear icon, stored per browser):
- Theme: Auto (follows the device), Light, Dark.
- Refresh interval: as fast as HWiNFO polls, or slower.
- Language: English, Russian.
- Columns: show, hide and reorder (arrows, or drag the headers on a PC).
- Expanded sensors and groups are remembered. Search filters readings by name.
- Changed values are highlighted like in HWiNFO.

**Host app window** (stored in `config.json` next to the exe):
- Port and access mode: local network or this PC only.
- HWiNFO location is detected automatically; a button starts HWiNFO if it is not running.
- Start minimized to tray, language.
- Follows HWiNFO's light or dark mode. Remembers its position on screen.

## Good to know

- **HWiNFO free version** turns shared memory off after 12 hours. Enable it again in HWiNFO settings; the monitor reconnects by itself. If you enable it while HWiNFO is already running and nothing happens, restart HWiNFO.
- **Polling period** is set in HWiNFO (Settings, Sensors). The monitor shows it but cannot change it.
- **Groups** such as "Core Clocks" are not published by HWiNFO, so the monitor rebuilds them from reading names ("Core 0 ...", "Core 1 ..."). Names may differ slightly from HWiNFO.
- **Min / Max / Average** come from HWiNFO and can only be reset there.
- **New HWiNFO versions** should keep working: the shared memory format has only ever grown at the end, and the reader follows the sizes HWiNFO reports.

## Building

Requires PowerShell 7, Go 1.27+ and go-winres (icon and manifest):

```powershell
go install github.com/tc-hib/go-winres@v0.3.3
.\scripts\build-release.ps1          # vet, tests, resources, dist\hwinfo-lan-monitor.exe
.\scripts\run-dev.ps1                # headless dev server on :8086
```

If Go is not on `PATH`, put your own `scripts\env.ps1` next to the scripts; they load it automatically.

For screenshots and development without HWiNFO, run with synthetic sensors: `hwinfo-lan-monitor.exe -demo dark` (or `-demo light`).

## Releases

Releases are made by hand from GitHub: **Actions → Release → Run workflow**, choose which part of the version to bump (patch, minor, major) or type an exact version. The workflow builds and tests on Windows, tags `vX.Y.Z` and publishes a GitHub Release with the exe and its SHA-256.
