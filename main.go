// Command dockprune is an interactive TUI to browse and delete Docker
// containers, images, and volumes.
//
// It talks to the Docker Engine directly through the official Go SDK, so it
// respects DOCKER_HOST, DOCKER_CONTEXT, and TLS settings from your environment.
//
// Source layout:
//   - docker.go  data access via the Docker SDK (list/remove)
//   - model.go   state machine (Bubble Tea model + update)
//   - view.go    rendering
//   - styles.go  lipgloss theme
//   - util.go    pure helpers
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

// appVersion prefers the ldflags-injected value, falling back to the module
// version stamped by `go install pkg@vX.Y.Z`.
func appVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			printUsage(os.Stdout)
			return
		case "-v", "--version", "version":
			fmt.Println("dockprune", appVersion())
			return
		default:
			fmt.Fprintf(os.Stderr, "dockprune: unknown argument %q\n\n", os.Args[1])
			printUsage(os.Stderr)
			os.Exit(2)
		}
	}

	d, err := newDockerClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "hint: is the Docker daemon running? check `docker info` and DOCKER_HOST.")
		os.Exit(1)
	}
	defer d.Close()

	p := tea.NewProgram(newModel(d), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if fm, ok := finalModel.(model); ok && fm.state == stateDone {
		if fail := reportFailures(fm.results); fail > 0 {
			os.Exit(1)
		}
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `dockprune - interactive TUI to delete Docker containers, images, and volumes

Usage:
  dockprune            launch the interactive resource picker
  dockprune -h         show this help
  dockprune -v         show version

Environment:
  DOCKER_HOST          Docker daemon address (default: unix:///var/run/docker.sock)
  DOCKER_CONTEXT       Docker context to use
  DOCKER_TLS_VERIFY    enable TLS verification
  DOCKER_CERT_PATH     path to TLS certificates
`)
}

// reportFailures prints any failed removes to stderr and returns the count.
func reportFailures(results []result) int {
	fail := 0
	for _, r := range results {
		if r.err != nil {
			fail++
			fmt.Fprintf(os.Stderr, "failed: %s: %s\n", r.title, oneLine(r.err.Error()))
		}
	}
	return fail
}
