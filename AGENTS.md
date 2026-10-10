# AGENTS.md

Interactive TUI (Bubble Tea) to browse and delete Docker containers, images, and
volumes. Talks to the Docker Engine via the official Go SDK
(`github.com/moby/moby/client`) — no shelling out to the `docker` CLI.

## Rules
- NEVER commit or push without explicit user approval.
- Tests never need a Docker daemon (fake API injected).

## Commands
- `make build` — build for current OS into `dist/dockprune`
- `make build-all` — cross-compile all OS/arch into `dist/`
- `make test` / `make test-race` — run tests (race detector required for the fan-out/backfill concurrency paths)
- `make lint` — golangci-lint, falls back to `go vet`
- `make demo` — regenerate `assets/demo.gif` with vhs against the fake API (see below)
- `make clean` — remove `dist/`

## Demo / GIF
- `demo.tape` (vhs) + `demo/fakeapi` (a demo-only fake Docker Engine API) produce
  `assets/demo.gif`. The fake server is a fixture, NOT part of the binary.

## Key facts
- SDK: `github.com/moby/moby/client` v0.6.x + `github.com/moby/moby/api`. This is the
  2026 canonical path; `github.com/docker/docker` v28 is frozen (`+incompatible`).
- Data access: `docker.go`. `newDockerClient` uses `client.New(client.FromEnv)` + `Ping`.
- Source layout: docker.go = data access, model.go = state machine, view.go = rendering,
  styles.go = lipgloss theme, util.go = pure helpers.
- Three tabs (containers/images/volumes). Selection is per-tab and persists across switches;
  cursor/filter reset on switch. `visibleIndexes()` maps cursor position -> storage index
  (never reslice `items`).
- Tests never need a Docker daemon: `model.remove` is injected (model.go) and `dockerAPI`
  is swapped for `fakeAPI` in model_test.go.
- `maxConcurrent = 8` caps parallel removes (model.go).
- Deletes always use `force`; the confirm screen (with dependency warnings) is the gate.
- `image.Summary.Containers` and `volume.UsageData` from list calls are unreliable (`-1`);
  "in use" is computed by cross-referencing `ContainerList`.
