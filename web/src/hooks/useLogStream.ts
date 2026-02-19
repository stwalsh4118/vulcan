"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { WorkloadStatus } from "@/lib/types";
import { TERMINAL_STATUSES } from "@/lib/types";

const LOG_SSE_BASE_URL = "/api/v1/workloads";
const MAX_RECONNECT_ATTEMPTS = 3;
const RECONNECT_DELAY_MS = 2000;

export type LogStreamState = "connecting" | "connected" | "closed" | "error";

export function useLogStream(workloadId: string, status: WorkloadStatus | undefined) {
  const [lines, setLines] = useState<string[]>([]);
  const [streamState, setStreamState] = useState<LogStreamState>("closed");
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectCountRef = useRef(0);

  const close = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setStreamState("closed");
  }, []);

  useEffect(() => {
    // Only connect for active workloads
    if (!status || TERMINAL_STATUSES.includes(status)) {
      close();
      return;
    }

    function connect() {
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

      es.onerror = () => {
        es.close();
        eventSourceRef.current = null;

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
  }, [workloadId, status, close]);

  return { lines, streamState };
}
