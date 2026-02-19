"use client";

import { useEffect, useRef, useState } from "react";
import type { LogStreamState } from "@/hooks/useLogStream";

const STATE_LABELS: Record<LogStreamState, string> = {
  connecting: "Connecting...",
  connected: "Streaming",
  closed: "Disconnected",
  error: "Connection failed",
  historical: "Complete",
};

export function LogViewer({
  lines,
  streamState,
}: {
  lines: string[];
  streamState: LogStreamState;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (!paused && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [lines, paused]);

  function handleResume() {
    setPaused(false);
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }

  const statusLabel =
    streamState === "historical"
      ? `Complete — ${lines.length} log line${lines.length !== 1 ? "s" : ""}`
      : STATE_LABELS[streamState];

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${
              streamState === "connected"
                ? "bg-green-400"
                : streamState === "connecting"
                  ? "bg-yellow-400 animate-pulse"
                  : streamState === "error"
                    ? "bg-red-400"
                    : streamState === "historical"
                      ? "bg-blue-400"
                      : "bg-zinc-500"
            }`}
          />
          <span className="text-xs text-muted-foreground">{statusLabel}</span>
        </div>
        {streamState === "connected" && (
          <button
            onClick={() => (paused ? handleResume() : setPaused(true))}
            className="rounded-md border border-border px-2.5 py-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
          >
            {paused ? "Resume" : "Pause"}
          </button>
        )}
      </div>
      <div
        ref={containerRef}
        className="rounded-lg border border-border bg-zinc-950 p-4 font-mono text-xs text-green-300 overflow-y-auto max-h-80 min-h-[200px]"
      >
        {lines.length === 0 ? (
          <span className="text-muted-foreground">
            {streamState === "closed" || streamState === "historical"
              ? "No logs available."
              : "Waiting for logs..."}
          </span>
        ) : (
          lines.map((line, i) => (
            <div key={i} className="leading-5">
              {line}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
