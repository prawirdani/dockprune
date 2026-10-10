// Command fakeapi serves a minimal, hard-coded Docker Engine API over a unix
// socket so the demo GIF can be recorded without touching a real Docker daemon
// (and without leaking the host's real container names).
//
// It is a demo/CI fixture, NOT part of the dockprune binary: it never ships in a
// release and `dockprune` itself has no knowledge of it. It answers exactly the
// endpoints the SDK client calls (ping, list containers/images/volumes, and
// removes), with fixed sample data.
//
//	go run ./demo/fakeapi --socket /tmp/dockprune-demo.sock
//	DOCKER_HOST=unix:///tmp/dockprune-demo.sock ./dist/dockprune
//
// ponytail: fixed data, no state, deletes just succeed. Enough for a recording;
// make it stateful only if a demo ever needs to reflect deletions.
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/volume"
)

func main() {
	socket := flag.String("socket", "/tmp/dockprune-demo.sock", "unix socket to listen on")
	flag.Parse()

	_ = os.Remove(*socket)
	ln, err := net.Listen("unix", *socket)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(*socket)

	srv := &http.Server{Handler: http.HandlerFunc(handle)}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		srv.Close()
	}()

	log.Printf("fake Docker API listening on unix://%s", *socket)
	log.Fatal(srv.Serve(ln))
}

func handle(w http.ResponseWriter, r *http.Request) {
	switch path := stripVersion(r.URL.Path); {
	case path == "/_ping":
		w.Header().Set("API-Version", "1.56")
		w.Header().Set("OSType", "linux")
		w.Header().Set("Docker-Experimental", "false")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "OK")
	case path == "/containers/json" && r.Method == http.MethodGet:
		writeJSON(w, containers())
	case path == "/images/json" && r.Method == http.MethodGet:
		writeJSON(w, images())
	case path == "/volumes" && r.Method == http.MethodGet:
		writeJSON(w, map[string]any{"Volumes": volumes(), "Warnings": []string{}})
	case strings.HasPrefix(path, "/containers/") && r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case strings.HasPrefix(path, "/images/") && r.Method == http.MethodDelete:
		writeJSON(w, []image.DeleteResponse{})
	case strings.HasPrefix(path, "/volumes/") && r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "not found: "+path, http.StatusNotFound)
	}
}

// stripVersion removes a leading "/v1.56" API-version segment so every route is
// version-agnostic.
func stripVersion(p string) string {
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(parts) > 0 && strings.HasPrefix(parts[0], "v1.") {
		parts = parts[1:]
	}
	return "/" + strings.Join(parts, "/")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func containers() []container.Summary {
	now := time.Now().Unix()
	return []container.Summary{
		{
			ID: "a1b2c3d4e5f6", Names: []string{"/web"}, Image: "nginx:latest",
			ImageID: "sha256:1a11c0de", State: container.StateRunning,
			Status: "Up 3 hours", Created: now - 3*3600, SizeRw: 12_500_000,
		},
		{
			ID: "f6e5d4c3b2a1", Names: []string{"/db"}, Image: "postgres:16",
			ImageID: "sha256:2b22c0de", State: container.StateExited,
			Status: "Exited (0) 2 days ago", Created: now - 2*86400,
			Mounts: []container.MountPoint{{Type: mount.TypeVolume, Name: "pgdata"}},
		},
		{
			ID: "112233445566", Names: []string{"/cache"}, Image: "redis:7",
			ImageID: "sha256:3c33c0de", State: container.StateRunning,
			Status: "Up 12 minutes", Created: now - 12*60,
			Mounts: []container.MountPoint{{Type: mount.TypeVolume, Name: "redis-data"}},
		},
		{
			ID: "665544332211", Names: []string{"/worker"}, Image: "alpine:latest",
			ImageID: "sha256:4d44c0de", State: container.StateExited,
			Status: "Exited (137) 5 days ago", Created: now - 5*86400,
		},
	}
}

func images() []image.Summary {
	now := time.Now().Unix()
	return []image.Summary{
		{ID: "sha256:1a11c0de", RepoTags: []string{"nginx:latest"}, Created: now - 10*86400, Size: 187_000_000},
		{ID: "sha256:2b22c0de", RepoTags: []string{"postgres:16"}, Created: now - 20*86400, Size: 430_000_000},
		{ID: "sha256:3c33c0de", RepoTags: []string{"redis:7"}, Created: now - 15*86400, Size: 138_000_000},
		{ID: "sha256:4d44c0de", RepoTags: []string{"alpine:latest"}, Created: now - 30*86400, Size: 7_800_000},
		{ID: "sha256:5e55c0de", RepoTags: nil, Created: now - 40*86400, Size: 1_020_000_000}, // dangling
	}
}

func volumes() []volume.Volume {
	now := time.Now().Add(-6 * 86400 * time.Second).Format(time.RFC3339Nano)
	return []volume.Volume{
		{Name: "pgdata", Driver: "local", Mountpoint: "/var/lib/docker/volumes/pgdata/_data", CreatedAt: now},
		{Name: "redis-data", Driver: "local", Mountpoint: "/var/lib/docker/volumes/redis-data/_data", CreatedAt: now},
		{Name: "scratch", Driver: "local", Mountpoint: "/var/lib/docker/volumes/scratch/_data", CreatedAt: now},
	}
}
