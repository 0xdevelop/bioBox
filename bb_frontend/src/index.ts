import "./styles.css";
import { callTool, clearSession, hasSession, landCLIInput, restoreSession, saveSession } from "./api.ts";
import { canManage, clearVerifiedRole, rememberVerifiedRole } from "./access.ts";
import { pxdesignDefaultFile, protenixDefaultFile } from "./generated_test_data.ts";
import { mountFlywheel, type FlywheelLane } from "./flywheel.ts";
import { pathForPage, routeFromPath, type AppPage } from "./routes.ts";
import { planSubmit } from "./submit_args.ts";
import {
  artifactFileFromPayload,
  artifactsFromPayload,
  canCancelTask,
  canListArtifacts,
  decodeBase64Content,
  tasksFromListPayload,
} from "./tasks.ts";
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
const appShell = document.querySelector<HTMLElement>("#app-shell");
const pageBusiness = document.querySelector<HTMLElement>("#page-business");
const pageManage = document.querySelector<HTMLElement>("#page-manage");
const navBusiness = document.querySelector<HTMLAnchorElement>("#nav-business");
const navManage = document.querySelector<HTMLAnchorElement>("#nav-manage");
const manageAccess = document.querySelector<HTMLElement>("#manage-access");
const manageInstanceDl = document.querySelector<HTMLElement>("#manage-instance-dl");
const manageBack = document.querySelector<HTMLAnchorElement>("#manage-back");
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
const taskList = document.querySelector<HTMLElement>("#task-list");
const taskDetail = document.querySelector<HTMLElement>("#task-detail");
const refreshTasks = document.querySelector<HTMLButtonElement>("#refresh-tasks");
const manageTools = document.querySelector<HTMLElement>("#manage-tools");
const userList = document.querySelector<HTMLElement>("#user-list");
const refreshUsers = document.querySelector<HTMLButtonElement>("#refresh-users");
const adminTaskList = document.querySelector<HTMLElement>("#admin-task-list");
const adminTaskDetail = document.querySelector<HTMLElement>("#admin-task-detail");
const refreshAdminTasks = document.querySelector<HTMLButtonElement>("#refresh-admin-tasks");
const resourceSet = document.querySelector<HTMLSelectElement>("#resource-set");
const resourceStatus = document.querySelector<HTMLButtonElement>("#resource-status");
const resourcePrepare = document.querySelector<HTMLButtonElement>("#resource-prepare");
const resourceDetail = document.querySelector<HTMLElement>("#resource-detail");
const sessionList = document.querySelector<HTMLElement>("#session-list");
const refreshSessions = document.querySelector<HTMLButtonElement>("#refresh-sessions");

if (
  loginScreen === null ||
  appShell === null ||
  pageBusiness === null ||
  pageManage === null ||
  navBusiness === null ||
  navManage === null ||
  manageAccess === null ||
  manageInstanceDl === null ||
  manageBack === null ||
  loginForm === null ||
  loginState === null ||
  logoutButton === null ||
  protenixForm === null ||
  pxdesignForm === null ||
  protenixFileName === null ||
  pxdesignFileName === null ||
  protenixOutput === null ||
  pxdesignOutput === null ||
  canvas === null ||
  taskList === null ||
  taskDetail === null ||
  refreshTasks === null ||
  manageTools === null ||
  userList === null ||
  refreshUsers === null ||
  adminTaskList === null ||
  adminTaskDetail === null ||
  refreshAdminTasks === null ||
  resourceSet === null ||
  resourceStatus === null ||
  resourcePrepare === null ||
  resourceDetail === null ||
  sessionList === null ||
  refreshSessions === null
) {
  throw new Error("frontend controls are missing");
}

bindDefaultFile(protenixForm, protenixFileName, protenixDefaultFile);
bindDefaultFile(pxdesignForm, pxdesignFileName, pxdesignDefaultFile);
mountFlywheel(canvas, (): FlywheelLane[] => [
  { name: "PROTENIX", lines: jobs.protenix.lines },
  { name: "PXDESIGN", lines: jobs.pxdesign.lines },
]);

void hydrateSession();
window.addEventListener("popstate", () => {
  applyRoute();
});

function currentPage(): AppPage {
  return routeFromPath(window.location.pathname).page;
}

function goTo(page: AppPage): void {
  const next = page === "business" && (window.location.pathname === "/" || window.location.pathname === "")
    ? "/"
    : pathForPage(page);
  if (window.location.pathname !== next) {
    history.pushState({ page }, "", next);
  }
  applyRoute();
}

function applyRoute(): void {
  const page = currentPage();
  if (!hasSession()) {
    loginScreen.hidden = false;
    appShell.hidden = true;
    document.title = "bioBox · 登录";
    return;
  }
  loginScreen.hidden = true;
  appShell.hidden = false;
  const allowed = canManage();
  navManage.hidden = !allowed;
  pageBusiness.hidden = page !== "business";
  pageManage.hidden = page !== "manage";
  navBusiness.setAttribute("aria-current", page === "business" ? "page" : "false");
  navManage.setAttribute("aria-current", page === "manage" ? "page" : "false");
  document.title = page === "manage" ? "bioBox · 管理" : "bioBox · 业务";
  if (page === "manage") {
    renderManagePage(allowed);
  } else {
    void loadTaskList();
  }
}

async function hydrateSession(): Promise<void> {
  restoreSession();
  if (!hasSession()) {
    clearVerifiedRole();
    applyRoute();
    return;
  }
  try {
    const outcome = await callTool("auth.jwt_token.check", {});
    if (outcome.isError) {
      clearSession();
      clearVerifiedRole();
      applyRoute();
      return;
    }
    rememberVerifiedRole(outcome.value);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    clearVerifiedRole();
  }
  applyRoute();
}

function renderManagePage(allowed: boolean): void {
  if (allowed) {
    manageAccess.textContent = "当前账号具有管理权限。角色以服务端数据库为准。";
    manageTools.hidden = false;
    void loadAdminUsers();
    void loadAdminTasks();
    void loadResourceSets();
    void loadAdminSessions();
  } else {
    manageAccess.textContent =
      "当前账号没有管理权限。管理入口仅对服务端确认的 admin 可见；直接访问本页不会开放操作。";
    manageTools.hidden = true;
  }
  manageInstanceDl.replaceChildren();
  addDefinition(manageInstanceDl, "页面地址", window.location.origin);
  const port = document.querySelector<HTMLMetaElement>('meta[name="biobox-mcp-port"]')?.content.trim() ?? "";
  addDefinition(manageInstanceDl, "MCP 端口", port === "" ? "未注入" : port);
}

function addDefinition(list: HTMLElement, title: string, value: string): void {
  const dt = document.createElement("dt");
  dt.textContent = title;
  const dd = document.createElement("dd");
  dd.textContent = value;
  list.append(dt, dd);
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

function handleAuthFailure(error: unknown): boolean {
  if (!permissionDenied(error)) {
    return false;
  }
  clearSession();
  clearVerifiedRole();
  applyRoute();
  loginState.textContent = "请重新登录";
  return true;
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
      void loadTaskList();
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
    void loadTaskList();
    if (taskId !== "") {
      await watchTask(job, output, taskId, ticket);
    }
  } catch (error: unknown) {
    if (job.ticket !== ticket) {
      return;
    }
    if (handleAuthFailure(error)) {
      return;
    }
    const message = error instanceof Error ? error.message : String(error);
    output.textContent = message;
    job.lines = [message];
  }
}

async function loadTaskList(): Promise<void> {
  if (!hasSession() || currentPage() !== "business") {
    return;
  }
  try {
    const outcome = await callTool("task.list", {});
    if (outcome.isError) {
      taskList.textContent = typeof outcome.value === "string" ? outcome.value : "无法加载任务列表";
      return;
    }
    renderTaskList(tasksFromListPayload(outcome.value));
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    taskList.textContent = error instanceof Error ? error.message : "无法加载任务列表";
  }
}

function renderTaskList(tasks: Record<string, unknown>[]): void {
  taskList.replaceChildren();
  if (tasks.length === 0) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = "暂无任务";
    taskList.append(empty);
    return;
  }
  for (const task of tasks) {
    taskList.append(renderTaskRow(task));
  }
}

function renderTaskRow(task: Record<string, unknown>): HTMLElement {
  const row = document.createElement("article");
  row.className = "task-row";
  const taskId = typeof task.task_id === "string" ? task.task_id : "";
  const meta = document.createElement("p");
  meta.className = "task-meta";
  meta.textContent = [
    String(task.method ?? ""),
    String(task.run_status ?? ""),
    String(task.process_status ?? ""),
    typeof task.created_at === "string" ? task.created_at : "",
    taskId,
  ].filter((part) => part !== "").join("  ·  ");
  const actions = document.createElement("div");
  actions.className = "task-actions";
  const view = document.createElement("button");
  view.type = "button";
  view.textContent = "查看";
  view.addEventListener("click", () => {
    void inspectTask(taskId);
  });
  actions.append(view);
  if (canCancelTask(task) && taskId !== "") {
    const cancel = document.createElement("button");
    cancel.type = "button";
    cancel.textContent = "取消";
    cancel.addEventListener("click", () => {
      void cancelTask(taskId);
    });
    actions.append(cancel);
  }
  if (canListArtifacts(task) && taskId !== "") {
    const artifacts = document.createElement("button");
    artifacts.type = "button";
    artifacts.textContent = "产物";
    artifacts.addEventListener("click", () => {
      void loadArtifacts(taskId, row);
    });
    actions.append(artifacts);
  }
  row.append(meta, actions);
  return row;
}

async function inspectTask(taskId: string): Promise<void> {
  if (taskId === "") {
    return;
  }
  try {
    const outcome = await callTool("task.get", { task_id: taskId });
    taskDetail.textContent = outcome.isError
      ? JSON.stringify(redact(outcome.value), null, 2)
      : formatTask(outcome.value);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    taskDetail.textContent = error instanceof Error ? error.message : "无法读取任务";
  }
}

async function cancelTask(taskId: string): Promise<void> {
  if (!window.confirm("取消该任务？")) {
    return;
  }
  try {
    const outcome = await callTool("task.cancel", { task_id: taskId });
    taskDetail.textContent = outcome.isError
      ? JSON.stringify(redact(outcome.value), null, 2)
      : JSON.stringify(redact(outcome.value), null, 2);
    await loadTaskList();
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    taskDetail.textContent = error instanceof Error ? error.message : "无法取消任务";
  }
}

async function loadArtifacts(taskId: string, row: HTMLElement): Promise<void> {
  try {
    const outcome = await callTool("task.artifacts", { task_id: taskId });
    if (outcome.isError) {
      taskDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
      return;
    }
    const artifacts = artifactsFromPayload(outcome.value);
    let list = row.querySelector<HTMLElement>(".artifact-list");
    if (list === null) {
      list = document.createElement("ul");
      list.className = "artifact-list";
      row.append(list);
    }
    list.replaceChildren();
    if (artifacts.length === 0) {
      const empty = document.createElement("li");
      empty.className = "hint";
      empty.textContent = "没有可下载产物";
      list.append(empty);
      return;
    }
    for (const artifact of artifacts) {
      const item = document.createElement("li");
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = `下载 ${artifact.path}`;
      button.addEventListener("click", () => {
        void downloadArtifact(taskId, artifact.path);
      });
      item.append(button);
      list.append(item);
    }
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    taskDetail.textContent = error instanceof Error ? error.message : "无法列出产物";
  }
}

async function downloadArtifact(taskId: string, path: string): Promise<void> {
  try {
    const outcome = await callTool("task.artifact", { task_id: taskId, path });
    if (outcome.isError) {
      taskDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
      return;
    }
    const file = artifactFileFromPayload(outcome.value);
    if (file === null) {
      taskDetail.textContent = "产物内容无法解码";
      return;
    }
    const blob = new Blob([decodeBase64Content(file.content)], { type: file.content_type });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = file.name;
    link.click();
    URL.revokeObjectURL(url);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    taskDetail.textContent = error instanceof Error ? error.message : "无法下载产物";
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
      rememberVerifiedRole(outcome.value);
      loginState.textContent = "";
      applyRoute();
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
    clearVerifiedRole();
    applyRoute();
  })();
});

navBusiness.addEventListener("click", (event) => {
  event.preventDefault();
  goTo("business");
});
navManage.addEventListener("click", (event) => {
  event.preventDefault();
  goTo("manage");
});
manageBack.addEventListener("click", (event) => {
  event.preventDefault();
  goTo("business");
});
refreshTasks.addEventListener("click", () => {
  void loadTaskList();
});

protenixForm.addEventListener("submit", (event) => {
  event.preventDefault();
  void submitJob(jobs.protenix, protenixOutput, chosenFile(protenixForm, protenixDefaultFile));
});

pxdesignForm.addEventListener("submit", (event) => {
  event.preventDefault();
  void submitJob(jobs.pxdesign, pxdesignOutput, chosenFile(pxdesignForm, pxdesignDefaultFile));
});


function showAdminError(target: HTMLElement, value: unknown, fallback: string): void {
  if (typeof value === "string" && value !== "") {
    target.textContent = value;
    return;
  }
  const body = asRecord(value);
  const message = body?.error_msg ?? body?.error_message ?? fallback;
  target.textContent = String(message);
}

async function loadAdminUsers(): Promise<void> {
  if (!canManage()) {
    return;
  }
  try {
    const outcome = await callTool("admin.user.list", {});
    if (outcome.isError) {
      showAdminError(userList, outcome.value, "无法加载用户列表");
      return;
    }
    const body = asRecord(outcome.value);
    const rows = Array.isArray(body?.users) ? body.users : [];
    userList.replaceChildren();
    if (rows.length === 0) {
      const empty = document.createElement("p");
      empty.className = "hint";
      empty.textContent = "暂无用户";
      userList.append(empty);
      return;
    }
    for (const raw of rows) {
      const user = asRecord(raw);
      if (user === null) {
        continue;
      }
      const row = document.createElement("article");
      row.className = "task-row";
      const meta = document.createElement("p");
      meta.className = "task-meta";
      meta.textContent = [
        String(user.user_name ?? ""),
        String(user.role ?? ""),
        typeof user.bind_email === "string" ? user.bind_email : "",
        String(user.user_id ?? ""),
      ].filter((part) => part !== "").join("  ·  ");
      const actions = document.createElement("div");
      actions.className = "task-actions";
      const userId = typeof user.user_id === "string" ? user.user_id : "";
      const currentRole = user.role === "admin" ? "admin" : "user";
      const nextRole = currentRole === "admin" ? "user" : "admin";
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = nextRole === "admin" ? "提升为 admin" : "降为 user";
      button.addEventListener("click", () => {
        void setUserRole(userId, nextRole);
      });
      actions.append(button);
      row.append(meta, actions);
      userList.append(row);
    }
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    userList.textContent = error instanceof Error ? error.message : "无法加载用户列表";
  }
}

async function setUserRole(userId: string, role: string): Promise<void> {
  if (userId === "") {
    return;
  }
  try {
    const outcome = await callTool("admin.user.set_role", { user_id: userId, role });
    if (outcome.isError) {
      showAdminError(userList, outcome.value, "无法更改角色");
      return;
    }
    await loadAdminUsers();
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    userList.textContent = error instanceof Error ? error.message : "无法更改角色";
  }
}

async function loadAdminTasks(): Promise<void> {
  if (!canManage()) {
    return;
  }
  try {
    const outcome = await callTool("admin.task.list", {});
    if (outcome.isError) {
      showAdminError(adminTaskList, outcome.value, "无法加载全站任务");
      return;
    }
    const body = asRecord(outcome.value);
    const rows = Array.isArray(body?.tasks) ? body.tasks : [];
    adminTaskList.replaceChildren();
    if (rows.length === 0) {
      const empty = document.createElement("p");
      empty.className = "hint";
      empty.textContent = "暂无任务";
      adminTaskList.append(empty);
      return;
    }
    for (const raw of rows) {
      const task = asRecord(raw);
      if (task === null) {
        continue;
      }
      const row = document.createElement("article");
      row.className = "task-row";
      const taskId = typeof task.task_id === "string" ? task.task_id : "";
      const meta = document.createElement("p");
      meta.className = "task-meta";
      meta.textContent = [
        String(task.method ?? ""),
        String(task.run_status ?? ""),
        String(task.process_status ?? ""),
        String(task.user_id ?? ""),
        taskId,
      ].filter((part) => part !== "").join("  ·  ");
      const actions = document.createElement("div");
      actions.className = "task-actions";
      const view = document.createElement("button");
      view.type = "button";
      view.textContent = "查看";
      view.addEventListener("click", () => {
        void inspectAdminTask(taskId);
      });
      actions.append(view);
      if (canCancelTask(task) && taskId !== "") {
        const cancel = document.createElement("button");
        cancel.type = "button";
        cancel.textContent = "取消";
        cancel.addEventListener("click", () => {
          void cancelAdminTask(taskId);
        });
        actions.append(cancel);
      }
      if (canListArtifacts(task) && taskId !== "") {
        const artifacts = document.createElement("button");
        artifacts.type = "button";
        artifacts.textContent = "产物";
        artifacts.addEventListener("click", () => {
          void loadAdminArtifacts(taskId, row);
        });
        actions.append(artifacts);
      }
      row.append(meta, actions);
      adminTaskList.append(row);
    }
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    adminTaskList.textContent = error instanceof Error ? error.message : "无法加载全站任务";
  }
}

async function inspectAdminTask(taskId: string): Promise<void> {
  if (taskId === "") {
    return;
  }
  try {
    const outcome = await callTool("admin.task.get", { task_id: taskId });
    adminTaskDetail.textContent = outcome.isError
      ? JSON.stringify(redact(outcome.value), null, 2)
      : formatTask(outcome.value);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    adminTaskDetail.textContent = error instanceof Error ? error.message : "无法读取任务";
  }
}

async function cancelAdminTask(taskId: string): Promise<void> {
  if (!window.confirm("取消该任务？")) {
    return;
  }
  try {
    const outcome = await callTool("admin.task.cancel", { task_id: taskId });
    adminTaskDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
    await loadAdminTasks();
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    adminTaskDetail.textContent = error instanceof Error ? error.message : "无法取消任务";
  }
}

async function loadAdminArtifacts(taskId: string, row: HTMLElement): Promise<void> {
  try {
    const outcome = await callTool("admin.task.artifacts", { task_id: taskId });
    if (outcome.isError) {
      adminTaskDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
      return;
    }
    const artifacts = artifactsFromPayload(outcome.value);
    let list = row.querySelector<HTMLElement>(".artifact-list");
    if (list === null) {
      list = document.createElement("ul");
      list.className = "artifact-list";
      row.append(list);
    }
    list.replaceChildren();
    if (artifacts.length === 0) {
      const empty = document.createElement("li");
      empty.className = "hint";
      empty.textContent = "没有可下载产物";
      list.append(empty);
      return;
    }
    for (const artifact of artifacts) {
      const item = document.createElement("li");
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = `下载 ${artifact.path}`;
      button.addEventListener("click", () => {
        void downloadAdminArtifact(taskId, artifact.path);
      });
      item.append(button);
      list.append(item);
    }
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    adminTaskDetail.textContent = error instanceof Error ? error.message : "无法列出产物";
  }
}

async function downloadAdminArtifact(taskId: string, path: string): Promise<void> {
  try {
    const outcome = await callTool("admin.task.artifact", { task_id: taskId, path });
    if (outcome.isError) {
      adminTaskDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
      return;
    }
    const file = artifactFileFromPayload(outcome.value);
    if (file === null) {
      adminTaskDetail.textContent = "产物内容无法解码";
      return;
    }
    const blob = new Blob([decodeBase64Content(file.content)], { type: file.content_type });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = file.name;
    link.click();
    URL.revokeObjectURL(url);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    adminTaskDetail.textContent = error instanceof Error ? error.message : "无法下载产物";
  }
}

async function loadResourceSets(): Promise<void> {
  if (!canManage()) {
    return;
  }
  try {
    const outcome = await callTool("resource.sets", {});
    if (outcome.isError) {
      showAdminError(resourceDetail, outcome.value, "无法列出资源集合");
      return;
    }
    const body = asRecord(outcome.value);
    const sets = Array.isArray(body?.sets) ? body.sets.filter((item): item is string => typeof item === "string") : [];
    resourceSet.replaceChildren();
    if (sets.length === 0) {
      const option = document.createElement("option");
      option.value = "";
      option.textContent = "当前没有已启用的资源集合";
      resourceSet.append(option);
      return;
    }
    for (const id of sets) {
      const option = document.createElement("option");
      option.value = id;
      option.textContent = id;
      resourceSet.append(option);
    }
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    resourceDetail.textContent = error instanceof Error ? error.message : "无法列出资源集合";
  }
}

async function queryResourceStatus(): Promise<void> {
  const set = resourceSet.value.trim();
  if (set === "") {
    resourceDetail.textContent = "请选择资源集合";
    return;
  }
  try {
    const outcome = await callTool("resource.status", { set });
    resourceDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    resourceDetail.textContent = error instanceof Error ? error.message : "无法查询资源状态";
  }
}

async function prepareResource(): Promise<void> {
  const set = resourceSet.value.trim();
  if (set === "") {
    resourceDetail.textContent = "请选择资源集合";
    return;
  }
  try {
    const status = await callTool("resource.status", { set });
    if (status.isError) {
      resourceDetail.textContent = JSON.stringify(redact(status.value), null, 2);
      return;
    }
    const token = asRecord(status.value)?.confirmation_token;
    if (typeof token !== "string" || token === "") {
      resourceDetail.textContent = "当前状态没有 confirmation_token，无需或无法准备。\n" + JSON.stringify(redact(status.value), null, 2);
      return;
    }
    const outcome = await callTool("resource.prepare", { set, confirmation_token: token });
    resourceDetail.textContent = JSON.stringify(redact(outcome.value), null, 2);
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    resourceDetail.textContent = error instanceof Error ? error.message : "无法准备资源";
  }
}

async function loadAdminSessions(): Promise<void> {
  if (!canManage()) {
    return;
  }
  try {
    const outcome = await callTool("admin.session.list", {});
    if (outcome.isError) {
      showAdminError(sessionList, outcome.value, "无法加载会话");
      return;
    }
    const body = asRecord(outcome.value);
    const rows = Array.isArray(body?.sessions) ? body.sessions : [];
    sessionList.replaceChildren();
    if (rows.length === 0) {
      const empty = document.createElement("p");
      empty.className = "hint";
      empty.textContent = "暂无会话";
      sessionList.append(empty);
      return;
    }
    for (const raw of rows) {
      const session = asRecord(raw);
      if (session === null) {
        continue;
      }
      const row = document.createElement("article");
      row.className = "task-row";
      const sessionId = typeof session.session_id === "string" ? session.session_id : "";
      const meta = document.createElement("p");
      meta.className = "task-meta";
      const revoked = session.revoked_at ? `已吊销 ${String(session.revoked_at)}` : "有效";
      meta.textContent = [String(session.user_id ?? ""), revoked, String(session.expires_at ?? ""), sessionId]
        .filter((part) => part !== "")
        .join("  ·  ");
      const actions = document.createElement("div");
      actions.className = "task-actions";
      if (!session.revoked_at && sessionId !== "") {
        const revoke = document.createElement("button");
        revoke.type = "button";
        revoke.textContent = "吊销";
        revoke.addEventListener("click", () => {
          void revokeSession(sessionId);
        });
        actions.append(revoke);
      }
      row.append(meta, actions);
      sessionList.append(row);
    }
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    sessionList.textContent = error instanceof Error ? error.message : "无法加载会话";
  }
}

async function revokeSession(sessionId: string): Promise<void> {
  if (!window.confirm("吊销该会话？")) {
    return;
  }
  try {
    const outcome = await callTool("admin.session.revoke", { session_id: sessionId });
    if (outcome.isError) {
      showAdminError(sessionList, outcome.value, "无法吊销会话");
      return;
    }
    await loadAdminSessions();
  } catch (error: unknown) {
    if (handleAuthFailure(error)) {
      return;
    }
    sessionList.textContent = error instanceof Error ? error.message : "无法吊销会话";
  }
}

refreshUsers.addEventListener("click", () => {
  void loadAdminUsers();
});
refreshAdminTasks.addEventListener("click", () => {
  void loadAdminTasks();
});
refreshSessions.addEventListener("click", () => {
  void loadAdminSessions();
});
resourceStatus.addEventListener("click", () => {
  void queryResourceStatus();
});
resourcePrepare.addEventListener("click", () => {
  void prepareResource();
});
