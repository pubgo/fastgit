import { useEffect, useRef, useState } from "react";
import { Tabs } from "antd";

import { useAppContext } from "../../app/providers/app-context";
import type { ModuleAction } from "../../app/types";
import { Button } from "../../components/ui/button";
import { ActionDialog } from "../actions/action-dialog";
import { buildActionValues } from "../actions/action-fields";
import { actionVerb, classifyAction, isLandingAction, pickDefaultAction } from "../actions/action-meta";

const actionGroupOrder = ["列表", "新增", "编辑", "切换", "执行", "关闭", "删除", "危险操作", "操作"];
const EMPTY_ACTIONS: ModuleAction[] = [];

function sortActionGroups(groups: Map<string, ModuleAction[]>): Array<[string, ModuleAction[]]> {
  return Array.from(groups.entries()).sort((left, right) => {
    const leftIndex = actionGroupOrder.indexOf(left[0]);
    const rightIndex = actionGroupOrder.indexOf(right[0]);
    const a = leftIndex === -1 ? actionGroupOrder.length : leftIndex;
    const b = rightIndex === -1 ? actionGroupOrder.length : rightIndex;
    return a - b || left[0].localeCompare(right[0]);
  });
}

export function ModuleActions() {
  const { selectedModule, runAction, runActionAndReload, prefetchAction, state } = useAppContext();
  const actions = selectedModule?.actions ?? EMPTY_ACTIONS;
  const [activeActionId, setActiveActionId] = useState<string | null>(null);
  const [activeGroup, setActiveGroup] = useState<string>("");
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [draftValues, setDraftValues] = useState<Record<string, string>>({});
  const lastAutoRunKeyRef = useRef<string>("");

  const activeAction = actions.find((action) => action.id === activeActionId) ?? pickDefaultAction(actions);
  const landingAction = pickDefaultAction(actions);
  const groupedActions = sortActionGroups(
    actions.reduce((groups, action) => {
      const key = classifyAction(action.id);
      const bucket = groups.get(key) ?? [];
      bucket.push(action);
      groups.set(key, bucket);
      return groups;
    }, new Map<string, ModuleAction[]>())
  );
  const defaultBaseBranch = state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "";
  const projectDefaultRemote = state.projectSettings[state.repoPath]?.defaultRemote ?? "";
  const defaultRemote =
    projectDefaultRemote || state.catalog.remotes.find((item) => item.active)?.value || state.catalog.remotes[0]?.value || "origin";

  useEffect(() => {
    const nextAction = pickDefaultAction(actions);
    setActiveActionId(nextAction?.id ?? null);
    setPanelCollapsed(false);
    setDialogOpen(false);
    setDraftValues(buildActionValues(nextAction, {}, { base: defaultBaseBranch, remote: defaultRemote }));
    lastAutoRunKeyRef.current = "";
  }, [selectedModule?.id, defaultBaseBranch, defaultRemote]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const media = window.matchMedia("(max-width: 1260px)");
    const collapseWhenNarrow = (matches: boolean) => {
      if (matches) {
        setPanelCollapsed(true);
      }
    };
    collapseWhenNarrow(media.matches);
    const handler = (event: MediaQueryListEvent) => collapseWhenNarrow(event.matches);
    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", handler);
      return () => media.removeEventListener("change", handler);
    }
    media.addListener(handler);
    return () => media.removeListener(handler);
  }, []);

  useEffect(() => {
    if (groupedActions.length === 0) {
      setActiveGroup("");
      return;
    }
    if (!groupedActions.some(([groupLabel]) => groupLabel === activeGroup)) {
      setActiveGroup(groupedActions[0][0]);
    }
  }, [activeGroup, groupedActions]);

  useEffect(() => {
    if (!selectedModule || !activeAction || !isLandingAction(activeAction)) {
      return;
    }

    const nextKey = `${selectedModule.id}:${activeAction.id}`;
    if (lastAutoRunKeyRef.current === nextKey) {
      return;
    }

    lastAutoRunKeyRef.current = nextKey;
    void runAction(selectedModule, activeAction, {});
  }, [activeAction, runAction, selectedModule]);

  useEffect(() => {
    if (!selectedModule) {
      return;
    }

    const jobs: Array<{ moduleId: string; actionId: string }> = [];
    switch (selectedModule.id) {
      case "remote":
        if (state.catalog.remotes.length === 0) {
          jobs.push({ moduleId: "remote", actionId: "remote_list" });
        }
        break;
      case "branch":
        if (state.catalog.branches.length === 0) {
          jobs.push({ moduleId: "branch", actionId: "branch_list" });
        }
        if (state.catalog.remotes.length === 0) {
          jobs.push({ moduleId: "remote", actionId: "remote_list" });
        }
        break;
      case "worktree":
        if (state.catalog.worktrees.length === 0) {
          jobs.push({ moduleId: "worktree", actionId: "worktree_list" });
        }
        if (state.catalog.branches.length === 0) {
          jobs.push({ moduleId: "branch", actionId: "branch_list" });
        }
        if (state.catalog.issues.length === 0) {
          jobs.push({ moduleId: "issue", actionId: "issue_list" });
        }
        break;
      case "issue":
        if (state.catalog.issues.length === 0) {
          jobs.push({ moduleId: "issue", actionId: "issue_list" });
        }
        break;
      case "tag":
        if (state.catalog.tags.length === 0) {
          jobs.push({ moduleId: "tag", actionId: "tag_list" });
        }
        if (state.catalog.remotes.length === 0) {
          jobs.push({ moduleId: "remote", actionId: "remote_list" });
        }
        break;
      case "repo":
        if (state.catalog.repoStatus.length === 0) {
          jobs.push({ moduleId: "repo", actionId: "repo_status" });
        }
        if (state.catalog.remotes.length === 0) {
          jobs.push({ moduleId: "remote", actionId: "remote_list" });
        }
        break;
      case "pr":
        if (state.catalog.prs.length === 0) {
          jobs.push({ moduleId: "pr", actionId: "pr_list" });
        }
        if (state.catalog.branches.length === 0) {
          jobs.push({ moduleId: "branch", actionId: "branch_list" });
        }
        break;
      default:
        break;
    }

    jobs.forEach((job) => {
      void prefetchAction(job.moduleId, job.actionId);
    });
  }, [
    prefetchAction,
    selectedModule,
    state.catalog.branches.length,
    state.catalog.issues.length,
    state.catalog.prs.length,
    state.catalog.remotes.length,
    state.catalog.repoStatus.length,
    state.catalog.tags.length,
    state.catalog.worktrees.length,
  ]);

  if (!selectedModule) {
    return (
      <section className="module-actions min-h-0">
        <div className="empty-state">未找到模块，请刷新。</div>
      </section>
    );
  }

  if (actions.length === 0 || !activeAction) {
    return (
      <section className="module-actions min-h-0">
        <header className="module-actions__header">
          <h2>{selectedModule.title}</h2>
          <p>{selectedModule.description}</p>
        </header>
        <div className="empty-state">该模块暂时没有可执行动作。</div>
      </section>
    );
  }

  const executeAction = (action: ModuleAction, values: Record<string, string>) => {
    if (!selectedModule) {
      return;
    }

    const shouldReloadList = !isLandingAction(action) && !action.id.endsWith("_view") && Boolean(landingAction?.id);
    if (shouldReloadList && landingAction) {
      void runActionAndReload(selectedModule, action, values, landingAction.id);
      return;
    }

    void runAction(selectedModule, action, values);
  };

  const openDialog = (action: ModuleAction) => {
    setActiveActionId(action.id);
    setDraftValues(buildActionValues(action, {}, { base: defaultBaseBranch, remote: defaultRemote }));
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    setDraftValues(buildActionValues(activeAction, {}, { base: defaultBaseBranch, remote: defaultRemote }));
  };

  const onActionClick = (action: ModuleAction) => {
    setActiveActionId(action.id);
    if (isLandingAction(action)) {
      executeAction(action, {});
      return;
    }
    if ((action.fields?.length ?? 0) > 0) {
      openDialog(action);
      return;
    }
    executeAction(action, {});
  };

  const onPrimaryExecute = () => {
    if ((activeAction.fields?.length ?? 0) > 0) {
      openDialog(activeAction);
      return;
    }
    executeAction(activeAction, {});
  };

  const onDialogSubmit = (values: Record<string, string>) => {
    executeAction(activeAction, values);
    closeDialog();
  };

  return (
    <section className={panelCollapsed ? "module-actions module-actions--collapsed min-h-0" : "module-actions min-h-0"}>
      <header className="module-actions__header">
        <div>
          <h2>{selectedModule.title}</h2>
          <p>{selectedModule.description}</p>
        </div>
        <div className="module-actions__summary">
          <span>{actions.length} 个功能</span>
          <strong>{classifyAction(activeAction.id)}</strong>
          <Button variant="ghost" onClick={() => setPanelCollapsed((value) => !value)}>
            {panelCollapsed ? "展开方法" : "收起方法"}
          </Button>
        </div>
      </header>

      {panelCollapsed ? (
        <section className="module-actions__collapsed">
          <div className="module-actions__collapsed-meta">
            <strong>{activeAction.title}</strong>
            <span>{activeAction.description}</span>
          </div>
          <div className="module-actions__collapsed-actions">
            {landingAction ? (
              <Button variant="ghost" onClick={() => executeAction(landingAction, {})} disabled={state.busy}>
                刷新列表
              </Button>
            ) : null}
            <Button variant="primary" onClick={onPrimaryExecute} disabled={state.busy}>
              {actionVerb(activeAction)}
            </Button>
          </div>
        </section>
      ) : (
        <section className="module-toolbar">
          <div className="module-toolbar__meta">
            <h3>{activeAction.title}</h3>
            <p>{activeAction.description}</p>
          </div>
          <div className="module-toolbar__actions">
            <span className="module-toolbar__hint">
              {(activeAction.fields?.length ?? 0) > 0 ? `${activeAction.fields?.length} 个参数` : "无额外参数"}
            </span>
            {landingAction ? (
              <Button variant="ghost" onClick={() => executeAction(landingAction, {})} disabled={state.busy}>
                刷新列表
              </Button>
            ) : null}
            <Button variant="primary" onClick={onPrimaryExecute} disabled={state.busy}>
              {actionVerb(activeAction)}
            </Button>
          </div>
        </section>
      )}

      {!panelCollapsed ? (
        <Tabs
          className="module-actions__tabs"
          activeKey={activeGroup || groupedActions[0]?.[0]}
          onChange={setActiveGroup}
          items={groupedActions.map(([groupLabel, groupActions]) => ({
            key: groupLabel,
            label: `${groupLabel} (${groupActions.length})`,
            children: (
              <div className="module-actions__chips" role="list">
                {groupActions.map((action) => {
                  const active = activeAction.id === action.id;
                  return (
                    <button
                      key={action.id}
                      type="button"
                      className={active ? "module-actions__chip module-actions__chip--active" : "module-actions__chip"}
                      onClick={() => onActionClick(action)}
                      title={action.description}
                    >
                      <span>{action.title}</span>
                      {(action.fields?.length ?? 0) > 0 ? <small>{action.fields?.length} 参数</small> : null}
                    </button>
                  );
                })}
              </div>
            ),
          }))}
        />
      ) : null}

      {dialogOpen ? (
        <ActionDialog
          action={activeAction}
          moduleId={selectedModule.id}
          catalog={state.catalog}
          values={draftValues}
          busy={state.busy}
          onChange={setDraftValues}
          onClose={closeDialog}
          onSubmit={onDialogSubmit}
        />
      ) : null}
    </section>
  );
}
