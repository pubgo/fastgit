import "./style.css";
import * as FastgitService from "../bindings/fastgitdesktop/fastgitservice";
import type { ActionField, CommandResult, DesktopModule, ModuleAction } from "../bindings/fastgitdesktop/models";

const app = document.querySelector<HTMLDivElement>("#app");
if (!app) throw new Error("#app not found");

app.innerHTML = `
  <main class="layout">
    <section class="hero">
      <h1>fastgit desktop console</h1>
      <p>仓库 / 分支 / worktree / issue / PR / tag 的统一管理台</p>
    </section>

    <section class="card repo-card">
      <div class="row">
        <label for="repo-path">Repo Path</label>
      </div>
      <div class="row gap">
        <input id="repo-path" class="input" placeholder="/path/to/repo" />
        <button id="set-repo" class="btn">切换仓库</button>
      </div>
      <div id="repo-status" class="hint"></div>
    </section>

    <section class="card raw-card">
      <div class="row">
        <label for="raw-cmd">原始 fastgit 命令</label>
      </div>
      <div class="row gap">
        <input id="raw-cmd" class="input" value="ggc status short" />
        <button id="run-raw" class="btn primary">执行</button>
      </div>
    </section>

    <section class="card">
      <div class="module-head">
        <h2>功能模块</h2>
        <span id="busy"></span>
      </div>
      <div id="modules" class="modules"></div>
    </section>

    <section class="card output-card">
      <div class="module-head">
        <h2>输出</h2>
        <span id="meta" class="hint"></span>
      </div>
      <pre id="output" class="output">等待执行...</pre>
    </section>
  </main>
`;

const repoPath = document.querySelector<HTMLInputElement>("#repo-path")!;
const setRepoBtn = document.querySelector<HTMLButtonElement>("#set-repo")!;
const repoStatus = document.querySelector<HTMLDivElement>("#repo-status")!;
const rawCmd = document.querySelector<HTMLInputElement>("#raw-cmd")!;
const runRawBtn = document.querySelector<HTMLButtonElement>("#run-raw")!;
const modulesWrap = document.querySelector<HTMLDivElement>("#modules")!;
const output = document.querySelector<HTMLElement>("#output")!;
const meta = document.querySelector<HTMLElement>("#meta")!;
const busy = document.querySelector<HTMLElement>("#busy")!;

let modules: DesktopModule[] = [];

function setBusy(value: boolean) {
  busy.textContent = value ? "运行中..." : "";
}

function renderResult(result: CommandResult, title?: string) {
  output.textContent = result.output || "(no output)";
  const header = title ? `${title} | ` : "";
  meta.textContent = `${header}${result.command} | exit=${result.exitCode}`;
}

function createFieldInput(moduleID: string, actionID: string, field: ActionField): HTMLInputElement {
  const input = document.createElement("input");
  input.className = "input";
  input.placeholder = field.placeholder || field.label;
  input.dataset.moduleId = moduleID;
  input.dataset.actionId = actionID;
  input.dataset.fieldKey = field.key;
  if (field.default) input.value = field.default;
  return input;
}

function collectActionValues(moduleID: string, actionID: string): Record<string, string> {
  const values: Record<string, string> = {};
  const selector = `input[data-module-id="${moduleID}"][data-action-id="${actionID}"]`;
  const inputs = modulesWrap.querySelectorAll<HTMLInputElement>(selector);
  inputs.forEach((input) => {
    const key = input.dataset.fieldKey;
    if (!key) return;
    values[key] = input.value.trim();
  });
  return values;
}

async function runAction(module: DesktopModule, action: ModuleAction) {
  setBusy(true);
  try {
    const values = collectActionValues(module.id, action.id);
    const result = await FastgitService.RunAction({
      moduleID: module.id,
      actionID: action.id,
      values,
    });
    renderResult(result, `${module.title} / ${action.title}`);
  } catch (err) {
    output.textContent = `执行失败: ${String(err)}`;
    meta.textContent = `${module.title} / ${action.title}`;
  } finally {
    setBusy(false);
  }
}

function renderModules() {
  modulesWrap.innerHTML = "";

  for (const module of modules) {
    const section = document.createElement("section");
    section.className = "module";

    const header = document.createElement("div");
    header.className = "module-title";
    header.innerHTML = `<h3>${module.title}</h3><p>${module.description}</p>`;
    section.appendChild(header);

    for (const action of module.actions || []) {
      const row = document.createElement("div");
      row.className = "action-row";

      const main = document.createElement("div");
      main.className = "action-main";
      main.innerHTML = `<strong>${action.title}</strong><span>${action.description}</span>`;

      const controls = document.createElement("div");
      controls.className = "action-controls";

      for (const field of action.fields || []) {
        controls.appendChild(createFieldInput(module.id, action.id, field));
      }

      const btn = document.createElement("button");
      btn.className = "btn small";
      btn.textContent = "执行";
      btn.addEventListener("click", () => void runAction(module, action));
      controls.appendChild(btn);

      row.appendChild(main);
      row.appendChild(controls);
      section.appendChild(row);
    }

    modulesWrap.appendChild(section);
  }
}

async function boot() {
  try {
    const root = await FastgitService.GetRepoRoot();
    repoPath.value = root;
    repoStatus.textContent = `当前仓库: ${root}`;
  } catch (err) {
    repoStatus.textContent = `读取仓库失败: ${String(err)}`;
  }

  try {
    modules = (await FastgitService.GetModules()) || [];
    renderModules();
  } catch (err) {
    modulesWrap.textContent = `加载模块失败: ${String(err)}`;
  }
}

setRepoBtn.addEventListener("click", async () => {
  const path = repoPath.value.trim();
  if (!path) return;
  setBusy(true);
  try {
    await FastgitService.SetRepoRoot(path);
    repoStatus.textContent = `当前仓库: ${path}`;
  } catch (err) {
    repoStatus.textContent = `切换失败: ${String(err)}`;
  } finally {
    setBusy(false);
  }
});

runRawBtn.addEventListener("click", async () => {
  const cmd = rawCmd.value.trim();
  if (!cmd) return;
  setBusy(true);
  try {
    const result = await FastgitService.RunFastgit(cmd);
    renderResult(result, "Raw fastgit");
  } catch (err) {
    output.textContent = `执行失败: ${String(err)}`;
    meta.textContent = "Raw fastgit";
  } finally {
    setBusy(false);
  }
});

rawCmd.addEventListener("keydown", (event) => {
  if (event.key !== "Enter") return;
  event.preventDefault();
  runRawBtn.click();
});

void boot();
