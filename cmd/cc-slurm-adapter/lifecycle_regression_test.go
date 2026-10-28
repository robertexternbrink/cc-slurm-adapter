package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ClusterCockpit/cc-slurm-adapter/internal/config"
)

func TestFailedInitPreservesExistingDaemonFiles(t *testing.T) {
	old := config.Config
	t.Cleanup(func() { config.Config = old })
	dir := t.TempDir()
	pid := filepath.Join(dir, "daemon.pid")
	socket := filepath.Join(dir, "daemon.sock")
	pidData := []byte(fmt.Sprint(os.Getpid()))
	socketData := []byte("other daemon socket marker")
	for name, data := range map[string][]byte{pid: pidData, socket: socketData} {
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	config.Config = config.ProgramConfig{PidFilePath: pid, PrepSockListenPath: socket, LastRunPath: filepath.Join(dir, "last_run")}
	if err := DaemonMain(); err == nil {
		t.Fatal("already running PID accepted")
	}
	for name, want := range map[string][]byte{pid: pidData, socket: socketData} {
		got, err := os.ReadFile(name)
		if err != nil || string(got) != string(want) {
			t.Fatalf("existing daemon file changed: %s: %q, %v", filepath.Base(name), got, err)
		}
	}
}

func TestPartialInitRemovesOnlyOwnedFiles(t *testing.T) {
	old := config.Config
	t.Cleanup(func() { config.Config = old })
	dir := t.TempDir()
	config.Config = config.ProgramConfig{PidFilePath: filepath.Join(dir, "daemon.pid"), PrepSockListenPath: filepath.Join(dir, "daemon.sock"), LastRunPath: filepath.Join(dir, "last_run")}
	// No executable can escape to a real scheduler: initialization fails at sacctmgr.
	t.Setenv("PATH", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := daemonInit(ctx, make(chan []byte, 1)); err == nil {
		t.Fatal("missing fake scheduler accepted")
	}
	for _, name := range []string{config.Config.PidFilePath, config.Config.PrepSockListenPath} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("owned file left after failed initialization: %s: %v", filepath.Base(name), err)
		}
	}
}
