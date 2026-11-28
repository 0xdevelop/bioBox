import { expect, test } from "bun:test";
import {
  artifactFileFromPayload,
  artifactsFromPayload,
  canCancelTask,
  canListArtifacts,
  decodeBase64Content,
  tasksFromListPayload,
} from "./tasks.ts";

test("parses owned task list and ignores non-objects", () => {
  const tasks = tasksFromListPayload({
    tasks: [
      { task_id: "a", run_status: "running", method: "protenix" },
      "skip",
      { task_id: "b", run_status: "done", method: "pxdesign" },
    ],
  });
  expect(tasks).toHaveLength(2);
  expect(canCancelTask(tasks[0])).toBe(true);
  expect(canListArtifacts(tasks[0])).toBe(false);
  expect(canCancelTask(tasks[1])).toBe(false);
  expect(canListArtifacts(tasks[1])).toBe(true);
});

test("empty or malformed list is empty", () => {
  expect(tasksFromListPayload({})).toEqual([]);
  expect(tasksFromListPayload(null)).toEqual([]);
  expect(tasksFromListPayload({ tasks: "nope" })).toEqual([]);
});

test("parses artifact list and file payload, decodes base64", () => {
  const listed = artifactsFromPayload({
    task_id: "t",
    artifacts: [
      { path: "output/model.cif", name: "model.cif", kind: "cif", size: 4, content_type: "chemical/x-cif" },
      { path: "" },
    ],
  });
  expect(listed).toEqual([
    { path: "output/model.cif", name: "model.cif", kind: "cif", size: 4, content_type: "chemical/x-cif" },
  ]);
  const file = artifactFileFromPayload({
    name: "model.cif",
    content_type: "chemical/x-cif",
    encoding: "base64",
    content: "YWJjZA==",
  });
  expect(file?.name).toBe("model.cif");
  expect(new TextDecoder().decode(decodeBase64Content(file!.content))).toBe("abcd");
  expect(artifactFileFromPayload({ content: "YWJjZA==", encoding: "utf8" })).toBeNull();
});
