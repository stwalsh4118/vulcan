/**
 * Streaming SSE proxy for workload logs.
 *
 * Next.js rewrites() buffers SSE responses. This route handler fetches the
 * upstream SSE stream from the Go backend and pipes it through an explicit
 * ReadableStream to the client, preserving real-time delivery.
 */

const BACKEND_BASE_URL = "http://localhost:8080";

export const dynamic = "force-dynamic";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;

  const upstream = await fetch(`${BACKEND_BASE_URL}/v1/workloads/${id}/logs`, {
    headers: { Accept: "text/event-stream" },
    cache: "no-store",
    signal: request.signal,
  });

  if (!upstream.ok) {
    return new Response(upstream.body, { status: upstream.status });
  }

  if (!upstream.body) {
    return new Response(null, { status: 200 });
  }

  // Explicitly pipe the upstream stream chunk-by-chunk to prevent buffering.
  const reader = upstream.body.getReader();
  const stream = new ReadableStream({
    start(controller) {
      (async () => {
        try {
          for (;;) {
            const { done, value } = await reader.read();
            if (done) {
              controller.close();
              return;
            }
            controller.enqueue(value);
          }
        } catch {
          controller.close();
        }
      })();
    },
    cancel() {
      reader.cancel();
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
      "Content-Encoding": "none",
    },
  });
}
