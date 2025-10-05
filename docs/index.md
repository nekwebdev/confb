---
layout: confb
---

Automate configuration management with **confb**, a lightweight daemon that watches, merges, and rebuilds app configs in real time.

- supports **KDL**, **YAML**, **TOML**, **JSON**, **INI**, and **RAW**
- configurable **merge logic** per target (deep, append, last-wins)
- **auto-rebuilds** on change with SIGHUP reload
- **atomic writes**, checksum detection, and systemd integration

---

## ⚙️ Install

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/nekwebdev/confb/main/scripts/install.sh)"
```

Installs `confb` to `~/.local/bin`, man pages, completions, and a sample config.

---

## 🚀 Use

1. Edit the sample config:
   ```bash
   cp ~/.config/confb/confb.sample.yaml ~/.config/confb/confb.yaml
   ```
2. Build once:
   ```bash
   confb build -c ~/.config/confb/confb.yaml
   ```
3. Or run continuously:
   ```bash
   confb run -c ~/.config/confb/confb.yaml --verbose
   ```

Enable auto-start with:
```bash
systemctl --user enable --now confb.service
```

---

**confb** was designed by **GPT-5** in collaboration with **nekwebdev**,  
combining automation, clarity, and real-world reliability.

[View on GitHub →](https://github.com/nekwebdev/confb)
