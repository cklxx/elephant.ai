export const MAX_TOOL_STREAM_CHUNKS = 200;
export const MAX_TOOL_STREAM_BYTES = 64 * 1024;

export function appendBoundedToolStreamChunk(chunks: string[], chunk: string): void {
  let nextChunk = chunk;
  if (nextChunk.length > MAX_TOOL_STREAM_BYTES) {
    nextChunk = nextChunk.slice(nextChunk.length - MAX_TOOL_STREAM_BYTES);
  }

  chunks.push(nextChunk);

  if (chunks.length > MAX_TOOL_STREAM_CHUNKS) {
    chunks.splice(0, chunks.length - MAX_TOOL_STREAM_CHUNKS);
  }

  let totalBytes = 0;
  for (let i = chunks.length - 1; i >= 0; i--) {
    totalBytes += chunks[i].length;
    if (totalBytes > MAX_TOOL_STREAM_BYTES) {
      chunks.splice(0, i + 1);
      break;
    }
  }
}
