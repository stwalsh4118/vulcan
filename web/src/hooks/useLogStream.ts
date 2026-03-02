"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { WorkloadStatus } from "@/lib/types";
import { TERMINAL_STATUSES } from "@/lib/types";
import { getLogHistory } from "@/lib/api";

const LOG_SSE_BASE_URL = "/api/v1/workloads";
const MAX_RECONNECT_ATTEMPTS = 3;
const RECONNECT_DELAY_MS = 2000;

export type LogStreamState =
  | "connecting"
  | "connected"
  | "closed"
  | "error"
  | "historical";

export function useLogStream(
  workloadId: string,
  status: WorkloadStatus | undefined,
) {
  const [lines, setLines] = useState<string[]>([]);
  const [streamState, setStreamState] = useState<LogStreamState>("closed");
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectCountRef = useRef(0);
  const doneReceivedRef = useRef(false);
  const isTerminal = !!status && TERMINAL_STATUSES.includes(status);

  const close = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  // SSE streaming for active workloads.
  useEffect(() => {
    if (!status || isTerminal) {
      close();
      return;
    }

    function connect() {
      doneReceivedRef.current = false;
      const url = `${LOG_SSE_BASE_URL}/${workloadId}/logs`;
      const es = new EventSource(url);
      eventSourceRef.current = es;
      setStreamState("connecting");

      es.onopen = () => {
        setStreamState("connected");
        reconnectCountRef.current = 0;
      };

      es.onmessage = (event) => {
        setLines((prev) => [...prev, event.data]);
      };

      es.addEventListener("done", () => {
        doneReceivedRef.current = true;
        es.close();
        eventSourceRef.current = null;
        setStreamState("closed");
      });

      es.onerror = () => {
        es.close();
        eventSourceRef.current = null;

        // If the server sent a "done" event, the stream closed gracefully.
        if (doneReceivedRef.current) return;

        if (reconnectCountRef.current < MAX_RECONNECT_ATTEMPTS) {
          reconnectCountRef.current += 1;
          setStreamState("connecting");
          setTimeout(connect, RECONNECT_DELAY_MS);
        } else {
          setStreamState("error");
        }
      };
    }

    connect();

    return () => {
      close();
    };
  }, [workloadId, status, isTerminal, close]);

  // Fetch historical logs when workload reaches terminal state.
  useEffect(() => {
    if (!isTerminal) return;

    let cancelled = false;
    getLogHistory(workloadId).then(
      (resp) => {
        if (cancelled) return;
        setLines(resp.lines.map((l) => l.line));
        setStreamState("historical");
      },
      () => {
        if (cancelled) return;
        setStreamState("error");
      },
    );

    return () => {
      cancelled = true;
    };
  }, [workloadId, isTerminal]);

  return { lines, streamState };
}
