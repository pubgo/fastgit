import type { ModuleAction } from "../../app/types";

export function classifyAction(actionID: string): string {
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

export function isLandingAction(action: ModuleAction): boolean {
  return (action.id.endsWith("_list") || action.id.endsWith("_status")) && (action.fields?.length ?? 0) === 0;
}

export function pickDefaultAction(actions: ModuleAction[]): ModuleAction | null {
  return actions.find(isLandingAction) ?? actions[0] ?? null;
}

export function isViewAction(action: ModuleAction): boolean {
  return action.id.endsWith("_view");
}

export function actionVerb(action: ModuleAction | null): string {
  if (!action) {
    return "执行";
  }
  return (action.fields?.length ?? 0) > 0 ? "填写并执行" : "立即执行";
}
