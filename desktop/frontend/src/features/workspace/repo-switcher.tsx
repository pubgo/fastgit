import { useEffect, useMemo, useState } from "react";

import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useAppContext } from "../../app/providers/app-context";

export function RepoSwitcher() {
  const { state, addRepo, switchRepo, removeRepo } = useAppContext();
  const [draft, setDraft] = useState("");
  const [selectedPath, setSelectedPath] = useState("");
  const [query, setQuery] = useState("");

  useEffect(() => {
    setSelectedPath(state.repoPath);
  }, [state.repoPath]);

  const canAdd = draft.trim().length > 0;

  const filteredRepos = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return state.repoNamespaces;
    }
    return state.repoNamespaces.filter((repoPath) => repoPath.toLowerCase().includes(keyword));
  }, [query, state.repoNamespaces]);

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

  return (
    <section className="repo-settings" aria-label="Project settings">
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

      <p className="repo-settings__status">{state.repoStatus}</p>
    </section>
  );
}
