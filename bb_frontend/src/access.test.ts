import { expect, test } from "bun:test";
import { canManage, clearVerifiedRole, rememberVerifiedRole, roleFromServerPayload } from "./access.ts";

test("canManage stays false until a server payload is remembered", () => {
  clearVerifiedRole();
  expect(canManage()).toBe(false);
  expect(roleFromServerPayload({ role: "admin" })).toBe("admin");
  expect(canManage()).toBe(false);
});

test("invented local objects do not grant access until remembered from a server payload", () => {
  clearVerifiedRole();
  expect(canManage()).toBe(false);
  rememberVerifiedRole({ user: { role: "user", user_name: "bob" } });
  expect(canManage()).toBe(false);
  rememberVerifiedRole({ role: "admin", user: { role: "admin", user_name: "alice" } });
  expect(canManage()).toBe(true);
  clearVerifiedRole();
  expect(canManage()).toBe(false);
});

test("unknown role values are not treated as admin", () => {
  clearVerifiedRole();
  rememberVerifiedRole({ role: "superadmin", is_admin: true });
  expect(canManage()).toBe(false);
});
