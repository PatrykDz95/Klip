<p align="center">
  <img width="150" height="150" src="cmd/klip/assets/img.png" alt="Klip Logo">
</p>
<h1 align="center">Klip</h1>
<p align="center">
  Secure peer-to-peer clipboard sharing and file transfer across devices on your local network.
</p>
<p align="center">
  <a href="https://klip-it.app">
    <img src="https://img.shields.io/static/v1?label=Website&message=klip-it.app&color=7c6df0" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip/actions/workflows/ci.yml">
    <img src="https://github.com/PatrykDz95/Klip/actions/workflows/ci.yml/badge.svg" alt="CI" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip/actions/workflows/ci.yml">
    <img src="https://img.shields.io/static/v1?label=Coverage&message=coverage.out%20artifact&color=2ea44f" alt="Coverage" />
  </a> 
  <a href="https://github.com/PatrykDz95/Klip/releases/latest">
    <img src="https://img.shields.io/github/v/release/PatrykDz95/Klip?label=Release" alt="Release" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/PatrykDz95/Klip?label=License" alt="License" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip">
    <img src="https://img.shields.io/static/v1?label=Language&message=Go&color=00ADD8" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip">
    <img src="https://img.shields.io/static/v1?label=Security&message=TLS%201.3&color=2ea44f" />
  </a>
</p>
<p align="center">
  <a href="https://klip-it.app">🌐 klip-it.app</a>
</p>

Klip runs in the system tray and automatically syncs your clipboard between all connected devices. Copy on one machine, paste on another. It also supports sending files directly to a connected peer.

## Features
- **Clipboard sync** — copy text on one device, paste on any other connected device
- **File transfer** — send files to peers via the system tray menu
- **Zero configuration** — devices discover each other automatically using mDNS
- **End-to-end encryption** — all traffic is encrypted with TLS 1.3 using auto-generated certificates
- **Cross-platform** — works on macOS, Linux, and Windows

## How it works
1. On startup, Klip generates a self-signed TLS certificate and advertises itself on the local network via mDNS
2. When another Klip instance is discovered, devices establish a mutual TLS connection and exchange a handshake
3. Clipboard changes are detected and broadcast to all connected peers in real time
4. File transfers use a separate data channel with offer/accept protocol and progress tracking

## Klip Pro
The free version supports up to 2 devices. [Klip Pro](https://klip-it.app/#pricing) removes this limit.

## Installation

### Homebrew (macOS & Linux) — recommended
Requires [Homebrew](https://brew.sh). Install and run:
```bash
brew install PatrykDz95/tap/klip
klip
```
Homebrew builds Klip from source on your machine, so on macOS the binary is **not quarantined and Gatekeeper never blocks it** — no "Open Anyway" step needed.

> Recent Homebrew versions require you to trust third-party taps. If the install is refused with a tap-trust message, run:
> ```bash
> brew trust --formula PatrykDz95/tap/klip
> ```
> then re-run the install.

To update later:
```bash
brew update && brew upgrade klip
```

### Scoop (Windows)
If you don't have [Scoop](https://scoop.sh) yet, install it first (PowerShell, no admin needed):
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```
Then install and run Klip:
```powershell
scoop install https://raw.githubusercontent.com/PatrykDz95/Klip/master/packaging/scoop/klip.json
klip
```
Windows SmartScreen may warn on first run because the app is unsigned — click **More info → Run anyway** (see the antivirus note below).

### Manual download
Download the latest release from the [Releases page](https://github.com/PatrykDz95/Klip/releases/latest).
On macOS, unzip `klip-darwin-arm64.zip` and move **Klip.app** to your Applications folder (see the first-launch note below).

### macOS — first launch
Klip is **not** malware — it is open-source and unsigned (it has no paid Apple Developer certificate). Because of that, macOS Gatekeeper blocks it on first launch with a message like *"Apple could not verify Klip is free of malware"*. You only need to allow it **once**.

**Option 1 — via System Settings (recommended, works on macOS Sequoia):**
1. Double-click **Klip.app** — the blocked dialog appears; click **Done**.
2. Open **System Settings → Privacy & Security**, scroll down to the *"Klip was blocked…"* notice and click **Open Anyway**.
3. Confirm with Touch ID / password, then click **Open** in the final dialog.

After this, Klip launches normally every time.

**Option 2 — via Terminal (one command):**
```bash
xattr -dr com.apple.quarantine /Applications/Klip.app
```
Then open Klip.app as usual.

## Windows Defender / Antivirus Notice
Klip is unsigned, so your antivirus may flag it as suspicious. This is a false positive — the app is open source and you can review or build it yourself:
```bash
git clone https://github.com/PatrykDz95/Klip
cd Klip
go build -ldflags "-s -w" -o klip.exe ./cmd/klip
```

Klip appears in your system tray. From there you can:
- See connected devices
- Send a file — click the device name in the tray menu and select a file
- Pause/resume clipboard syncing

## Testing
- Unit/integration tests: `go test ./... -count=1 -timeout=2m`
- Race detector: `go test -race ./... -count=1 -timeout=5m`
- Coverage report file (local): `go test ./... -covermode=atomic -coverprofile=coverage.out`

## Security
- All connections use **TLS 1.3**
- Peer identity uses **TOFU + certificate fingerprint pinning**
- First contact auto-trusts a device; future certificate changes require user confirmation
- Certificates are **ECDSA P-256**, auto-generated and stored in `~/.klip-sync/certs/`
- Trusted peer fingerprints are stored in `~/.klip-sync/trusted_peers.json`
- Clipboard data is never sent unencrypted
- File transfers require explicit acceptance from the receiving device
