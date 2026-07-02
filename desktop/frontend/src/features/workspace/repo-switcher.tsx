import { useEffect, useMemo, useState } from "react";
import { Select } from "antd";

import { useAppContext } from "../../app/providers/app-context";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";

type SettingsSection = "projects" | "auth" | "defaults";

function sourceLabel(source: string | undefined): string {
  switch (source) {
    case "session":
      return "当前会话";
    case "env":
      return "环境变量";
    default:
      return "未配置";
  }
}

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? path;
}

function pathSegments(path: string): string[] {
  return path.split(/[\\/]/).filter(Boolean);
}

function buildProjectLabels(paths: string[]): Map<string, string> {
  const labels = new Map<string, string>();

  for (const path of paths) {
    const segments = pathSegments(path);
    if (segments.length === 0) {
      labels.set(path, path);
      continue;
    }

    let label = segments[segments.length - 1];
    for (let depth = 1; depth <= segments.length; depth += 1) {
      const candidate = segments.slice(-depth).join("/");
      const duplicated = paths.some((otherPath) => {
        if (otherPath === path) {
          return false;
        }
        const otherSegments = pathSegments(otherPath);
        return otherSegments.slice(-depth).join("/") === candidate;
      });
      label = candidate;
      if (!duplicated) {
        break;
      }
    }

    labels.set(path, label);
  }

  return labels;
}

const sections: Array<{ id: SettingsSection; title: string; summary: string }> = [
  { id: "projects", title: "Projects", summary: "仓库命名空间与切换" },
  { id: "auth", title: "Auth", summary: "GitHub 认证与会话 Token" },
  { id: "defaults", title: "Defaults", summary: "当前项目默认分支与 remote" },
];

export function RepoSwitcher() {
  const { state, addRepo, switchRepo, removeRepo, setGitHubToken, refreshGitHubAuthStatus, updateProjectSettings, prefetchAction } = useAppContext();
  const [activeSection, setActiveSection] = useState<SettingsSection>("projects");
  const [draft, setDraft] = useState("");
  const [selectedPath, setSelectedPath] = useState("");
  const [query, setQuery] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [savingToken, setSavingToken] = useState(false);
  const [baseBranchDraft, setBaseBranchDraft] = useState("");
  const [defaultRemoteDraft, setDefaultRemoteDraft] = useState("");

  useEffect(() => {
    setSelectedPath((current) => current || state.repoPath);
  }, [state.repoPath]);

  useEffect(() => {
    setBaseBranchDraft(state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "");
  }, [state.projectSettings, state.repoPath]);

  useEffect(() => {
    setDefaultRemoteDraft(state.projectSettings[state.repoPath]?.defaultRemote ?? "");
  }, [state.projectSettings, state.repoPath]);

  useEffect(() => {
    if (!state.repoPath || state.catalog.branches.length > 0) {
      return;
    }
    void prefetchAction("branch", "branch_list");
  }, [prefetchAction, state.catalog.branches.length, state.repoPath]);

  useEffect(() => {
    if (!state.repoPath || state.catalog.remotes.length > 0) {
      return;
    }
    void prefetchAction("remote", "remote_list");
  }, [prefetchAction, state.catalog.remotes.length, state.repoPath]);

  const canAdd = draft.trim().length > 0;
  const branchOptions = state.catalog.branches;
  const remoteOptions = state.catalog.remotes;
  const projectLabels = useMemo(() => buildProjectLabels(state.repoNamespaces), [state.repoNamespaces]);

  const filteredRepos = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return state.repoNamespaces;
    }
    return state.repoNamespaces.filter((repoPath) => {
      const label = projectLabels.get(repoPath) ?? basename(repoPath);
      return repoPath.toLowerCase().includes(keyword) || label.toLowerCase().includes(keyword);
    });
  }, [projectLabels, query, state.repoNamespaces]);

  const focusedProjectPath = selectedPath || state.repoPath;
  const focusedProjectSettings = focusedProjectPath ? state.projectSettings[focusedProjectPath] : undefined;
  const focusedProjectLabel = focusedProjectPath ? projectLabels.get(focusedProjectPath) ?? basename(focusedProjectPath) : "未选择";
  const currentProjectLabel = state.repoPath ? projectLabels.get(state.repoPath) ?? basename(state.repoPath) : "未选择";
  const isFocusedProjectCurrent = Boolean(focusedProjectPath && focusedProjectPath === state.repoPath);

  const onAddOnly = async () => {
    const next = draft.trim();
    if (!next) {
      return;
    }
    await addRepo(next);
    setDraft("");
    setSelectedPath(next);
  };

  const onAddAndSwitch = async () => {
    const next = draft.trim();
    if (!next) {
      return;
    }
    await switchRepo(next);
    setDraft("");
    setSelectedPath(next);
  };

  const onApplyToken = async () => {
    setSavingToken(true);
    try {
      await setGitHubToken(tokenDraft);
      setTokenDraft("");
      setShowToken(false);
    } finally {
      setSavingToken(false);
    }
  };

  const onClearToken = async () => {
    setSavingToken(true);
    try {
      await setGitHubToken("");
      setTokenDraft("");
      setShowToken(false);
      await refreshGitHubAuthStatus();
    } finally {
      setSavingToken(false);
    }
  };

  const onSaveProjectDefaults = () => {
    updateProjectSettings({ defaultBaseBranch: baseBranchDraft.trim() });
  };

  const onSaveProjectRemote = () => {
    updateProjectSettings({ defaultRemote: defaultRemoteDraft.trim() });
  };

  return (
    <section className="repo-settings" aria-label="Project settings">
      <aside className="repo-settings__nav">
        <div className="repo-settings__overview">
          <span>Current Project</span>
          <strong>{currentProjectLabel}</strong>
          <small>{state.repoPath || "未选择项目"}</small>
        </div>
        <div className="repo-settings__nav-list" role="tablist" aria-label="settings sections">
          {sections.map((section) => (
            <button
              key={section.id}
              type="button"
              className={activeSection === section.id ? "repo-settings__nav-item repo-settings__nav-item--active" : "repo-settings__nav-item"}
              onClick={() => setActiveSection(section.id)}
            >
              <strong>{section.title}</strong>
              <span>{section.summary}</span>
            </button>
          ))}
        </div>
      </aside>

      <div className="repo-settings__content">
        {activeSection === "projects" ? (
          <div className="settings-section settings-section--panel">
            <header className="repo-settings__header">
              <h3>Projects</h3>
              <p>{state.repoNamespaces.length} 个项目</p>
            </header>

            <div className="repo-settings__summary-grid">
              <div className="repo-settings__current">
                <span>当前项目</span>
                <strong>{currentProjectLabel}</strong>
                <small>{state.repoPath || "未选择项目"}</small>
              </div>
              <div className="repo-settings__current">
                <span>聚焦项目</span>
                <strong>{focusedProjectLabel}</strong>
                <small>{focusedProjectPath || "请从列表选择项目"}</small>
              </div>
            </div>

            <div className="workspace-panel__row workspace-panel__row--add">
              <Input
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                placeholder="/path/to/project (git repository)"
              />
              <Button variant="ghost" onClick={() => void onAddOnly()} disabled={!canAdd}>
                仅添加
              </Button>
              <Button variant="primary" onClick={() => void onAddAndSwitch()} disabled={!canAdd}>
                添加并切换
              </Button>
            </div>

            <div className="workspace-panel__row">
              <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索项目路径或别名..." />
              <span className="repo-settings__count">
                {filteredRepos.length} / {state.repoNamespaces.length}
              </span>
            </div>

            <div className="repo-settings__focusbar">
              <div className="repo-settings__focusmeta">
                <strong>{focusedProjectLabel}</strong>
                <span>{focusedProjectPath || "未选择项目"}</span>
              </div>
              <div className="repo-settings__focusactions">
                <span className={isFocusedProjectCurrent ? "repo-badge repo-badge--active" : "repo-badge"}>
                  {isFocusedProjectCurrent ? "当前项目" : "可切换"}
                </span>
                <Button variant="ghost" onClick={() => void switchRepo(focusedProjectPath)} disabled={!focusedProjectPath || isFocusedProjectCurrent}>
                  切换到此项目
                </Button>
                <Button variant="ghost" onClick={() => void removeRepo(focusedProjectPath)} disabled={!focusedProjectPath || isFocusedProjectCurrent}>
                  移除此项目
                </Button>
              </div>
            </div>

            <div className="repo-list" role="list">
              {filteredRepos.map((repoPath) => {
                const isCurrent = repoPath === state.repoPath;
                const isSelected = repoPath === focusedProjectPath;
                const baseBranch = state.projectSettings[repoPath]?.defaultBaseBranch ?? "";
                const defaultRemote = state.projectSettings[repoPath]?.defaultRemote ?? "";
                return (
                  <article
                    key={repoPath}
                    className={[
                      "repo-item",
                      isCurrent ? "repo-item--current" : "",
                      isSelected ? "repo-item--selected" : "",
                    ]
                      .filter(Boolean)
                      .join(" ")}
                    onClick={() => setSelectedPath(repoPath)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        setSelectedPath(repoPath);
                      }
                    }}
                  >
                    <div className="repo-item__meta">
                      <strong>{projectLabels.get(repoPath) ?? basename(repoPath)}</strong>
                      <span>{repoPath}</span>
                      <div className="repo-item__tags">
                        <span className="repo-badge">{baseBranch ? `base ${baseBranch}` : "base 未设置"}</span>
                        <span className="repo-badge">{defaultRemote ? `remote ${defaultRemote}` : "remote 未设置"}</span>
                      </div>
                    </div>
                    <div className="repo-item__actions">
                      <span className={isCurrent ? "repo-badge repo-badge--active" : "repo-badge"}>
                        {isCurrent ? "当前" : "待切换"}
                      </span>
                    </div>
                  </article>
                );
              })}
              {filteredRepos.length === 0 ? <div className="repo-list__empty">没有匹配的项目</div> : null}
            </div>
          </div>
        ) : null}

        {activeSection === "auth" ? (
          <div className="settings-section settings-section--panel">
            <header className="repo-settings__header">
              <h3>GitHub Auth</h3>
              <p>{state.githubAuthStatus?.configured ? "已连接" : "未连接"}</p>
            </header>

            <div className="repo-settings__summary-grid repo-settings__summary-grid--wide">
              <div className="repo-settings__current">
                <span>认证来源</span>
                <strong>{sourceLabel(state.githubAuthStatus?.source)}</strong>
                <small>{state.githubAuthStatus?.message ?? "GitHub 状态未知"}</small>
              </div>
              <div className="repo-settings__current">
                <span>作用域</span>
                <strong>当前会话</strong>
                <small>Token 只在桌面客户端当前会话中生效，不写入本地存储。</small>
              </div>
            </div>

            <div className="workspace-panel__row workspace-panel__row--add">
              <Input
                type={showToken ? "text" : "password"}
                value={tokenDraft}
                onChange={(event) => setTokenDraft(event.target.value)}
                placeholder="ghp_ / gho_... 仅当前会话生效"
                autoComplete="off"
                spellCheck={false}
              />
              <Button variant="ghost" onClick={() => setShowToken((value) => !value)}>
                {showToken ? "隐藏" : "显示"}
              </Button>
              <Button variant="primary" onClick={() => void onApplyToken()} disabled={savingToken || tokenDraft.trim().length === 0}>
                应用 Token
              </Button>
            </div>

            <div className="repo-settings__focusbar">
              <div className="repo-settings__focusmeta">
                <strong>认证控制</strong>
                <span>建议优先使用会话 Token，避免把敏感信息固化在工作区配置里。</span>
              </div>
              <div className="repo-settings__focusactions">
                <Button variant="ghost" onClick={() => void refreshGitHubAuthStatus()} disabled={savingToken}>
                  刷新状态
                </Button>
                <Button variant="ghost" onClick={() => void onClearToken()} disabled={savingToken}>
                  清除会话 Token
                </Button>
              </div>
            </div>
          </div>
        ) : null}

        {activeSection === "defaults" ? (
          <div className="settings-section settings-section--panel">
            <header className="repo-settings__header">
              <h3>Project Defaults</h3>
              <p>{state.repoPath ? "只对当前项目生效" : "未选择项目"}</p>
            </header>

            <div className="repo-settings__summary-grid repo-settings__summary-grid--wide">
              <div className="repo-settings__current">
                <span>当前项目</span>
                <strong>{currentProjectLabel}</strong>
                <small>{state.repoPath || "未选择项目"}</small>
              </div>
              <div className="repo-settings__current">
                <span>聚焦项目</span>
                <strong>{focusedProjectLabel}</strong>
                <small>
                  {isFocusedProjectCurrent
                    ? "当前正在编辑该项目默认值"
                    : "默认值只能编辑当前项目；如需修改聚焦项目，请先切换过去。"}
                </small>
              </div>
            </div>

            {!isFocusedProjectCurrent && focusedProjectPath ? (
              <div className="repo-settings__notice">
                <strong>当前是跨项目查看模式</strong>
                <span>你正在查看 `{focusedProjectLabel}`，但默认值编辑只会写入当前项目。先切换项目再保存更安全。</span>
                <Button variant="ghost" onClick={() => void switchRepo(focusedProjectPath)}>
                  切换到该项目后编辑
                </Button>
              </div>
            ) : null}

            <div className="repo-settings__current">
              <span>默认 Base Branch</span>
              <strong>{focusedProjectSettings?.defaultBaseBranch || "未设置，回退到动作默认值"}</strong>
            </div>

            <div className="workspace-panel__row workspace-panel__row--add">
              {branchOptions.length > 0 ? (
                <Select
                  className="workspace-panel__control"
                  value={baseBranchDraft || undefined}
                  options={branchOptions.map((branch) => ({
                    label: branch.value ?? branch.primary,
                    value: branch.value ?? branch.primary,
                  }))}
                  placeholder="未设置"
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  onChange={(next) => setBaseBranchDraft(String(next ?? ""))}
                  disabled={!state.repoPath}
                />
              ) : (
                <Input
                  className="workspace-panel__control"
                  value={baseBranchDraft}
                  onChange={(event) => setBaseBranchDraft(event.target.value)}
                  placeholder="main"
                  disabled={!state.repoPath}
                />
              )}
              <Button variant="ghost" onClick={() => void prefetchAction("branch", "branch_list")} disabled={!state.repoPath}>
                刷新分支
              </Button>
              <Button variant="primary" onClick={onSaveProjectDefaults} disabled={!state.repoPath}>
                保存 Base 默认值
              </Button>
            </div>

            <p className="repo-settings__status">用于 `PR 创建` 和 `Worktree 创建` 的 base 字段默认值。</p>

            <div className="repo-settings__current">
              <span>默认 Remote</span>
              <strong>{focusedProjectSettings?.defaultRemote || "未设置，回退到 tracking / origin"}</strong>
            </div>

            <div className="workspace-panel__row workspace-panel__row--add">
              {remoteOptions.length > 0 ? (
                <Select
                  className="workspace-panel__control"
                  value={defaultRemoteDraft || undefined}
                  options={remoteOptions.map((remote) => ({
                    label: remote.value ?? remote.primary,
                    value: remote.value ?? remote.primary,
                  }))}
                  placeholder="未设置"
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  onChange={(next) => setDefaultRemoteDraft(String(next ?? ""))}
                  disabled={!state.repoPath}
                />
              ) : (
                <Input
                  className="workspace-panel__control"
                  value={defaultRemoteDraft}
                  onChange={(event) => setDefaultRemoteDraft(event.target.value)}
                  placeholder="origin"
                  disabled={!state.repoPath}
                />
              )}
              <Button variant="ghost" onClick={() => void prefetchAction("remote", "remote_list")} disabled={!state.repoPath}>
                刷新 Remote
              </Button>
              <Button variant="primary" onClick={onSaveProjectRemote} disabled={!state.repoPath}>
                保存 Remote 默认值
              </Button>
            </div>

            <p className="repo-settings__status">用于 `仓库拉取/推送`、`分支对齐`、`tag 推送/对齐` 的默认 remote。</p>
          </div>
        ) : null}

        <p className="repo-settings__status">{state.repoStatus}</p>
      </div>
    </section>
  );
}
