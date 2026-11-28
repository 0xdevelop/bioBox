import { asRecord } from "./task_view.ts";

export function tasksFromListPayload(value: unknown): Record<string, unknown>[] {
  const body = asRecord(value);
  const raw = body?.tasks;
  if (!Array.isArray(raw)) {
    return [];
  }
  const tasks: Record<string, unknown>[] = [];
  for (const item of raw) {
    const task = asRecord(item);
    if (task !== null) {
      tasks.push(task);
    }
  }
  return tasks;
}

export function canCancelTask(task: Record<string, unknown>): boolean {
  return task.run_status !== "done";
}

export function canListArtifacts(task: Record<string, unknown>): boolean {
  return task.run_status === "done";
}

export type ArtifactInfo = {
  path: string;
  name: string;
  kind: string;
  size: number;
  content_type: string;
};

export function artifactsFromPayload(value: unknown): ArtifactInfo[] {
  const body = asRecord(value);
  const raw = body?.artifacts;
  if (!Array.isArray(raw)) {
    return [];
  }
  const artifacts: ArtifactInfo[] = [];
  for (const item of raw) {
    const row = asRecord(item);
    const path = typeof row?.path === "string" ? row.path : "";
    if (path === "") {
      continue;
    }
    artifacts.push({
      path,
      name: typeof row?.name === "string" && row.name !== "" ? row.name : path,
      kind: typeof row?.kind === "string" ? row.kind : "",
      size: typeof row?.size === "number" ? row.size : 0,
      content_type: typeof row?.content_type === "string" ? row.content_type : "",
    });
  }
  return artifacts;
}

export type ArtifactFile = {
  name: string;
  content_type: string;
  content: string;
};

export function artifactFileFromPayload(value: unknown): ArtifactFile | null {
  const body = asRecord(value);
  const content = typeof body?.content === "string" ? body.content : "";
  const encoding = typeof body?.encoding === "string" ? body.encoding : "base64";
  if (content === "" || encoding !== "base64") {
    return null;
  }
  const name = typeof body?.name === "string" && body.name !== ""
    ? body.name
    : typeof body?.path === "string" ? String(body.path).split("/").pop() ?? "artifact" : "artifact";
  return {
    name,
    content_type: typeof body?.content_type === "string" && body.content_type !== ""
      ? body.content_type
      : "application/octet-stream",
    content,
  };
}

export function decodeBase64Content(content: string): Uint8Array {
  const binary = atob(content);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}
