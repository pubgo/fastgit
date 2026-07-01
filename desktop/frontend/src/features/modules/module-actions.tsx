import { useEffect, useRef, useState } from "react";

import { useAppContext } from "../../app/providers/app-context";
import type { ModuleAction } from "../../app/types";
import { Button } from "../../components/ui/button";
import { ActionDialog } from "../actions/action-dialog";
import { buildActionValues } from "../actions/action-fields";
import { actionVerb, classifyAction, isLandingAction, pickDefaultAction } from "../actions/action-meta";

export function ModuleActions() {
  const { selectedModule, runAction, runActionAndReload, prefetchAction, state } = useAppContext();
  const actions = selectedModule?.actions ?? [];
  const [activeActionId, setActiveActionId] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [draftValues, setDraftValues] = useState<Record<string, string>>({});
  const lastAutoRunKeyRef = useRef<string>("");

  const activeAction = actions.find((action) => action.id === activeActionId) ?? pickDefaultAction(actions);
  const landingAction = pickDefaultAction(actions);
  const defaultBaseBranch = state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "";

  useEffect(() => {
    const nextAction = pickDefaultAction(actions);
    setActiveActionId(nextAction?.id ?? null);
    setDialogOpen(false);
    setDraftValues(buildActionValues(nextAction, {}, { base: defaultBaseBranch }));
    lastAutoRunKeyRef.current = "";
  }, [selectedModule?.id, actions, defaultBaseBranch]);

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
      case "branch":
        if (state.catalog.branches.length === 0) {
          jobs.push({ moduleId: "branch", actionId: "branch_list" });
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
        break;
      case "repo":
        if (state.catalog.repoStatus.length === 0) {
          jobs.push({ moduleId: "repo", actionId: "repo_status" });
        }
        break;
      case "pr":
        if (state.catalog.prs.length === 0) {
          jobs.push({ moduleId: "pr", actionId: "pr_status" });
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
    state.catalog.repoStatus.length,
    state.catalog.tags.length,
    state.catalog.worktrees.length,
  ]);

  if (!selectedModule) {
    return (
      <section className="module-actions">
        <div className="empty-state">未找到模块，请刷新。</div>
      </section>
    );
  }

  if (actions.length === 0 || !activeAction) {
    return (
      <section className="module-actions">
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
    setDraftValues(buildActionValues(action, {}, { base: defaultBaseBranch }));
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    setDraftValues(buildActionValues(activeAction, {}, { base: defaultBaseBranch }));
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
    <section className="module-actions">
      <header className="module-actions__header">
        <div>
          <h2>{selectedModule.title}</h2>
          <p>{selectedModule.description}</p>
        </div>
        <div className="module-actions__summary">
          <span>{actions.length} 个功能</span>
          <strong>{classifyAction(activeAction.id)}</strong>
        </div>
      </header>

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

      <div className="module-actions__quicklist" role="list" aria-label={`${selectedModule.title} quick actions`}>
        {actions.map((action) => {
          const active = activeAction.id === action.id;
          return (
            <button
              key={action.id}
              type="button"
              className={active ? "module-actions__quickitem module-actions__quickitem--active" : "module-actions__quickitem"}
              onClick={() => onActionClick(action)}
            >
              <strong>{action.title}</strong>
              <span>{action.description}</span>
              <small>{classifyAction(action.id)}</small>
            </button>
          );
        })}
      </div>

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
