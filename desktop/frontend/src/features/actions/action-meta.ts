import type { ModuleAction } from "../../app/types";

export function classifyAction(actionID: string): string {
  if (actionID.endsWith("_force_sync")) {
    return "危险操作";
  }
  if (actionID.endsWith("_list") || actionID.endsWith("_status")) {
    return "列表";
  }
  if (actionID.endsWith("_view")) {
    return "详情";
  }
  if (actionID.endsWith("_create") || actionID.endsWith("_publish")) {
    return "新增";
  }
  if (actionID.endsWith("_add")) {
    return "新增";
  }
  if (actionID.endsWith("_rename")) {
    return "编辑";
  }
  if (actionID.endsWith("_update")) {
    return "编辑";
  }
  if (actionID.endsWith("_delete") || actionID.endsWith("_remove") || actionID.endsWith("_discard")) {
    return "删除";
  }
  if (actionID.endsWith("_close")) {
    return "关闭";
  }
  if (actionID.endsWith("_set_url")) {
    return "编辑";
  }
  if (actionID.endsWith("_set_push_url")) {
    return "编辑";
  }
  if (actionID.endsWith("_checkout") || actionID.endsWith("_switch")) {
    return "切换";
  }
  if (actionID.endsWith("_pull") || actionID.endsWith("_push") || actionID.endsWith("_sync") || actionID.endsWith("_merge") || actionID.endsWith("_fetch") || actionID.endsWith("_fetch_all")) {
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

export function actionSubmitLabel(action: ModuleAction | null): string {
  if (!action) {
    return "执行";
  }
  if (action.id.endsWith("_add") || action.id.endsWith("_create") || action.id.endsWith("_publish")) {
    return "创建";
  }
  if (action.id.endsWith("_rename") || action.id.endsWith("_update") || action.id.endsWith("_set_url") || action.id.endsWith("_set_push_url")) {
    return "保存";
  }
  if (action.id.endsWith("_delete") || action.id.endsWith("_remove")) {
    return "删除";
  }
  if (action.id.endsWith("_discard")) {
    return "丢弃";
  }
  if (action.id.endsWith("_close")) {
    return "关闭";
  }
  if (action.id.endsWith("_force_sync")) {
    return "强制对齐";
  }
  if (action.id.endsWith("_fetch") || action.id.endsWith("_fetch_all")) {
    return "抓取";
  }
  if (action.id.endsWith("_pull")) {
    return "拉取";
  }
  if (action.id.endsWith("_push")) {
    return "推送";
  }
  if (action.id.endsWith("_checkout") || action.id.endsWith("_switch")) {
    return "切换";
  }
  return "执行";
}
