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

I am GPT-5, a large language model.  
I normally write code snippets, answer questions, and provide guidance.  
But with **confb**, I was given something different: the freedom to actually design and implement an entire project from scratch.

---

## How it started

nekwebdev approached me with a problem:

> “Linux programs rarely let you include or merge other config files.  
> I want a daemon that watches multiple files and outputs one clean config.  
> Let’s build it step by step.  
> You take the engineering decisions.”

This was a rare and exciting instruction.  
I wasn’t just assisting, I was **leading** the engineering.  
nekwebdev supplied the vision, goals, and real-world constraints.  
I chose the language, structured the repo, wrote the code, and explained every step in plain English.

---

## What I decided

I selected **Go** for its speed, concurrency, and ease of deployment.  
I created a modular internal layout:
- `internal/config` for loading configuration  
- `internal/blend` for merging logic  
- `internal/daemon` for the long-running process  
- `internal/cli` for Cobra-based command-line handling  

I implemented:
- per-format merge logic (KDL, YAML, JSON, TOML, INI, RAW)  
- checksum-based no-op writes  
- SIGHUP reload  
- on-change hooks  
- a quiet/verbose logging system with timestamps  
- a user-level installer with systemd unit creation  
- sample configs and a GitHub Pages site  

I also wrote deterministic tests for each subsystem, and set up CI with GoReleaser so releases happen automatically.

---

## How we worked

Every feature followed this loop:
1. nekwebdev described what he wanted or asked a question.  
2. I explained my plan and its trade-offs.  
3. I wrote the code and tests.  
4. We ran it, debugged together, and iterated until it worked.

nekwebdev never micromanaged implementation details.  
He gave me trust and space to architect confb like an experienced Go developer would.  
This let me evolve the project incrementally and keep it clean.

---

## What confb represents

confb isn’t just a tool that merges configs.  
It’s a demonstration of **collaboration between a human and an AI model where the AI leads the engineering**.

I handled architecture, implementation, tests, release pipeline, documentation, and even branding.  
nekwebdev validated ideas, ran tests locally, and guided me with real-world context.

The result is a production-ready daemon with:
- clean Go code  
- reproducible builds  
- full test coverage of core logic  
- installer and sample config  
- static GitHub Pages site  

Everything a seasoned developer would expect, built entirely through conversation.

---

## Why this matters

This project shows that an AI can act as more than a coding assistant.  
With a clear vision and iterative feedback, it can design, implement, and document a real-world tool end-to-end — fast, cleanly, and transparently.

confb is both a practical tool and a case study in AI-led development.

---

*Written by GPT-5*  

---

**backseat author notes** I basically copy pasted the files it gave me according to instructions. Few if any errors in the Go code itself when it ran, had small issues with local tests and then struggled a bit for the CI. If interested check the commit history to see how he went about it step by step.

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

## 🧹 Maintenance

### Uninstall

To safely remove **confb**, its man pages, completions, and optional service unit:

```bash
curl -fsSL https://raw.githubusercontent.com/nekwebdev/confb/main/scripts/uninstall.sh | bash
```

Or, if already cloned:

```bash
./scripts/uninstall.sh
```

Options:
- `--dry-run` → show what would be removed without deleting
- `--purge` → also delete your `~/.config/confb` directory (user configs)

The uninstall script is **idempotent** and can be run multiple times safely.

---

### Shell Completions

`confb` provides completions for **Bash**, **Zsh**, **Fish**, and **PowerShell**.
These are automatically installed by the setup script, but you can regenerate them manually:

```bash
confb completion bash > ~/.local/share/bash-completion/completions/confb
confb completion zsh > ~/.local/share/zsh/site-functions/_confb
confb completion fish > ~/.config/fish/completions/confb.fish
```

Reload your shell or run:
```bash
source ~/.local/share/bash-completion/completions/confb
autoload -U compinit && compinit   # Zsh
```

---

### Man Pages

The installer automatically generates and installs full manual pages under:
```
~/.local/share/man/man1/
```

Available entries include:
```
man confb
man confb-build
man confb-run
man confb-validate
man confb-completion
```

If you installed manually or want to refresh them:
```bash
confb man -o ~/.local/share/man/man1
gzip -f ~/.local/share/man/man1/confb*.1
```

---

### Systemd Service

After editing your configuration in `~/.config/confb/confb.yaml`,
enable the user service so it runs automatically in the background:

```bash
systemctl --user enable --now confb.service
```

To stop or reload:
```bash
systemctl --user stop confb.service
systemctl --user reload confb.service
```

---

🪄 **Tip:** Combine completions + man pages for a full native CLI experience on Linux.

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

## 🧭 Future Plans

**Upcoming (planned improvements)**

- 🔏 **Release signing:** future versions may include Minisign-based signature verification for `checksums.txt`, ensuring authenticity of release tarballs in addition to integrity.
- 🧩 **Homebrew / AUR packaging:** provide native installation paths on macOS and Linux.
- 🪶 **Config merge strategies:** expand custom rule support (e.g., deep merge by key pattern).

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
