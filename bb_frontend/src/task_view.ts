export function asRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value === "object" && value !== null && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return null;
}

export function taskFromPayload(value: unknown): Record<string, unknown> | null {
  const body = asRecord(value);
  if (body === null) {
    return null;
  }
  return asRecord(body.task) ?? body;
}

export function isTaskDone(value: unknown): boolean {
  return taskFromPayload(value)?.run_status === "done";
}
