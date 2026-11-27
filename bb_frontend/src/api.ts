declare const BIOBOX_MCP_URL: string;

export type ToolCallOutcome = {
  isError: boolean;
  value: unknown;
};

type RpcEnvelope = {
  error?: { message?: string };
  result?: {
    isError?: boolean;
    content?: Array<{ type?: string; text?: string }>;
  };
};

const protocolVersion = "2026-07-28";
const sessionKey = "biobox.session";

let bearerToken = "";
let requestID = 0;

export function mcpCallURL(): string {
  const trimmed = BIOBOX_MCP_URL.trim();
  if (trimmed !== "") {
    return new URL(trimmed.includes("://") ? trimmed : `http://${trimmed}`).href;
  }
  const port = Number(document.querySelector<HTMLMetaElement>('meta[name="biobox-mcp-port"]')?.content);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error("此页面未配置可用的 MCP 端口");
  }
  const url = new URL("/", window.location.origin);
  url.port = String(port);
  return url.href;
}

export function restoreSession(): string {
  bearerToken = sessionStorage.getItem(sessionKey) ?? "";
  return bearerToken;
}

export function saveSession(token: string): void {
  bearerToken = token.trim();
  if (bearerToken === "") {
    sessionStorage.removeItem(sessionKey);
    return;
  }
  sessionStorage.setItem(sessionKey, bearerToken);
}

export function clearSession(): void {
  bearerToken = "";
  sessionStorage.removeItem(sessionKey);
}

export function hasSession(): boolean {
  return bearerToken !== "";
}

export async function landCLIInput(suite: "protenix" | "pxdesign", file: File): Promise<string> {
  const headers: Record<string, string> = {};
  if (bearerToken !== "") {
    headers.Authorization = `Bearer ${bearerToken}`;
  }
  const form = new FormData();
  form.set("suite", suite);
  form.set("file", file, file.name);
  const response = await fetch("/cli-input", { method: "POST", headers, body: form });
  if (response.status === 403) {
    throw new Error("未登录");
  }
  const raw = await response.text();
  let payload: { path?: string };
  try {
    payload = JSON.parse(raw) as { path?: string };
  } catch {
    throw new Error(`输入文件没有写入（HTTP ${response.status}）`);
  }
  if (response.status !== 200 || typeof payload.path !== "string" || payload.path === "") {
    throw new Error(`输入文件没有写入（HTTP ${response.status}）`);
  }
  return payload.path;
}

export async function callTool(
  name: string,
  args: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ToolCallOutcome> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "application/json, text/event-stream",
    "Mcp-Protocol-Version": protocolVersion,
    "Mcp-Method": "tools/call",
    "Mcp-Name": name,
  };
  if (bearerToken !== "") {
    headers.Authorization = `Bearer ${bearerToken}`;
  }
  const response = await fetch(mcpCallURL(), {
    method: "POST",
    headers,
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: ++requestID,
      method: "tools/call",
      params: {
        name,
        arguments: args,
        _meta: {
          "io.modelcontextprotocol/protocolVersion": protocolVersion,
          "io.modelcontextprotocol/clientInfo": { name: "bioBox-web", version: "1.0.0" },
          "io.modelcontextprotocol/clientCapabilities": { extensions: {} },
        },
      },
    }),
    signal,
  });
  const raw = await response.text();
  let envelope: RpcEnvelope;
  try {
    envelope = JSON.parse(raw) as RpcEnvelope;
  } catch {
    throw new Error(`MCP 响应不是 JSON（HTTP ${response.status}）`);
  }
  if (envelope.error) {
    throw new Error(envelope.error.message || "MCP 协议错误");
  }
  const result = envelope.result;
  if (!result) {
    throw new Error(`MCP 没有 result（HTTP ${response.status}）`);
  }
  const text = result.content?.find((item) => typeof item.text === "string")?.text ?? "";
  let value: unknown = text;
  if (text !== "") {
    try {
      value = JSON.parse(text) as unknown;
    } catch {
      value = text;
    }
  }
  return { isError: result.isError === true, value };
}
