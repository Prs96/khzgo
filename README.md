# khzgo

A minimal terminal music player. Browse your library, queue tracks, and play through mpv — with cover art rendered right in the terminal.

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

| Key                | Action                       |
| ------------------ | ---------------------------- |
| `enter` / `l`      | play selected                |
| `a`                | add to queue                 |
| `d`                | remove from queue            |
| `s`                | shuffle-play this folder     |
| `A`                | play all (folder order)      |
| `n`                | next (random if queue empty) |
| `p`                | previous track               |
| `space`            | pause / resume               |
| `-` / `=`          | volume down / up             |
| `backspace` / `h`  | up one directory             |
| `j` / `k` / arrows | navigate                     |
| `/`                | filter                       |
| `?`                | toggle help                  |
| `q`                | quit                         |
