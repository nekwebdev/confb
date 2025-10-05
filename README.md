# 🧩 confb — Config Blender  
> “One config to rule them all.”

---

## ⚡ TL;DR — Quick Start

```bash
# install the latest release (safe, no root)
sh -c "$(curl -fsSL https://raw.githubusercontent.com/nekwebdev/confb/main/scripts/install.sh)"

# copy and edit your config
cp ~/.config/confb/confb.sample.yaml ~/.config/confb/confb.yaml
$EDITOR ~/.config/confb/confb.yaml

# build once
confb build -c ~/.config/confb/confb.yaml

# run as daemon (auto rebuild on file changes)
confb run -c ~/.config/confb/confb.yaml --verbose

# reload live configuration (no restart needed)
pkill -HUP confb
```

---

## 🌱 Origin Story

**confb** was collaboratively designed and built by **nekwebdev** and **ChatGPT (GPT-5)** through a step-by-step, hands-on development process — line by line, commit by commit.

The project grew from a simple idea:

> Many Linux programs don’t support `include` or multi-file configs.  
> confb brings that flexibility — safely, predictably, and with full format awareness.

The goal was always clarity over cleverness, and reliability over magic.  
Today, confb stands as a small, single binary that quietly keeps your configuration files perfectly blended.

---

## 🚀 Overview

**confb** (Config Blender) is a lightweight Go daemon that watches a set of source configuration files, merges or concatenates them, and writes clean, validated outputs.

It can run as:
- a one-shot builder (`confb build`)
- or a background daemon (`confb run`) that automatically rebuilds when inputs change.

---

## 🧰 Features

✅ Watch and rebuild outputs on file changes  
✅ Merge multiple formats:
   - **KDL** — merge duplicate sections or keys  
   - **YAML / JSON / TOML** — deep maps, replace/append arrays  
   - **INI** — append or override duplicate keys  
   - **RAW** — simple concatenation  
✅ Debounce rebuilds to avoid thrashing  
✅ SIGHUP reload of main `confb.yaml`  
✅ Per-target `on_change` hooks (e.g. reload your app)  
✅ Optional systemd user service integration  
✅ Atomic writes for safety  
✅ Cross-platform (Linux/macOS, amd64 & arm64)  
✅ Zero dependencies, zero runtime overhead  

---

## 🧩 Supported Formats & Merge Rules

| Format | Key Behavior | Map Merge | Array Merge | Section Control |
|--------|---------------|------------|--------------|-----------------|
| **KDL** | `first_wins`, `last_wins`, `append` | — | — | merge specific sections only |
| **YAML / JSON / TOML** | — | `deep` or `replace` | `append`, `unique_append`, `replace` | — |
| **INI** | `last_wins` or `append` for repeated keys | — | — | per-section |
| **RAW** | no parsing | — | — | simple concatenation |

---

## ⚙️ Installation

### 🐚 One-liner (recommended)

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/nekwebdev/confb/main/scripts/install.sh)"
```

This will:
- Detect your OS/architecture  
- Download the latest release tarball from GitHub  
- Verify its checksum  
- Install `confb` to `~/.local/bin/confb`  
- Create:
  - `~/.config/confb/confb.sample.yaml`
  - `~/.config/systemd/user/confb.service` (not enabled)

No root privileges required.

---

### 🔧 Manual install

```bash
git clone https://github.com/nekwebdev/confb.git
cd confb
make build
install -m 755 bin/confb ~/.local/bin/
```

---

## 🧠 Getting Started

### 1. Prepare your config

```bash
cp ~/.config/confb/confb.sample.yaml ~/.config/confb/confb.yaml
$EDITOR ~/.config/confb/confb.yaml
```

Minimal example:

```yaml
version: 1
targets:
  - name: niri
    format: kdl
    output: ~/.config/niri/config.kdl
    sources:
      - path: ~/.config/niri/colors.kdl
      - path: ~/.config/niri/src/*.kdl
        sort: lex
    merge:
      rules:
        keys: last_wins
        section_keys: ["layout"]
```

---

### 2. Build once

```bash
confb build -c ~/.config/confb/confb.yaml
```

---

### 3. Run as daemon

```bash
confb run -c ~/.config/confb/confb.yaml --verbose
```

To reload config live:

```bash
pkill -HUP confb
```

---

### 4. Enable at login (optional)

If using systemd:

```bash
systemctl --user enable --now confb.service
```

Or add to your session startup:

```bash
~/.local/bin/confb run -c ~/.config/confb/confb.yaml &
```

---

## 🧩 File layout

| Path | Description |
|------|--------------|
| `~/.local/bin/confb` | binary |
| `~/.config/confb/confb.yaml` | your config |
| `~/.config/confb/confb.sample.yaml` | full reference example |
| `~/.config/systemd/user/confb.service` | systemd unit |

---

## ⚡ CLI Reference

| Command | Description |
|----------|--------------|
| `confb build` | One-shot merge/concat |
| `confb validate` | Validate config |
| `confb run` | Daemon with file watch |
| `--quiet` / `--verbose` | Log level |
| `--color` | ANSI colors in log |
| `--debounce-ms <ms>` | Rebuild delay |
| `--config <path>` | Alt config path |

---

## 🧱 Example on_change hook

```yaml
on_change: |
  systemctl --user reload myapp || true
  notify-send "confb" "{target} updated!"
```

Vars:
- `{target}` — target name  
- `{output}` — output path  
- `{timestamp}` — ISO timestamp  

---

## 🔄 SIGHUP Reload

Change your config on the fly:

```bash
pkill -HUP confb
```

confb will re-parse `confb.yaml`, update watchers, and rebuild all targets.

---

## 🧮 Safety

- SHA-256 output checksums prevent redundant writes  
- Atomic writes ensure never-corrupted files  
- Merge errors log but never overwrite good output  

---

## 🧱 Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Local dry-run release

```bash
goreleaser release --snapshot --clean
```

### Tag a release

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions (via GoReleaser) will build binaries and publish release artifacts.

---

## 🧾 License

GPLv3 © 2025 **nekwebdev**  
Built collaboratively with **OpenAI GPT-5**.

---

## 💬 Acknowledgments

- **nekwebdev** — vision, architecture, and persistence.  
- **ChatGPT (GPT-5)** — code implementation, architecture, and tooling integration.  
- **Open-source community** — for libraries and ideas.

confb was built with one guiding principle:  
> *Make configuration composable again.*
