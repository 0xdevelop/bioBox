import "./styles.css";
import { callTool, clearSession, hasSession, landCLIInput, restoreSession, saveSession } from "./api.ts";
import { pxdesignDefaultFile, protenixDefaultFile } from "./generated_test_data.ts";
import { mountFlywheel, type FlywheelLane } from "./flywheel.ts";
import { planSubmit } from "./submit_args.ts";
import { asRecord, isTaskDone, taskFromPayload } from "./task_view.ts";

type DefaultFile = { name: string; text: string };
type JobName = "protenix" | "pxdesign";
type JobView = {
  method: JobName;
  lines: string[];
  taskId: string;
  abort: AbortController | null;
  ticket: number;
};

const jobs: Record<JobName, JobView> = {
  protenix: { method: "protenix", lines: ["idle"], taskId: "", abort: null, ticket: 0 },
  pxdesign: { method: "pxdesign", lines: ["idle"], taskId: "", abort: null, ticket: 0 },
};

const loginScreen = document.querySelector<HTMLElement>("#login-screen");
const business = document.querySelector<HTMLElement>("#business");
const loginForm = document.querySelector<HTMLFormElement>("#login-form");
const loginState = document.querySelector<HTMLElement>("#login-state");
const logoutButton = document.querySelector<HTMLButtonElement>("#logout-submit");
const protenixForm = document.querySelector<HTMLFormElement>("#protenix-form");
const pxdesignForm = document.querySelector<HTMLFormElement>("#pxdesign-form");
const protenixFileName = document.querySelector<HTMLElement>("#protenix-file-name");
const pxdesignFileName = document.querySelector<HTMLElement>("#pxdesign-file-name");
const protenixOutput = document.querySelector<HTMLElement>("#protenix-output");
const pxdesignOutput = document.querySelector<HTMLElement>("#pxdesign-output");
const canvas = document.querySelector<HTMLCanvasElement>("#flywheel");

if (
  loginScreen === null ||
  business === null ||
  loginForm === null ||
  loginState === null ||
  logoutButton === null ||
  protenixForm === null ||
  pxdesignForm === null ||
  protenixFileName === null ||
  pxdesignFileName === null ||
  protenixOutput === null ||
  pxdesignOutput === null ||
  canvas === null
) {
  throw new Error("frontend controls are missing");
}

bindDefaultFile(protenixForm, protenixFileName, protenixDefaultFile);
bindDefaultFile(pxdesignForm, pxdesignFileName, pxdesignDefaultFile);
mountFlywheel(canvas, (): FlywheelLane[] => [
  { name: "PROTENIX", lines: jobs.protenix.lines },
  { name: "PXDESIGN", lines: jobs.pxdesign.lines },
]);

restoreSession();
if (hasSession()) {
  showBusiness();
} else {
  showLogin();
}

function showLogin(): void {
  loginScreen.hidden = false;
  business.hidden = true;
}

function showBusiness(): void {
  loginScreen.hidden = true;
  business.hidden = false;
}

function bindDefaultFile(form: HTMLFormElement, label: HTMLElement, fallback: DefaultFile): void {
  label.textContent = fallback.name;
  const field = form.elements.namedItem("file");
  if (!(field instanceof HTMLInputElement)) {
    return;
  }
  field.required = fallback.name === "";
  field.addEventListener("change", () => {
    label.textContent = field.files?.[0]?.name || fallback.name;
  });
}

function chosenFile(form: HTMLFormElement, fallback: DefaultFile): File | null {
  const field = form.elements.namedItem("file");
  const picked = field instanceof HTMLInputElement ? field.files?.[0] : undefined;
  if (picked) {
    return picked;
  }
  return fallback.name ? new File([fallback.text], fallback.name, { type: "application/json" }) : null;
}

function textValue(form: HTMLFormElement, name: string): string {
  const field = form.elements.namedItem(name);
  if (!(field instanceof HTMLInputElement)) {
    return "";
  }
  return field.value.trim();
}

const hiddenResultKeys = new Set(["jwt_token", "refresh_token", "password", "api_key"]);

function redact(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => redact(item));
  }
  const body = asRecord(value);
  if (body === null) {
    return value;
  }
  const copy: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(body)) {
    if (hiddenResultKeys.has(key)) {
      continue;
    }
    copy[key] = redact(item);
  }
  return copy;
}

function formatTask(value: unknown): string {
  const task = taskFromPayload(redact(value));
  if (task === null) {
    return JSON.stringify(value, null, 2);
  }
  const lines = [
    `task_id ${String(task.task_id ?? "")}`,
    `run_status ${String(task.run_status ?? "")}`,
    `process_status ${String(task.process_status ?? "")}`,
    `progress ${String(task.progress ?? "")} ${String(task.progress_message ?? "")}`.trim(),
  ];
  if (task.error_message) {
    lines.push(`error ${String(task.error_code ?? "")} ${String(task.error_message)}`);
  }
  const steps = Array.isArray(task.steps) ? task.steps : [];
  for (const step of steps) {
    const row = asRecord(step);
    if (row === null || row.step_kind !== "business") {
      continue;
    }
    const statuses = Array.isArray(row.business_statuses) ? row.business_statuses.join(",") : "";
    lines.push(`${String(row.step)} ${statuses} ${String(row.message ?? "")}`.trim());
  }
  if (task.result !== undefined) {
    lines.push(JSON.stringify(task.result));
  }
  return lines.filter((line) => line !== "").join("\n");
}

function permissionDenied(error: unknown): boolean {
  return error instanceof Error && error.message === "未登录";
}

async function watchTask(job: JobView, output: HTMLElement, taskId: string, ticket: number): Promise<void> {
  const controller = new AbortController();
  job.abort = controller;
  job.taskId = taskId;
  while (job.ticket === ticket && !controller.signal.aborted) {
    const outcome = await callTool("task.get", { task_id: taskId }, controller.signal);
    if (job.ticket !== ticket) {
      return;
    }
    const text = outcome.isError ? JSON.stringify(redact(outcome.value), null, 2) : formatTask(outcome.value);
    output.textContent = text;
    job.lines = text.split("\n").slice(0, 8);
    if (outcome.isError || isTaskDone(outcome.value)) {
      return;
    }
    await new Promise((resolve) => {
      const timer = setTimeout(resolve, 2000);
      controller.signal.addEventListener("abort", () => {
        clearTimeout(timer);
        resolve(undefined);
      }, { once: true });
    });
  }
}

async function submitJob(job: JobView, output: HTMLElement, file: File | null): Promise<void> {
  if (file === null) {
    output.textContent = "请选择输入文件";
    return;
  }
  job.abort?.abort();
  const ticket = job.ticket + 1;
  job.ticket = ticket;
  try {
    output.textContent = "读取输入";
    job.lines = ["reading input"];
    const fileText = await file.text();
    const plan = planSubmit(fileText);
    const args: Record<string, unknown> = {};
    if (plan.kind === "input_data") {
      Object.assign(args, plan.args);
      output.textContent = file.name;
    } else {
      output.textContent = "写入输入文件";
      const inputPath = await landCLIInput(job.method, file);
      args.input = inputPath;
      output.textContent = inputPath;
    }
    if (job.ticket !== ticket) {
      return;
    }
    job.lines = ["submitting", file.name];
    const outcome = await callTool(job.method, args);
    if (job.ticket !== ticket) {
      return;
    }
    if (outcome.isError) {
      const text = typeof outcome.value === "string" ? outcome.value : JSON.stringify(redact(outcome.value), null, 2);
      output.textContent = text;
      job.lines = text.split("\n").slice(0, 8);
      return;
    }
    const accepted = asRecord(outcome.value);
    const taskId = typeof accepted?.task_id === "string" ? accepted.task_id : "";
    output.textContent = formatTask(outcome.value);
    job.lines = [String(accepted?.run_status ?? "accepted"), taskId];
    if (taskId !== "") {
      await watchTask(job, output, taskId, ticket);
    }
  } catch (error: unknown) {
    if (job.ticket !== ticket) {
      return;
    }
    if (permissionDenied(error)) {
      clearSession();
      showLogin();
      loginState.textContent = "请重新登录";
      return;
    }
    const message = error instanceof Error ? error.message : String(error);
    output.textContent = message;
    job.lines = [message];
  }
}

loginForm.addEventListener("submit", (event) => {
  event.preventDefault();
  const email = textValue(loginForm, "email");
  const password = textValue(loginForm, "password");
  const passwordField = loginForm.elements.namedItem("password");
  if (passwordField instanceof HTMLInputElement) {
    passwordField.value = "";
  }
  void (async () => {
    if (email === "" || password === "") {
      loginState.textContent = "需要邮箱和密码";
      return;
    }
    try {
      const outcome = await callTool("auth.login.email", {
        login_method: "password",
        email,
        password,
      });
      if (outcome.isError) {
        loginState.textContent = typeof outcome.value === "string" ? outcome.value : "登录失败";
        return;
      }
      const token = asRecord(outcome.value)?.jwt_token;
      if (typeof token !== "string" || token === "") {
        loginState.textContent = "登录失败";
        return;
      }
      saveSession(token);
      loginState.textContent = "";
      showBusiness();
    } catch (error: unknown) {
      loginState.textContent = error instanceof Error ? error.message : "登录失败";
    }
  })();
});

logoutButton.addEventListener("click", () => {
  void (async () => {
    try {
      if (hasSession()) {
        await callTool("auth.logout", {});
      }
    } catch {
      // 本地会话照常清掉。
    }
    clearSession();
    showLogin();
  })();
});

protenixForm.addEventListener("submit", (event) => {
  event.preventDefault();
  void submitJob(jobs.protenix, protenixOutput, chosenFile(protenixForm, protenixDefaultFile));
});

pxdesignForm.addEventListener("submit", (event) => {
  event.preventDefault();
  void submitJob(jobs.pxdesign, pxdesignOutput, chosenFile(pxdesignForm, pxdesignDefaultFile));
});
