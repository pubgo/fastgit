import { useEffect, useMemo, useState } from "react";

import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useAppContext } from "../../app/providers/app-context";

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

export function RepoSwitcher() {
  const { state, addRepo, switchRepo, removeRepo, setGitHubToken, refreshGitHubAuthStatus, updateProjectSettings, prefetchAction } = useAppContext();
  const [draft, setDraft] = useState("");
  const [selectedPath, setSelectedPath] = useState("");
  const [query, setQuery] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [savingToken, setSavingToken] = useState(false);
  const [baseBranchDraft, setBaseBranchDraft] = useState("");

  useEffect(() => {
    setSelectedPath(state.repoPath);
  }, [state.repoPath]);

  useEffect(() => {
    setBaseBranchDraft(state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "");
  }, [state.projectSettings, state.repoPath]);

  useEffect(() => {
    if (!state.repoPath || state.catalog.branches.length > 0) {
      return;
    }
    void prefetchAction("branch", "branch_list");
  }, [prefetchAction, state.catalog.branches.length, state.repoPath]);

  const canAdd = draft.trim().length > 0;

  const filteredRepos = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return state.repoNamespaces;
    }
    return state.repoNamespaces.filter((repoPath) => repoPath.toLowerCase().includes(keyword));
  }, [query, state.repoNamespaces]);
  const branchOptions = state.catalog.branches;
  const currentProjectSettings = state.repoPath ? state.projectSettings[state.repoPath] : undefined;

  const onAddOnly = async () => {
    const next = draft.trim();
    if (!next) {
      return;
    }
    await addRepo(next);
    setDraft("");
  };

  const onAddAndSwitch = async () => {
    const next = draft.trim();
    if (!next) {
      return;
    }
    await switchRepo(next);
    setDraft("");
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

  return (
    <section className="repo-settings" aria-label="Project settings">
      <div className="settings-section">
        <header className="repo-settings__header">
          <h3>Settings · Projects</h3>
          <p>{state.repoNamespaces.length} 个项目</p>
        </header>

        <div className="repo-settings__current">
          <span>当前项目</span>
          <strong>{state.repoPath || "未选择"}</strong>
        </div>

        <div className="workspace-panel__row workspace-panel__row--add">
          <Input
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder="/path/to/project (git repository)"
          />
          <Button variant="ghost" onClick={() => void onAddOnly()} disabled={!canAdd}>
            添加
          </Button>
          <Button variant="primary" onClick={() => void onAddAndSwitch()} disabled={!canAdd}>
            添加并切换
          </Button>
        </div>

        <div className="workspace-panel__row">
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="筛选项目..." />
          <span className="repo-settings__count">
            {filteredRepos.length} / {state.repoNamespaces.length}
          </span>
        </div>

        <div className="repo-list" role="list">
          {filteredRepos.map((repoPath) => {
            const isCurrent = repoPath === state.repoPath;
            const isSelected = repoPath === selectedPath;
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
                  <strong>{repoPath.split(/[\\/]/).filter(Boolean).pop() || repoPath}</strong>
                  <span>{repoPath}</span>
                </div>
                <div className="repo-item__actions">
                  <span className={isCurrent ? "repo-badge repo-badge--active" : "repo-badge"}>
                    {isCurrent ? "当前" : "可切换"}
                  </span>
                  <Button
                    variant="ghost"
                    onClick={(event) => {
                      event.stopPropagation();
                      void switchRepo(repoPath);
                    }}
                    disabled={isCurrent}
                  >
                    切换
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={(event) => {
                      event.stopPropagation();
                      void removeRepo(repoPath);
                    }}
                    disabled={isCurrent}
                  >
                    移除
                  </Button>
                </div>
              </article>
            );
          })}
          {filteredRepos.length === 0 && <div className="repo-list__empty">没有匹配的项目</div>}
        </div>
      </div>

      <div className="settings-section">
        <header className="repo-settings__header">
          <h3>Settings · GitHub</h3>
          <p>{state.githubAuthStatus?.configured ? "已连接" : "未连接"}</p>
        </header>

        <div className="repo-settings__current">
          <span>认证来源</span>
          <strong>{sourceLabel(state.githubAuthStatus?.source)}</strong>
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
            应用
          </Button>
        </div>

        <div className="workspace-panel__row">
          <Button variant="ghost" onClick={() => void refreshGitHubAuthStatus()} disabled={savingToken}>
            刷新状态
          </Button>
          <Button variant="ghost" onClick={() => void onClearToken()} disabled={savingToken}>
            清除会话 Token
          </Button>
          <span className="repo-settings__count">不会写入本地存储</span>
        </div>

        <p className="repo-settings__status">{state.githubAuthStatus?.message ?? "GitHub 状态未知"}</p>
      </div>

      <div className="settings-section">
        <header className="repo-settings__header">
          <h3>Settings · Project Defaults</h3>
          <p>{state.repoPath ? "当前项目生效" : "未选择项目"}</p>
        </header>

        <div className="repo-settings__current">
          <span>默认 Base Branch</span>
          <strong>{currentProjectSettings?.defaultBaseBranch || "未设置，回退到动作默认值"}</strong>
        </div>

        <div className="workspace-panel__row workspace-panel__row--add">
          {branchOptions.length > 0 ? (
            <select
              className="ui-input ui-select"
              value={baseBranchDraft}
              onChange={(event) => setBaseBranchDraft(event.target.value)}
              disabled={!state.repoPath}
            >
              <option value="">未设置</option>
              {branchOptions.map((branch) => (
                <option key={branch.id} value={branch.value ?? branch.primary}>
                  {branch.value ?? branch.primary}
                </option>
              ))}
            </select>
          ) : (
            <Input
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
            保存默认值
          </Button>
        </div>

        <p className="repo-settings__status">用于 `PR 创建` 和 `Worktree 创建` 的 base 字段默认值。</p>
      </div>

      <p className="repo-settings__status">{state.repoStatus}</p>
    </section>
  );
}
