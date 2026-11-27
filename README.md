# sshv

Lightweight Go TUI SSH vault for macOS. Reads/writes `~/.ssh/config` directly and stores optional passwords in macOS Keychain for automatic PTY login.

## Install / Build

```bash
go build -o /opt/homebrew/bin/sshv ./cmd/sshv
```

## Keybindings

- `j` / `k` or `↑` / `↓` — navigate hosts
- `Enter` — connect (auto-fills Keychain password if stored)
- `p` — reveal/hide stored password for selected host
- `a` — add host
- `e` — edit host (`Ctrl+R` to reveal password, `Ctrl+X` to clear)
- `d` — delete host (creates `~/.ssh/config.bak` before editing)
- `/` — search hosts (`Esc` to clear)
- `q` — quit
