import { FormEvent, useEffect, useRef, useState } from "react";

import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useAppContext } from "../../app/providers/app-context";
import type { ModuleAction } from "../../app/types";

function classifyAction(actionID: string): string {
  if (actionID.endsWith("_list") || actionID.endsWith("_status")) {
    return "列表";
  }
  if (actionID.endsWith("_view")) {
    return "详情";
  }
  if (actionID.endsWith("_create") || actionID.endsWith("_publish")) {
    return "新增";
  }
  if (actionID.endsWith("_delete") || actionID.endsWith("_remove")) {
    return "删除";
  }
  if (actionID.endsWith("_checkout") || actionID.endsWith("_switch")) {
    return "切换";
  }
  if (actionID.endsWith("_pull") || actionID.endsWith("_push") || actionID.endsWith("_sync") || actionID.endsWith("_merge")) {
    return "执行";
  }
  return "操作";
}

function isLandingAction(action: ModuleAction): boolean {
  return (action.id.endsWith("_list") || action.id.endsWith("_status")) && (action.fields?.length ?? 0) === 0;
}

function pickDefaultAction(actions: ModuleAction[]): ModuleAction | null {
  return actions.find(isLandingAction) ?? actions[0] ?? null;
}

function defaultValuesFor(action: ModuleAction | null): Record<string, string> {
  const out: Record<string, string> = {};
  for (const field of action?.fields ?? []) {
    out[field.key] = field.default || "";
  }
  return out;
}

function actionVerb(action: ModuleAction | null): string {
  if (!action) {
    return "执行";
  }
  return (action.fields?.length ?? 0) > 0 ? "填写并执行" : "立即执行";
}

export function ModuleActions() {
  const { selectedModule, runAction } = useAppContext();
  const actions = selectedModule?.actions ?? [];
  const [activeActionId, setActiveActionId] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [draftValues, setDraftValues] = useState<Record<string, string>>({});
  const lastAutoRunKeyRef = useRef<string>("");

  const activeAction = actions.find((action) => action.id === activeActionId) ?? pickDefaultAction(actions);

  useEffect(() => {
    const nextAction = pickDefaultAction(actions);
    setActiveActionId(nextAction?.id ?? null);
    setDialogOpen(false);
    setDraftValues(defaultValuesFor(nextAction));
    lastAutoRunKeyRef.current = "";
  }, [selectedModule?.id, actions]);

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

  const openDialog = (action: ModuleAction) => {
    setActiveActionId(action.id);
    setDraftValues(defaultValuesFor(action));
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    setDraftValues(defaultValuesFor(activeAction));
  };

  const onActionClick = (action: ModuleAction) => {
    setActiveActionId(action.id);
    if (isLandingAction(action)) {
      return;
    }
    if ((action.fields?.length ?? 0) > 0) {
      openDialog(action);
    }
  };

  const executeAction = (action: ModuleAction, values: Record<string, string>) => {
    if (!selectedModule) {
      return;
    }
    void runAction(selectedModule, action, values);
  };

  const onPrimaryExecute = () => {
    if ((activeAction.fields?.length ?? 0) > 0) {
      openDialog(activeAction);
      return;
    }
    executeAction(activeAction, {});
  };

  const onDialogSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    executeAction(activeAction, draftValues);
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

      <div className="module-actions__tabs" role="tablist" aria-label={`${selectedModule.title} actions`}>
        {actions.map((action) => {
          const active = activeAction.id === action.id;
          return (
            <button
              key={action.id}
              type="button"
              role="tab"
              aria-selected={active}
              className={active ? "module-actions__tab module-actions__tab--active" : "module-actions__tab"}
              onClick={() => onActionClick(action)}
            >
              <span>{action.title}</span>
              <small>{classifyAction(action.id)}</small>
            </button>
          );
        })}
      </div>

      <section className="module-toolbar">
        <div className="module-toolbar__meta">
          <h3>{activeAction.title}</h3>
          <p>{activeAction.description}</p>
        </div>
        <div className="module-toolbar__actions">
          {(activeAction.fields?.length ?? 0) > 0 ? (
            <span className="module-toolbar__hint">{activeAction.fields?.length} 个参数</span>
          ) : (
            <span className="module-toolbar__hint">无额外参数</span>
          )}
          <Button variant="ghost" onClick={() => executeAction(activeAction, {})} disabled={(activeAction.fields?.length ?? 0) > 0}>
            刷新
          </Button>
          <Button variant="primary" onClick={onPrimaryExecute}>
            {actionVerb(activeAction)}
          </Button>
        </div>
      </section>

      {dialogOpen ? (
        <div className="action-dialog__backdrop" onClick={closeDialog}>
          <section
            className="action-dialog"
            role="dialog"
            aria-modal="true"
            aria-label={activeAction.title}
            onClick={(event) => event.stopPropagation()}
          >
            <header className="action-dialog__header">
              <div>
                <h3>{activeAction.title}</h3>
                <p>{activeAction.description}</p>
              </div>
              <button type="button" className="action-dialog__close" onClick={closeDialog} aria-label="close dialog">
                ×
              </button>
            </header>
            <form onSubmit={onDialogSubmit} className="action-dialog__form">
              <div className="action-dialog__fields">
                {(activeAction.fields ?? []).map((field) => (
                  <label key={field.key} className="action-field">
                    <span>{field.label}</span>
                    {field.key.toLowerCase().includes("body") ? (
                      <textarea
                        className="ui-textarea"
                        name={field.key}
                        placeholder={field.placeholder || field.label}
                        value={draftValues[field.key] ?? ""}
                        required={Boolean(field.required)}
                        onChange={(event) =>
                          setDraftValues((prev) => ({
                            ...prev,
                            [field.key]: event.target.value,
                          }))
                        }
                      />
                    ) : (
                      <Input
                        name={field.key}
                        placeholder={field.placeholder || field.label}
                        value={draftValues[field.key] ?? ""}
                        required={Boolean(field.required)}
                        onChange={(event) =>
                          setDraftValues((prev) => ({
                            ...prev,
                            [field.key]: event.target.value,
                          }))
                        }
                      />
                    )}
                  </label>
                ))}
              </div>
              <footer className="action-dialog__footer">
                <Button type="button" variant="ghost" onClick={closeDialog}>
                  取消
                </Button>
                <Button type="submit" variant="primary">
                  执行
                </Button>
              </footer>
            </form>
          </section>
        </div>
      ) : null}
    </section>
  );
}
