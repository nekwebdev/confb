---
layout: confb
---

Automate configuration management with **confb**, a lightweight daemon that watches, merges, and rebuilds application config files.

- supports KDL, YAML, TOML, JSON, INI, and RAW formats  
- deep merge or unique-append logic per target  
- auto-rebuild on file change with SIGHUP reload  
- safe atomic writes and checksum detection  
- includes installer, systemd user service, and sample config  

## Quick start
```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/nekwebdev/confb/main/scripts/install.sh)"
confb build -c ~/.config/confb/confb.yaml
confb run -c ~/.config/confb/confb.yaml
```

This was en expirement with GPT-5, check more in the [project readme](https://github.com/nekwebdev/confb).
