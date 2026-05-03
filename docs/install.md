---
title: Installation
layout: default
nav_order: 2
---

# Installation

`m` ships as a single static binary. There are pre-built installers for
macOS and Linux on every tagged release.

## macOS

1. Go to the [latest release](https://github.com/subzone/m/releases/latest).
2. Download `m_<version>_macos_universal.pkg`. The package contains a
   universal binary (Intel + Apple Silicon).
3. Double-click the `.pkg`. macOS installs `m` to `/usr/local/bin/m`.

### Gatekeeper warning

The `.pkg` is **unsigned** until the project pays for an Apple Developer
Program account. On first launch you'll see "cannot be opened because
the developer cannot be verified". Two ways around it:

- **Right-click → Open** on the `.pkg`. macOS will then let you proceed.
- Or run `xattr -d com.apple.quarantine ~/Downloads/m_*.pkg` before
  double-clicking.

After install, the binary itself runs without any further prompts.

## Linux (Debian / Ubuntu)

```bash
wget https://github.com/subzone/m/releases/latest/download/m_<version>_linux_amd64.deb
sudo dpkg -i m_*_linux_amd64.deb
```

For ARM64 (Raspberry Pi, AWS Graviton, etc.), grab the `_linux_arm64.deb`
variant.

The package installs `/usr/local/bin/m`.

## Linux (other distros)

Grab the tarball:

```bash
wget https://github.com/subzone/m/releases/latest/download/m_<version>_linux_amd64.tar.gz
tar -xzf m_*_linux_amd64.tar.gz
sudo mv m /usr/local/bin/
```

## Build from source

Requires Go 1.26 or newer.

```bash
git clone https://github.com/subzone/m.git
cd m
go install ./cmd/m
```

The binary lands in `$(go env GOPATH)/bin/m`. Add that to your `PATH` if
it isn't already.

## Verify the install

```bash
m --version
```

You should see the tagged version (e.g. `m version 0.0.2`). Built-from-
source installs report `m version dev`.

## Next steps

- **[Quickstart](quickstart.html)** — run the first-launch wizard.
- **[Configuration](configuration.html)** — where files live, env vars.
