"use client";

import { useState, useRef } from "react";
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
// 10 MB pre-encoding; backend allows 15 MB to account for base64 overhead.
const MAX_ARCHIVE_SIZE = 10 * 1024 * 1024;

const ISOLATION_LABELS: Record<string, string> = {
  auto: "Auto",
  microvm: "MicroVM (Firecracker)",
  isolate: "Isolate",
  gvisor: "gVisor",
};

type CodeMode = "inline" | "archive";

export default function NewWorkloadPage() {
  const router = useRouter();
  const { data: backends } = useBackends();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [runtime, setRuntime] = useState<Runtime>("node");
  const [isolation, setIsolation] = useState<IsolationMode>("auto");
  const [code, setCode] = useState("");
  const [codeMode, setCodeMode] = useState<CodeMode>("inline");
  const [archiveFile, setArchiveFile] = useState<File | null>(null);
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

  function validateAndSetFile(file: File): boolean {
    if (!file.name.endsWith(".tar.gz") && !file.name.endsWith(".tgz")) {
      setError("Only .tar.gz archives are accepted");
      return false;
    }
    if (file.size > MAX_ARCHIVE_SIZE) {
      setError(`Archive must be under ${MAX_ARCHIVE_SIZE / (1024 * 1024)} MB`);
      return false;
    }
    setError(null);
    setArchiveFile(file);
    return true;
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!validateAndSetFile(file)) {
      e.target.value = "";
    }
  }

  function readFileAsBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const result = reader.result as string;
        // Remove the data URL prefix (e.g., "data:application/gzip;base64,")
        const base64 = result.split(",")[1];
        resolve(base64);
      };
      reader.onerror = reject;
      reader.readAsDataURL(file);
    });
  }

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

      let codeArchive: string | undefined;
      let codeValue: string | undefined;

      if (codeMode === "archive") {
        if (!archiveFile) {
          setError("Please select a .tar.gz archive");
          setSubmitting(false);
          return;
        }
        codeArchive = await readFileAsBase64(archiveFile);
      } else {
        codeValue = code || undefined;
      }

      const workload = await createWorkload({
        runtime,
        isolation,
        code: codeValue,
        code_archive: codeArchive,
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
                  {ISOLATION_LABELS[mode] ?? mode}
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

        {/* Code Mode Toggle */}
        <div>
          <label className="block text-sm font-medium mb-2">Code Source</label>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setCodeMode("inline")}
              className={`rounded-md px-4 py-1.5 text-sm font-medium transition-colors ${
                codeMode === "inline"
                  ? "bg-accent text-white"
                  : "bg-muted text-muted-foreground hover:text-foreground"
              }`}
            >
              Inline Code
            </button>
            <button
              type="button"
              onClick={() => setCodeMode("archive")}
              className={`rounded-md px-4 py-1.5 text-sm font-medium transition-colors ${
                codeMode === "archive"
                  ? "bg-accent text-white"
                  : "bg-muted text-muted-foreground hover:text-foreground"
              }`}
            >
              Upload Archive
            </button>
          </div>
        </div>

        {/* Code Editor (inline mode) */}
        {codeMode === "inline" && (
          <div>
            <label className="block text-sm font-medium mb-2">Code</label>
            <CodeEditor value={code} onChange={setCode} runtime={runtime} />
          </div>
        )}

        {/* File Upload (archive mode) */}
        {codeMode === "archive" && (
          <div>
            <label className="block text-sm font-medium mb-2">
              Archive (.tar.gz)
            </label>
            <div
              onClick={() => fileInputRef.current?.click()}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault();
                const file = e.dataTransfer.files[0];
                if (file) {
                  validateAndSetFile(file);
                }
              }}
              className="cursor-pointer rounded-lg border-2 border-dashed border-border bg-muted/30 p-8 text-center transition-colors hover:border-accent/50 hover:bg-muted/50"
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".tar.gz,.tgz"
                onChange={handleFileSelect}
                className="hidden"
              />
              {archiveFile ? (
                <div>
                  <p className="text-sm font-medium">{archiveFile.name}</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    {(archiveFile.size / 1024).toFixed(1)} KB
                  </p>
                </div>
              ) : (
                <div>
                  <p className="text-sm text-muted-foreground">
                    Click or drag a .tar.gz file here
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Max {MAX_ARCHIVE_SIZE / (1024 * 1024)} MB
                  </p>
                </div>
              )}
            </div>
          </div>
        )}

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
