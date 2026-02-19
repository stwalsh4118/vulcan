// testserver starts a Vulcan API server with stub backends for E2E testing.
// Usage: go run ./cmd/testserver
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/seantiz/vulcan/internal/api"
	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

// stubBackend is a configurable mock backend for E2E tests.
type stubBackend struct {
	name      string
	runtimes  []string
	isolation string
	delay     time.Duration
	output    []byte
	logLines  []string
}

func (s *stubBackend) Execute(_ context.Context, spec backend.WorkloadSpec) (backend.WorkloadResult, error) {
	time.Sleep(s.delay)

	if spec.LogWriter != nil {
		for _, line := range s.logLines {
			spec.LogWriter(line)
		}
	}

	return backend.WorkloadResult{
		ExitCode: 0,
		Output:   s.output,
	}, nil
}

func (s *stubBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                s.name,
		SupportedRuntimes:   s.runtimes,
		SupportedIsolations: []string{s.isolation},
		MaxConcurrency:      10,
	}
}

func (s *stubBackend) Cleanup(_ context.Context, _ string) error { return nil }

func main() {
	addr := ":8080"
	if v := os.Getenv("VULCAN_LISTEN_ADDR"); v != "" {
		addr = v
	}

	db, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	reg := backend.NewRegistry()
	reg.Register(model.IsolationIsolate, &stubBackend{
		name:      "stub-isolate",
		runtimes:  []string{model.RuntimeNode, model.RuntimeWasm},
		isolation: model.IsolationIsolate,
		delay:     500 * time.Millisecond,
		output:    []byte("hello from isolate"),
		logLines:  []string{"[isolate] starting execution", "[isolate] running code", "[isolate] done"},
	})
	reg.Register(model.IsolationMicroVM, &stubBackend{
		name:      "stub-microvm",
		runtimes:  []string{model.RuntimeGo, model.RuntimePython},
		isolation: model.IsolationMicroVM,
		delay:     500 * time.Millisecond,
		output:    []byte("hello from microvm"),
		logLines:  []string{"[microvm] booting vm", "[microvm] executing", "[microvm] done"},
	})

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	eng := engine.NewEngine(db, reg, logger)
	srv := api.NewServer(addr, db, reg, eng, logger)

	logger.Info("testserver: starting", "addr", addr)
	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
