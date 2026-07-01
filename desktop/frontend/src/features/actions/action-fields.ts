import type { ModuleAction, OutputListItem, ResourceCatalog } from "../../app/types";

export interface FieldOption {
  label: string;
  value: string;
}

function toOptions(items: OutputListItem[]): FieldOption[] {
  return items
    .map((item) => ({
      label: item.secondary ? `${item.primary} · ${item.secondary}` : item.primary,
      value: item.value ?? item.primary,
    }))
    .filter((item, index, list) => item.value && list.findIndex((candidate) => candidate.value === item.value) === index);
}

export function buildActionValues(
  action: ModuleAction | null,
  seedValues: Record<string, string> = {},
  preferredDefaults: Record<string, string> = {}
): Record<string, string> {
  const out: Record<string, string> = { ...seedValues };
  for (const field of action?.fields ?? []) {
    if (!out[field.key]) {
      out[field.key] = preferredDefaults[field.key] || field.default || "";
    }
  }
  return out;
}

export function resolveActionFieldOptions(moduleID: string, actionID: string, fieldKey: string, catalog: ResourceCatalog): FieldOption[] {
  if (fieldKey === "method") {
    return [
      { label: "squash", value: "squash" },
      { label: "merge", value: "merge" },
      { label: "rebase", value: "rebase" },
    ];
  }

  if (moduleID === "branch" && fieldKey === "name" && (actionID === "branch_checkout" || actionID === "branch_delete" || actionID === "branch_force_sync")) {
    return toOptions(
      actionID === "branch_delete"
        ? catalog.branches.filter((item) => !item.active)
        : catalog.branches
    );
  }

  if (moduleID === "worktree" && fieldKey === "base") {
    return toOptions(catalog.branches);
  }

  if (moduleID === "worktree" && fieldKey === "target") {
    if (actionID === "worktree_remove") {
      return toOptions(catalog.worktrees);
    }

    return [
      ...toOptions(catalog.issues),
      ...toOptions(catalog.branches.filter((item) => !item.active)),
    ];
  }

  if (moduleID === "issue" && actionID === "issue_view" && fieldKey === "id") {
    return toOptions(catalog.issues);
  }

  if (moduleID === "tag" && actionID === "tag_push" && fieldKey === "name") {
    return toOptions(catalog.tags);
  }

  if (moduleID === "pr" && actionID === "pr_create" && fieldKey === "base") {
    return toOptions(catalog.branches);
  }

  return [];
}
