/**
 * Streaming SSE proxy for workload logs.
 *
 * Next.js rewrites() buffers SSE responses. This route handler fetches the
 * upstream SSE stream from the Go backend and pipes it to the client as a
 * ReadableStream, preserving real-time delivery.
 */

const BACKEND_BASE_URL = "http://localhost:8080";

export const dynamic = "force-dynamic";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;

  const upstream = await fetch(`${BACKEND_BASE_URL}/v1/workloads/${id}/logs`, {
    headers: { Accept: "text/event-stream" },
  });

  if (!upstream.ok) {
    return new Response(upstream.body, { status: upstream.status });
  }

  return new Response(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
