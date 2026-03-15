<p align="center">
  <img width="150" height="150" src="cmd/klip/assets/klip-fixed-256.png" alt="Klip Logo">
</p>
<h1 align="center">Klip</h1>
<p align="center">
  Secure peer-to-peer clipboard sharing and file transfer across devices on your local network.
</p>
<p align="center">
  <a href="https://github.com/PatrykDz95/Klip">
    <img src="https://img.shields.io/static/v1?label=Language&message=Go&color=00ADD8" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip/blob/main/LICENSE">
    <img src="https://img.shields.io/static/v1?label=License&message=MIT&color=000" />
  </a>
  <a href="https://github.com/PatrykDz95/Klip">
    <img src="https://img.shields.io/static/v1?label=Security&message=TLS%201.3&color=2ea44f" />
  </a>
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

## Installation

```bash
go install klip/cmd/klip@latest
```

Or build from source:

```bash
git clone https://github.com/PatrykDz95/Klip.git
cd klip
go build -o klip ./cmd/klip
```

### Linux dependencies

Klip requires `xclip` or `xsel` for clipboard access on Linux.

## Usage

```bash
# Start with default settings
klip

# Custom device name and port
klip -name "My Laptop" -port 9876

# Verbose logging
klip -v

# Connect to a specific peer manually
klip -peer 192.168.1.100:9876
```

Klip appears in your system tray. From there you can:

- See connected devices
- Send files by clicking on a device name (reads file path from clipboard)
- Pause/resume clipboard syncing

## Architecture

```
cmd/klip/          Entry point and embedded assets
internal/
  app/             Application layer (UI, orchestration, clipboard monitoring)
  clipboard/       Platform-specific clipboard drivers (macOS, Linux, Windows)
  p2p/             Networking (mDNS discovery, TLS connections, file transfer)
  security/        Certificate generation and management
```

## Security

- All connections use **TLS 1.3** with mutual authentication
- Certificates are **ECDSA P-256**, auto-generated and stored in `~/.klip-sync/certs/`
- Clipboard data is never sent unencrypted
- File transfers require explicit acceptance from the receiving device
