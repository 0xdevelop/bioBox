import { expect, test } from "bun:test";
import { planSubmit } from "./submit_args.ts";

test("json object uses input_data", () => {
  const plan = planSubmit(`{"name":"ten_alanine"}`);
  expect(plan.kind).toBe("input_data");
  if (plan.kind === "input_data") {
    expect(plan.args.input_data).toEqual({ name: "ten_alanine" });
  }
});

test("yaml or invalid json falls back to path upload", () => {
  expect(planSubmit("name: src\n").kind).toBe("path");
  expect(planSubmit("").kind).toBe("path");
});
