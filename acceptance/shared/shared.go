// Package shared implements utility functions for acceptance testing Hydra as
// well as shared test cases.
package shared

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPgPoolConnect is used when pgxpool cannot connect to a database.
var ErrPgPoolConnect = errors.New("pgxpool did not connect")

// MustHaveValidContainerLogDir ensures that if a container log directory is
// present it is has an absolute path as go tests cannot determine the directory
// that they are running from.
func MustHaveValidContainerLogDir(logDir string) {
	if logDir != "" && !filepath.IsAbs(logDir) {
		log.Fatalf("the container log dir must be absolute, got %s", logDir)
	}
}

// CreatePGPool calls pgxpool.New and then sends a Ping to the database to
// ensure it is running. If the ping fails it returns a wrapped
// ErrPgPoolConnect.
func CreatePGPool(t *testing.T, ctx context.Context, username, password string, port int) (*pgxpool.Pool, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d", username, password, port))
	if err != nil {
		return nil, fmt.Errorf("failed to construct new pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPgPoolConnect, err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool, nil
}

// TerminateContainer terminates a running docker container. If logDir is
// included then the container logs are saved to that directory before it is
// terminated. If kill is false docker stop is used, otherwise docker kill is.
func TerminateContainer(t *testing.T, ctx context.Context, containerName, logDir string, kill bool) {
	if containerName == "" {
		return
	}

	writeLogs(t, ctx, containerName, logDir)

	var termCmd *exec.Cmd
	if kill {
		termCmd = exec.CommandContext(ctx, "docker", "kill", containerName)
	} else {
		termCmd = exec.CommandContext(ctx, "docker", "stop", "--time", "30", containerName)
	}

	if output, err := termCmd.CombinedOutput(); err != nil {
		t.Fatalf("unable to terminate container %s: %s", err, output)
	}
}

func writeLogs(t *testing.T, ctx context.Context, containerName, logDir string) {
	if logDir == "" {
		return
	}

	logCmd := exec.CommandContext(ctx, "docker", "logs", containerName)
	logOutput, err := logCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unable to fetch container log %s: %s", err, logOutput)
	}

	if err := os.WriteFile(filepath.Join(logDir, fmt.Sprintf("%s.log", containerName)), logOutput, 0644); err != nil {
		t.Fatalf("unable to write container log: %s", err)
	}
}
