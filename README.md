# dockprune

Interactive terminal UI (TUI) to browse and **bulk-delete Docker containers,
images, and volumes** — reclaim disk space from your terminal in seconds.
Talks straight to the Docker Engine through the official Go SDK, so it respects
your existing Docker environment (`DOCKER_HOST`, contexts, TLS).

![dockprune demo](assets/demo.gif)

## Features

- Three tabs — **containers**, **images**, **volumes** — with live item counts
- List name, image/driver, status, and age per resource
- Multi-select with `space`, select-all (`a`) / select-none (`n`)
- Live search/filter (`/`) over name, image, status, and id
- Details pane for the focused resource (id, image, status, size, age)
- Dependency warnings: running containers and in-use images/volumes are flagged
  before you delete
- `y/N` confirmation gate; bulk delete runs with a bounded worker pool and live
  ✓/✗ progress
- Graceful handling of a missing daemon, connection errors, and empty tabs

## Requirements

- Go 1.26+ to build from source
- A running Docker daemon (local socket, or a remote host via `DOCKER_HOST`)
- A TTY for the interactive UI

## Installation

### Build from source

```sh
go install .        # or: make build
```

`make build` drops the binary at `dist/dockprune`.

### Build all platforms

```sh
make build-all      # cross-compiles linux/darwin/windows × amd64/arm64 into dist/
```

## Usage

```sh
dockprune
```

Keybindings:

| Key                     | Action                                      |
|-------------------------|---------------------------------------------|
| `↑` / `↓` / `k` / `j`   | Move cursor                                 |
| `g` / `home`, `G` / `end` | Jump to first / last row                  |
| `←` / `→` / `tab` / `1` `2` `3` | Switch resource tab                 |
| `space`                 | Toggle selection                            |
| `a` / `n`               | Select all / none (of the visible set)      |
| `/`                     | Search (type, `enter` to apply, `esc` to clear) |
| `enter`                 | Proceed to confirmation                     |
| `y`                     | Confirm deletion (irreversible)             |
| `N` / `esc` / `q`       | Go back / quit                              |

Deletion is **irreversible**: `dockprune` force-removes running containers and
in-use images/volumes after you confirm. Exit code is non-zero when any delete
fails.

### Environment

| Variable            | Description                                             |
|---------------------|---------------------------------------------------------|
| `DOCKER_HOST`       | Daemon address (default `unix:///var/run/docker.sock`)  |
| `DOCKER_CONTEXT`    | Docker context to use                                   |
| `DOCKER_TLS_VERIFY` | Enable TLS verification                                 |
| `DOCKER_CERT_PATH`  | Path to TLS certificates                                |

Any setting that `docker` itself honors is read through the SDK's `FromEnv`.

## How it works

- Lists containers/images/volumes via the Docker Engine SDK
  (`github.com/moby/moby/client`).
- Cross-references running containers to flag images and volumes that are
  still in use.
- Removes the selected resources with `force` (running containers are
  force-stopped; in-use images/volumes are force-removed), at most 8 in flight
  (`maxConcurrent` in `model.go`).

## Development

```sh
make build        # build for the current OS into dist/dockprune
make build-all    # build for every OS/arch into dist/
make test         # run tests
make test-race    # run tests with the race detector
make lint         # golangci-lint, falls back to go vet
make demo         # regenerate assets/demo.gif (needs vhs; records against a fake API)
make clean        # remove dist/
```

`make demo` starts `demo/fakeapi` — a demo-only fake Docker Engine API — and
points `DOCKER_HOST` at it, so recording the GIF never touches a real daemon.

## Layout

| File         | Responsibility                          |
|--------------|-----------------------------------------|
| `docker.go`  | Data access: Docker SDK list/remove     |
| `model.go`   | State machine (Bubble Tea model)        |
| `view.go`    | Rendering                               |
| `styles.go`  | lipgloss theme                          |
| `util.go`    | Pure helpers                            |

Tests never need a Docker daemon: `model.remove` is injected and `dockerAPI` is
swapped for a fake.

## License

MIT
