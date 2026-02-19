"use client";

import CodeMirror from "@uiw/react-codemirror";
import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { oneDark } from "@codemirror/theme-one-dark";
import type { Runtime } from "@/lib/types";

const LANGUAGE_EXTENSIONS: Partial<Record<Runtime, ReturnType<typeof javascript | typeof python>>> = {
  node: javascript({ jsx: false, typescript: true }),
  python: python(),
  go: undefined,
  wasm: javascript({ jsx: false, typescript: false }),
  oci: undefined,
};

const PLACEHOLDER_CODE: Partial<Record<Runtime, string>> = {
  node: 'console.log("Hello from Node.js");',
  python: 'print("Hello from Python")',
  go: 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello from Go")\n}',
};

export function CodeEditor({
  value,
  onChange,
  runtime,
}: {
  value: string;
  onChange: (value: string) => void;
  runtime: Runtime;
}) {
  const langExtension = LANGUAGE_EXTENSIONS[runtime];
  const extensions = langExtension ? [langExtension] : [];

  return (
    <div className="rounded-md border border-border overflow-hidden">
      <CodeMirror
        value={value}
        height="300px"
        theme={oneDark}
        extensions={extensions}
        onChange={onChange}
        placeholder={PLACEHOLDER_CODE[runtime] ?? "Enter your code here..."}
      />
    </div>
  );
}
