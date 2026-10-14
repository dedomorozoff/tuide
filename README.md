# tuiDE

A desktop environment inside your terminal — windows, tiling, launcher, taskbar, and apps — all in one static binary.

Built with Go and [Bubble Tea v2](https://charm.land/bubbletea).

## Features

- **Window manager** — drag, resize, snap (halves & quarters), master-stack tiling, 9 workspaces
- **Mouse & keyboard** — full mouse support + extensive hotkeys
- **Terminal** — real PTY (Unix pty / Windows ConPTY) with a virtual terminal: colors, scrollback, alternate screen
- **Launcher** — fuzzy search for apps and actions (`Ctrl+Alt+Space`)
- **Taskbar** — clickable window buttons, clock, start menu
- **Built-in apps** — welcome, clock, notes, calculator, settings, terminal (file manager & editor coming soon)
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
| `Ctrl+Alt+W` | Close window |
| `Ctrl+Alt+Q` | Quit |

### Desktop

| Keys | Action |
| --- | --- |
| `Ctrl+Alt+Space` | App launcher (fuzzy) |
| `Ctrl+Alt+A` | Action palette (fuzzy) |
| `Ctrl+Alt+D` | State inspector |
| Click `≡ start` | Open launcher |
| Click window button | Focus window; click focused — minimize |

### Terminal

The desktop claims no bare `Ctrl` chord: every shortcut above uses `Ctrl+Alt`
(or `Alt+Tab`), so everything else belongs to the program in the window. Inside
a terminal window that means `Ctrl+C` interrupts, `Ctrl+W` deletes a word and
`Ctrl+Q` is XON, exactly as in any other terminal.

| Keys | Action |
| --- | --- |
| Any unbound key | Goes to the shell |
| Mouse wheel | Scrolls the scrollback, until the program asks for the mouse |
| `Ctrl+Shift+V` | Paste from the clipboard (bracketed paste) |

`vim`, `htop`, `less` and `nano` work: colors, the alternate screen, cursor
shapes and resizing are all handled.

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full development plan and progress.

## License

[MIT](LICENSE)
