export type JobName = "protenix" | "pxdesign";

export type SubmitPlan =
  | { kind: "input_data"; args: Record<string, unknown> }
  | { kind: "path" };

export function planSubmit(fileText: string): SubmitPlan {
  try {
    const data: unknown = JSON.parse(fileText);
    if (typeof data === "object" && data !== null) {
      return { kind: "input_data", args: { input_data: data } };
    }
  } catch {
    // YAML or invalid JSON keeps the /cli-input path compatibility.
  }
  return { kind: "path" };
}
