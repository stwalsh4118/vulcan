package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

func (s *Server) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify workload exists.
	wl, err := s.store.GetWorkload(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, "workload not found")
		return
	}
	if err != nil {
		s.logger.Error("get workload for logs", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get workload")
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// If already in a terminal state, return empty stream immediately.
	if wl.Status == model.StatusCompleted || wl.Status == model.StatusFailed || wl.Status == model.StatusKilled {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Disable write timeout for long-lived SSE connections.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		s.logger.Error("set write deadline for SSE", "error", err)
	}

	// Subscribe to the log stream. This is safe even if the workload completed
	// between the status check above and this call — Subscribe on a closed topic
	// returns a closed channel, causing the loop below to exit immediately.
	ch, unsub := s.engine.Broker().Subscribe(id)
	defer unsub()

	w.WriteHeader(http.StatusOK)
	flusher, canFlush := w.(http.Flusher)
	if canFlush {
		flusher.Flush()
	}

	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return // Workload finished; broker closed the channel.
			}
			if err := writeSSEData(w, line); err != nil {
				return // Write failed (e.g. client gone).
			}
			if canFlush {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return // Client disconnected.
		}
	}
}

// writeSSEData writes a log line as an SSE data event. Multi-line strings are
// split so that each segment gets its own "data:" prefix, per the SSE spec.
func writeSSEData(w http.ResponseWriter, line string) error {
	for seg := range strings.SplitSeq(line, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", seg); err != nil {
			return err
		}
	}
	// Blank line terminates the event.
	_, err := fmt.Fprint(w, "\n")
	return err
}
