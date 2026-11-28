import { asRecord } from "./task_view.ts";

let verifiedRole = "";

/**
 * 从登录或 auth.jwt_token.check 的服务端响应中取出 role。
 * 只认服务端返回的载荷，不解析 JWT，也不接受客户端自行拼造的权限声明作为持久状态。
 */
export function roleFromServerPayload(payload: unknown): string {
  const body = asRecord(payload);
  if (body === null) {
    return "";
  }
  if (body.role === "admin" || body.role === "user") {
    return body.role;
  }
  const user = asRecord(body.user);
  if (user?.role === "admin" || user?.role === "user") {
    return user.role;
  }
  return "";
}

export function rememberVerifiedRole(payload: unknown): void {
  verifiedRole = roleFromServerPayload(payload);
}

export function clearVerifiedRole(): void {
  verifiedRole = "";
}

/** 仅当 rememberVerifiedRole 已写入服务端校验的 admin 时为 true。 */
export function canManage(): boolean {
  return verifiedRole === "admin";
}
