import { useEffect } from "react";

import { useAppContext } from "../../app/providers/app-context";
import { Button } from "../../components/ui/button";

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? "workspace";
}

export function ProjectContextBar() {
  const { state, prefetchAction } = useAppContext();

  useEffect(() => {
    if (!state.repoPath) {
      return;
    }

    if (state.catalog.branches.length === 0) {
      void prefetchAction("branch", "branch_list");
    }
    if (state.catalog.remotes.length === 0) {
      void prefetchAction("remote", "remote_list");
    }
    if (state.catalog.repoStatus.length === 0) {
      void prefetchAction("repo", "repo_status");
    }
  }, [prefetchAction, state.catalog.branches.length, state.catalog.remotes.length, state.catalog.repoStatus.length, state.repoPath]);

  if (!state.repoPath) {
    return null;
  }

  const currentBranch = state.catalog.branches.find((item) => item.active) ?? null;
  const trackedRemote = currentBranch?.fields?.remote || "";
  const projectDefaultRemote = state.projectSettings[state.repoPath]?.defaultRemote ?? "";
  const defaultRemote = projectDefaultRemote || trackedRemote || state.catalog.remotes.find((item) => item.active)?.value || state.catalog.remotes[0]?.value || "-";
  const upstream = currentBranch?.fields?.upstream || "";
  const syncLabel = currentBranch?.fields?.sync_label || "";
  const dirtyCount = state.catalog.repoStatus.length;
  const dirtyLabel = dirtyCount > 0 ? `${dirtyCount} 个改动` : "工作区干净";
  const currentBaseBranch = state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "";
  const githubLabel = state.githubAuthStatus?.configured
    ? `GitHub ${state.githubAuthStatus.source === "session" ? "会话 Token" : "环境变量"}`
    : "GitHub 未连接";

  const refreshContext = () => {
    void Promise.all([
      prefetchAction("repo", "repo_status"),
      prefetchAction("branch", "branch_list"),
      prefetchAction("remote", "remote_list"),
    ]);
  };

  return (
    <section className="project-contextbar">
      <div className="project-contextbar__identity">
        <div className="project-contextbar__title">
          <strong>{basename(state.repoPath)}</strong>
          <span>{state.repoPath}</span>
        </div>
        <div className="project-contextbar__chips">
          <span className="project-contextbar__chip">{currentBranch ? `分支 ${currentBranch.primary}` : "分支 -"}</span>
          <span className="project-contextbar__chip">{defaultRemote ? `Remote ${defaultRemote}` : "Remote -"}</span>
          <span className="project-contextbar__chip">{currentBaseBranch ? `Base ${currentBaseBranch}` : "Base 未设置"}</span>
          <span className="project-contextbar__chip">{dirtyLabel}</span>
          <span className="project-contextbar__chip">{githubLabel}</span>
        </div>
      </div>
      <div className="project-contextbar__meta">
        {upstream ? <span>{upstream}</span> : <span>未配置 upstream</span>}
        {syncLabel ? <span>{syncLabel}</span> : null}
      </div>
      <div className="project-contextbar__actions">
        <Button variant="ghost" onClick={refreshContext} disabled={state.busy}>
          刷新上下文
        </Button>
      </div>
    </section>
  );
}
