"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { CodeEditor } from "@/components/form/CodeEditor";
import { useBackends } from "@/lib/hooks";
import { createWorkload, ApiClientError } from "@/lib/api";
import { RUNTIMES, ISOLATION_MODES } from "@/lib/types";
import type { Runtime, IsolationMode } from "@/lib/types";

const DEFAULT_CPU = 1;
const DEFAULT_MEM_MB = 128;
const DEFAULT_TIMEOUT_S = 30;

export default function NewWorkloadPage() {
  const router = useRouter();
  const { data: backends } = useBackends();

  const [runtime, setRuntime] = useState<Runtime>("node");
  const [isolation, setIsolation] = useState<IsolationMode>("auto");
  const [code, setCode] = useState("");
  const [inputJson, setInputJson] = useState("");
  const [cpus, setCpus] = useState(DEFAULT_CPU);
  const [memMb, setMemMb] = useState(DEFAULT_MEM_MB);
  const [timeoutS, setTimeoutS] = useState(DEFAULT_TIMEOUT_S);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Determine available isolation modes from backends
  const availableIsolations = new Set(
    backends?.flatMap((b) => b.capabilities.supported_isolations) ?? [],
  );

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      let parsedInput: Record<string, unknown> | undefined;
      if (inputJson.trim()) {
        try {
          parsedInput = JSON.parse(inputJson);
        } catch {
          setError("Invalid JSON input");
          setSubmitting(false);
          return;
        }
      }

      const workload = await createWorkload({
        runtime,
        isolation,
        code: code || undefined,
        input: parsedInput,
        resources: {
          cpus,
          mem_mb: memMb,
          timeout_s: timeoutS,
        },
      });

      router.push(`/workloads/${workload.id}`);
    } catch (err) {
      if (err instanceof ApiClientError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred");
      }
      setSubmitting(false);
    }
  }

  return (
    <div>
      <PageHeader
        title="New Workload"
        description="Submit a workload for execution"
      />

      <form onSubmit={handleSubmit} className="max-w-3xl space-y-6">
        {error && (
          <div className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
            {error}
          </div>
        )}

        {/* Runtime */}
        <div>
          <label className="block text-sm font-medium mb-2">Runtime</label>
          <select
            value={runtime}
            onChange={(e) => setRuntime(e.target.value as Runtime)}
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground"
          >
            {RUNTIMES.map((r) => (
              <option key={r} value={r} className="capitalize">
                {r}
              </option>
            ))}
          </select>
        </div>

        {/* Isolation Mode */}
        <div>
          <label className="block text-sm font-medium mb-2">
            Isolation Mode
          </label>
          <select
            value={isolation}
            onChange={(e) => setIsolation(e.target.value as IsolationMode)}
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground"
          >
            {ISOLATION_MODES.map((mode) => {
              const isAvailable =
                mode === "auto" || availableIsolations.has(mode);
              return (
                <option key={mode} value={mode} disabled={!isAvailable}>
                  {mode}
                  {!isAvailable ? " (not available)" : ""}
                </option>
              );
            })}
          </select>
          {!availableIsolations.has(isolation) && isolation !== "auto" && (
            <p className="mt-1 text-xs text-yellow-400">
              This isolation backend is not currently available. The workload may
              fail.
            </p>
          )}
        </div>

        {/* Code Editor */}
        <div>
          <label className="block text-sm font-medium mb-2">Code</label>
          <CodeEditor value={code} onChange={setCode} runtime={runtime} />
        </div>

        {/* Input JSON */}
        <div>
          <label className="block text-sm font-medium mb-2">
            Input (JSON)
          </label>
          <textarea
            value={inputJson}
            onChange={(e) => setInputJson(e.target.value)}
            placeholder='{"key": "value"}'
            rows={4}
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono text-foreground placeholder:text-muted-foreground"
          />
        </div>

        {/* Resource Limits */}
        <div>
          <label className="block text-sm font-medium mb-2">
            Resource Limits
          </label>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-xs text-muted-foreground mb-1">
                CPUs
              </label>
              <input
                type="number"
                min={1}
                max={16}
                value={cpus}
                onChange={(e) => setCpus(Number(e.target.value))}
                className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground"
              />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">
                Memory (MB)
              </label>
              <input
                type="number"
                min={32}
                max={8192}
                value={memMb}
                onChange={(e) => setMemMb(Number(e.target.value))}
                className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground"
              />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">
                Timeout (s)
              </label>
              <input
                type="number"
                min={1}
                max={300}
                value={timeoutS}
                onChange={(e) => setTimeoutS(Number(e.target.value))}
                className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground"
              />
            </div>
          </div>
        </div>

        {/* Submit */}
        <button
          type="submit"
          disabled={submitting}
          className="rounded-md bg-accent px-6 py-2.5 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {submitting ? "Submitting..." : "Submit Workload"}
        </button>
      </form>
    </div>
  );
}
