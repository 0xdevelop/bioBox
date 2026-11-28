import { expect, test } from "bun:test";
import { isAppPath, normalizePath, pathForPage, routeFromPath } from "./routes.ts";

test("root and business are the business page", () => {
  expect(routeFromPath("/").page).toBe("business");
  expect(routeFromPath("/business").page).toBe("business");
  expect(routeFromPath("/business/").page).toBe("business");
  expect(isAppPath("/")).toBe(true);
  expect(isAppPath("/business")).toBe(true);
});

test("manage path is distinct and normalizes trailing slash", () => {
  expect(normalizePath("/manage/")).toBe("/manage");
  expect(routeFromPath("/manage").page).toBe("manage");
  expect(routeFromPath("/manage/").page).toBe("manage");
  expect(isAppPath("/manage")).toBe(true);
  expect(pathForPage("manage")).toBe("/manage");
  expect(pathForPage("business")).toBe("/business");
});

test("unknown paths are not app pages and do not become manage", () => {
  expect(isAppPath("/admin")).toBe(false);
  expect(isAppPath("/index.js")).toBe(false);
  expect(routeFromPath("/admin").page).toBe("business");
});
