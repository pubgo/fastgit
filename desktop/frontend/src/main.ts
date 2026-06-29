import "./style.css";
import { FastgitService } from "../bindings/fastgitdesktop";

const app = document.querySelector<HTMLDivElement>("#app")!;

app.innerHTML = `
  <main class="layout">
    <header class="header">
      <h1>fastgit desktop</h1>
      <p>在桌面里执行 fastgit 命令并查看输出</p>
    </header>

    <section class="panel">
      <label for="repo">Repo Path</label>
      <div class="row">
        <input id="repo" class="input" placeholder="/path/to/repo" />
        <button id="save-repo" class="btn">切换仓库</button>
      </div>
      <small id="repo-hint"></small>
    </section>

    <section class="panel">
      <label for="cmd">fastgit command</label>
      <div class="row">
        <input id="cmd" class="input" value="ggc status short" />
        <button id="run" class="btn primary">执行</button>
      </div>
      <div class="quick-actions">
        <button class="chip" data-cmd="ggc status short">状态</button>
        <button class="chip" data-cmd="pull">pull</button>
        <button class="chip" data-cmd="push">push</button>
        <button class="chip" data-cmd="worktree">worktree</button>
      </div>
    </section>

    <section class="panel output-panel">
      <div class="output-head">
        <span>Output</span>
        <span id="meta"></span>
      </div>
      <pre id="output" class="output">等待执行命令...</pre>
    </section>
  </main>
`;

const repoInput = document.querySelector<HTMLInputElement>("#repo")!;
const repoHint = document.querySelector<HTMLElement>("#repo-hint")!;
const cmdInput = document.querySelector<HTMLInputElement>("#cmd")!;
const runBtn = document.querySelector<HTMLButtonElement>("#run")!;
const saveRepoBtn = document.querySelector<HTMLButtonElement>("#save-repo")!;
const output = document.querySelector<HTMLElement>("#output")!;
const meta = document.querySelector<HTMLElement>("#meta")!;

async function boot() {
  try {
    const root = await FastgitService.GetRepoRoot();
    repoInput.value = root;
    repoHint.textContent = `当前仓库: ${root}`;
  } catch (err) {
    repoHint.textContent = `读取仓库失败: ${String(err)}`;
  }
}

async function run() {
  const command = cmdInput.value.trim();
  if (!command) return;

  runBtn.disabled = true;
  output.textContent = "running...";
  meta.textContent = "";

  try {
    const res = await FastgitService.RunFastgit(command);
    output.textContent = res.output || "(no output)";
    meta.textContent = `${res.command} | exit=${res.exitCode}`;
  } catch (err) {
    output.textContent = `执行失败: ${String(err)}`;
  } finally {
    runBtn.disabled = false;
  }
}

saveRepoBtn.addEventListener("click", async () => {
  const path = repoInput.value.trim();
  if (!path) return;
  try {
    await FastgitService.SetRepoRoot(path);
    repoHint.textContent = `当前仓库: ${path}`;
  } catch (err) {
    repoHint.textContent = `切换失败: ${String(err)}`;
  }
});

runBtn.addEventListener("click", () => void run());

cmdInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    void run();
  }
});

document.querySelectorAll<HTMLButtonElement>(".chip").forEach((btn) => {
  btn.addEventListener("click", () => {
    cmdInput.value = btn.dataset.cmd || "";
    void run();
  });
});

void boot();
