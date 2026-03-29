<p align="center">
  <img width="150" height="150" src="cmd/klip/assets/klip-fixed-256.png" alt="Klip Logo">
</p>
<h1 align="center">Klip</h1>
<p align="center">
  Secure peer-to-peer clipboard sharing and file transfer across devices on your local network.
</p>
<p align="center">
  <a href="https://klip-it.app">
    <img src="https://img.shields.io/static/v1?label=Website&message=klip-it.app&color=7c6df0" />
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

### macOS / Windows
Download the latest release from the [Releases page](https://github.com/PatrykDz95/Klip/releases/latest).

### macOS Gatekeeper Notice
macOS may block Klip on first launch because it is not signed with an Apple Developer certificate. This is expected for open-source tools distributed outside the App Store.

**To open it anyway:**

Option 1 — right-click the binary and select **Open**, then confirm in the dialog.

Option 2 — run this in Terminal after downloading:
```bash
xattr -cr /path/to/klip-darwin-arm64
```
Then double-click as normal.

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

## Security
- All connections use **TLS 1.3** with mutual authentication
- Certificates are **ECDSA P-256**, auto-generated and stored in `~/.klip-sync/certs/`
- Clipboard data is never sent unencrypted
- File transfers require explicit acceptance from the receiving device
