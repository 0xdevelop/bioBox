import { expect, test } from "bun:test";
import { isTaskDone, taskFromPayload } from "./task_view.ts";

test("flat task payload is treated as done", () => {
  const payload = { task_id: "abc", run_status: "done", process_status: "success" };
  expect(taskFromPayload(payload)?.run_status).toBe("done");
  expect(isTaskDone(payload)).toBe(true);
});

test("nested task payload is treated as done", () => {
  const payload = { task: { task_id: "abc", run_status: "done" } };
  expect(isTaskDone(payload)).toBe(true);
});

test("running task is not done", () => {
  expect(isTaskDone({ run_status: "running" })).toBe(false);
  expect(isTaskDone({ task: { run_status: "running" } })).toBe(false);
});
