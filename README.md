# khzgo

A minimal terminal music player. Browse your library, queue tracks, and play through mpv with cover art rendered right in the terminal.

## Screenshot

<!-- TODO: add screenshot -->

![khzgo](docs/screenshot.png)

## Requirements

- Go 1.26+
- [mpv](https://mpv.io/)
- [ffmpeg](https://ffmpeg.org/) (cover art extraction)
- [chafa](https://hpjansson.org/chafa/) (cover art rendering)

## Build & run

```sh
git clone https://github.com/Prs96/khzgo.git
cd khzgo
go build
./khzgo ~/Music
```

Or skip the binary and run directly:

```sh
go run . ~/Music
```

## Keys

| Key                | Action                          |
| ------------------ | ------------------------------- |
| `enter` / `l`      | play selected                   |
| `a`                | add to queue                    |
| `d`                | remove from queue               |
| `s`                | shuffle-play this folder        |
| `A`                | play all (folder order)         |
| `n`                | next (random if queue empty)    |
| `p`                | previous track                  |
| `<` / `>`          | seek backward / forward 10s     |
| `x`                | stop playback                   |
| `r`                | toggle repeat track             |
| `D`                | clear queue                     |
| `space`            | pause / resume                  |
| `-` / `=`          | volume down / up                |
| `backspace` / `h`  | up one directory                |
| `j` / `k` / arrows | navigate                        |
| `/`                | filter (searches whole library) |
| `?`                | toggle help                     |
| `q`                | quit                            |

## Session

On quit, khzgo saves your session to `~/.config/khzgo/state.json` with last
folder, queue, history, volume, repeat mode, and the current track with its
position. The next launch restores all of it and resumes playback where you
left off (paused tracks come back paused).
