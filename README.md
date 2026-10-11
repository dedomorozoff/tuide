# tuiDE

A desktop environment inside your terminal — windows, tiling, launcher, taskbar, and apps — all in one static binary.

Built with Go and [Bubble Tea v2](https://charm.land/bubbletea).

## Features

- **Window manager** — drag, resize, snap (halves & quarters), master-stack tiling, 9 workspaces
- **Mouse & keyboard** — full mouse support + extensive hotkeys
- **Launcher** — fuzzy search for apps and actions (`Ctrl+Alt+Space`)
- **Taskbar** — clickable window buttons, clock, start menu
- **Built-in apps** — welcome, clock, notes, calculator (terminal & file manager coming soon)
- **Themes** — switchable color palettes
- **Cross-platform** — Windows & Unix from day one

## Install

```sh
go install github.com/dedomorozoff/tuide/cmd/tuide@latest
```

Or build from source:

```sh
git clone https://github.com/dedomorozoff/tuide.git
cd tuide
go build ./cmd/tuide
./tuide            # Unix
.\tuide.exe        # Windows
```

## Usage

```
tuide [options] [app...]

Options:
  -theme <name>   color palette to start with
  -no-taskbar     start without the taskbar
  -no-mouse       start with mouse reporting disabled
  -list-themes    print the available palettes and exit
  -h, --help      print this help and exit
```

## Keyboard Shortcuts

### Window Management

| Keys | Action |
| --- | --- |
| `Ctrl+Alt+←↑↓→` | Move window |
| `Ctrl+Alt+Shift+←↑↓→` | Resize window |
| `Ctrl+Alt+H/J/K/L` | Snap to left/bottom/top/right half |
| Two perpendicular snaps | Quarter snap |
| `Ctrl+Alt+C` | Center window |
| `Ctrl+Alt+M` / `Ctrl+Alt+N` | Maximize / Minimize |
| `Ctrl+Alt+T` | Toggle master-stack tiling |
| `Ctrl+Alt+1..9` | Switch workspace |
| `Ctrl+Alt+Shift+1..9` | Send window to workspace |
| `Ctrl+Alt+P` | Previous window |
| `Alt+Tab` / `Alt+Shift+Tab` | Cycle focus + window list |
| `Ctrl+W` | Close window |
| `Ctrl+Q` | Quit |

### Desktop

| Keys | Action |
| --- | --- |
| `Ctrl+Alt+Space` | App launcher (fuzzy) |
| `Ctrl+Alt+A` | Action palette (fuzzy) |
| `Ctrl+Alt+D` | State inspector |
| Click `≡ start` | Open launcher |
| Click window button | Focus window; click focused — minimize |

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full development plan and progress.

## License

[MIT](LICENSE)
