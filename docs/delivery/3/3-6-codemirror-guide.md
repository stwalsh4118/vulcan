# CodeMirror (@uiw/react-codemirror) — Usage Guide

**Date**: 2026-02-19
**Version**: @uiw/react-codemirror 4.25.4, @codemirror/lang-javascript 6.2.4, @codemirror/lang-python 6.2.1
**Docs**: https://uiw.github.io/react-codemirror/

## Installation

```bash
pnpm add @uiw/react-codemirror @codemirror/lang-javascript @codemirror/lang-python @codemirror/theme-one-dark
```

## Basic Usage (React)

```tsx
"use client";

import CodeMirror from "@uiw/react-codemirror";
import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { oneDark } from "@codemirror/theme-one-dark";

function Editor() {
  const [value, setValue] = useState("console.log('hello')");

  return (
    <CodeMirror
      value={value}
      height="300px"
      theme={oneDark}
      extensions={[javascript({ jsx: true, typescript: true })]}
      onChange={(val) => setValue(val)}
    />
  );
}
```

## Language Extensions

| Runtime | Extension |
|---------|-----------|
| JS/TS   | `javascript({ jsx: true, typescript: true })` |
| Python  | `python()` |
| Go      | No official extension; use plain text |

## Key Props

- `value`: Controlled string content
- `onChange`: `(value: string, viewUpdate: ViewUpdate) => void`
- `height`: CSS height string
- `theme`: Theme object (oneDark for dark mode)
- `extensions`: Array of CodeMirror extensions (language, etc.)
- `readOnly`: Boolean
- `placeholder`: Placeholder text

## Next.js Notes

- MUST be rendered as client component (`"use client"`)
- Works with Next.js App Router — no dynamic import needed with `@uiw/react-codemirror`
