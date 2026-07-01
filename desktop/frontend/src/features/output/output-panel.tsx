import { useEffect, useMemo, useRef, useState } from "react";

import { useAppContext } from "../../app/providers/app-context";
import type { ModuleAction, OutputListItem } from "../../app/types";
import { Button } from "../../components/ui/button";
import { ActionDialog } from "../actions/action-dialog";
import { buildActionValues } from "../actions/action-fields";
import { isViewAction } from "../actions/action-meta";

type SortMode = "default" | "name-asc" | "name-desc" | "active-first" | "number-desc" | "number-asc" | "status";
type ViewMode = "list" | "detail" | "raw";
type ColumnDef = {
  key: string;
  label: string;
  width?: string;
  className?: string;
  render: (item: OutputListItem) => string;
};
type RowAction = {
  label: string;
  actionId: string;
  values: Record<string, string>;
  disabled?: boolean;
};
type ToolbarAction = RowAction & {
  variant?: "primary" | "ghost";
  tone?: "default" | "danger";
};

type DetailEntry = {
  label: string;
  value: string;
  href?: string;
};

function findModuleAction(moduleActions: ModuleAction[] | null | undefined, actionId: string): ModuleAction | null {
  return moduleActions?.find((item) => item.id === actionId) ?? null;
}

function getField(item: OutputListItem, key: string, fallback = "-"): string {
  return item.fields?.[key] || fallback;
}

function getNumberField(item: OutputListItem, key = "number"): number {
  const raw = item.fields?.[key] ?? item.value ?? "";
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) ? value : 0;
}

function getTableConfig(actionId: string | undefined): {
  columns: ColumnDef[];
  searchPlaceholder: string;
  filterLabel: string;
} {
  switch (actionId) {
    case "branch_list":
      return {
        searchPlaceholder: "搜索分支名或状态...",
        filterLabel: "全部状态",
        columns: [
          {
            key: "branch",
            label: "分支",
            width: "minmax(280px, 2fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "branch", item.primary),
          },
          {
            key: "status",
            label: "状态",
            width: "minmax(120px, 0.8fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "status", item.badge ?? "-"),
          },
        ],
      };
    case "worktree_list":
      return {
        searchPlaceholder: "搜索路径、分支、提交...",
        filterLabel: "全部状态",
        columns: [
          {
            key: "path",
            label: "路径",
            width: "minmax(280px, 2.2fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "path", item.primary),
          },
          { key: "branch", label: "分支", width: "minmax(160px, 1.1fr)", render: (item) => getField(item, "branch") },
          { key: "commit", label: "提交", width: "minmax(120px, 0.8fr)", render: (item) => getField(item, "commit") },
          {
            key: "status",
            label: "状态",
            width: "minmax(120px, 0.8fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "status", item.badge ?? "-"),
          },
        ],
      };
    case "issue_list":
      return {
        searchPlaceholder: "搜索 issue 编号、标题、状态...",
        filterLabel: "全部状态",
        columns: [
          { key: "number", label: "编号", width: "minmax(100px, 0.7fr)", render: (item) => `#${getField(item, "number", item.value ?? item.primary)}` },
          {
            key: "title",
            label: "标题",
            width: "minmax(320px, 2.3fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "title", item.primary),
          },
          {
            key: "state",
            label: "状态",
            width: "minmax(120px, 0.8fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "state", item.badge ?? "-"),
          },
        ],
      };
    case "pr_status":
      return {
        searchPlaceholder: "搜索 PR 编号、标题、状态...",
        filterLabel: "全部状态",
        columns: [
          { key: "number", label: "编号", width: "minmax(100px, 0.7fr)", render: (item) => `#${getField(item, "number", item.value ?? item.primary)}` },
          {
            key: "title",
            label: "标题",
            width: "minmax(320px, 2.3fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "title", item.primary),
          },
          {
            key: "state",
            label: "状态",
            width: "minmax(120px, 0.8fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "state", item.badge ?? "-"),
          },
        ],
      };
    case "tag_list":
      return {
        searchPlaceholder: "搜索 tag...",
        filterLabel: "全部标签",
        columns: [
          {
            key: "tag",
            label: "Tag",
            width: "minmax(320px, 2fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "tag", item.primary),
          },
        ],
      };
    case "repo_status":
      return {
        searchPlaceholder: "搜索文件路径或状态码...",
        filterLabel: "全部状态",
        columns: [
          {
            key: "path",
            label: "文件",
            width: "minmax(320px, 2.2fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "path", item.primary),
          },
          {
            key: "status",
            label: "状态",
            width: "minmax(120px, 0.8fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "status", item.badge ?? "-"),
          },
        ],
      };
    default:
      return {
        searchPlaceholder: "搜索名称、状态、分支...",
        filterLabel: "全部状态",
        columns: [
          {
            key: "primary",
            label: "资源",
            width: "minmax(260px, 1.8fr)",
            className: "output-table__cell--primary",
            render: (item) => item.primary,
          },
          { key: "secondary", label: "详情", width: "minmax(200px, 1.2fr)", render: (item) => item.secondary ?? "-" },
          {
            key: "badge",
            label: "状态",
            width: "minmax(120px, 0.8fr)",
            className: "output-table__cell--status",
            render: (item) => item.badge ?? "-",
          },
        ],
      };
  }
}

function describeSelectedItem(actionId: string | undefined, item: OutputListItem): DetailEntry[] {
  switch (actionId) {
    case "branch_list":
      return [
        { label: "分支", value: getField(item, "branch", item.value ?? item.primary) },
        { label: "状态", value: getField(item, "status", item.badge ?? (item.active ? "current" : "normal")) },
      ];
    case "worktree_list":
      return [
        { label: "路径", value: getField(item, "path", item.primary) },
        { label: "分支", value: getField(item, "branch") },
        { label: "提交", value: getField(item, "commit") },
        { label: "状态", value: getField(item, "status", item.badge ?? "ready") },
      ];
    case "issue_list":
      return [
        { label: "Issue", value: `#${getField(item, "number", item.value ?? item.primary)}` },
        { label: "标题", value: getField(item, "title", item.primary) },
        { label: "状态", value: getField(item, "state", item.badge ?? "-") },
      ];
    case "tag_list":
      return [{ label: "Tag", value: getField(item, "tag", item.value ?? item.primary) }];
    case "repo_status":
      return [
        { label: "文件", value: getField(item, "path", item.primary) },
        { label: "状态", value: getField(item, "status", item.badge ?? "-") },
      ];
    case "pr_status":
      return [
        { label: "PR", value: `#${getField(item, "number", item.value ?? item.primary)}` },
        { label: "标题", value: getField(item, "title", item.primary) },
        { label: "状态", value: getField(item, "state", item.badge ?? "-") },
        {
          label: "链接",
          value: item.url ?? getField(item, "url", item.secondary ?? "-"),
          href: item.url ?? (getField(item, "url", "") || undefined),
        },
      ];
    default:
      return [
        { label: "名称", value: item.primary },
        { label: "详情", value: item.secondary ?? "-" },
      ];
  }
}

function buildRowActions(actionId: string | undefined, item: OutputListItem): RowAction[] {
  switch (actionId) {
    case "branch_list":
      return [
        { label: "对齐远端", actionId: "branch_force_sync", values: { name: item.value ?? item.primary } },
        { label: "切换", actionId: "branch_checkout", values: { name: item.value ?? item.primary }, disabled: item.active },
        { label: "删除", actionId: "branch_delete", values: { name: item.value ?? item.primary }, disabled: item.active },
      ];
    case "issue_list":
      return [{ label: "查看", actionId: "issue_view", values: { id: item.value ?? item.primary } }];
    case "tag_list":
      return [{ label: "推送", actionId: "tag_push", values: { name: item.value ?? item.primary } }];
    case "worktree_list":
      return [{ label: "删除", actionId: "worktree_remove", values: { target: item.value ?? item.primary }, disabled: item.active }];
    case "pr_status":
      return [
        { label: "同步", actionId: "pr_sync", values: {} },
        { label: "合并", actionId: "pr_merge", values: { method: "squash" } },
      ];
    default:
      return [];
  }
}

function buildToolbarActions(actionId: string | undefined, item: OutputListItem | null): ToolbarAction[] {
  switch (actionId) {
    case "branch_list":
      return [
        { label: "刷新列表", actionId: "branch_list", values: {}, variant: "ghost" },
        { label: "新建分支", actionId: "branch_create", values: {}, variant: "primary" },
        ...(item
          ? [
              { label: "强制对齐远端", actionId: "branch_force_sync", values: { name: item.value ?? item.primary }, variant: "ghost" as const, tone: "danger" as const },
              { label: "切换分支", actionId: "branch_checkout", values: { name: item.value ?? item.primary }, disabled: item.active, variant: "ghost" as const },
              { label: "删除分支", actionId: "branch_delete", values: { name: item.value ?? item.primary }, disabled: item.active, variant: "ghost" as const, tone: "danger" as const },
            ]
          : []),
      ];
    case "worktree_list":
      return [
        { label: "刷新列表", actionId: "worktree_list", values: {}, variant: "ghost" },
        { label: "新建 Worktree", actionId: "worktree_create", values: {}, variant: "primary" },
        ...(item
          ? [{ label: "删除 Worktree", actionId: "worktree_remove", values: { target: item.value ?? item.primary }, disabled: item.active, variant: "ghost" as const, tone: "danger" as const }]
          : []),
      ];
    case "issue_list":
      return [
        { label: "刷新列表", actionId: "issue_list", values: {}, variant: "ghost" },
        { label: "新建 Issue", actionId: "issue_create", values: {}, variant: "primary" },
        ...(item ? [{ label: "查看详情", actionId: "issue_view", values: { id: item.value ?? item.primary }, variant: "ghost" as const }] : []),
      ];
    case "pr_status":
      return [
        { label: "刷新状态", actionId: "pr_status", values: {}, variant: "ghost" },
        { label: "创建 PR", actionId: "pr_create", values: {}, variant: "primary" },
        ...(item
          ? [
              { label: "同步 PR", actionId: "pr_sync", values: {}, variant: "ghost" as const },
              { label: "合并 PR", actionId: "pr_merge", values: { method: "squash" }, variant: "ghost" as const },
            ]
          : []),
      ];
    case "tag_list":
      return [
        { label: "刷新列表", actionId: "tag_list", values: {}, variant: "ghost" },
        { label: "创建 Tag", actionId: "tag_publish", values: {}, variant: "primary" },
        ...(item ? [{ label: "推送 Tag", actionId: "tag_push", values: { name: item.value ?? item.primary }, variant: "ghost" as const }] : []),
      ];
    case "repo_status":
      return [
        { label: "刷新状态", actionId: "repo_status", values: {}, variant: "ghost" },
        { label: "拉取", actionId: "repo_pull", values: {}, variant: "primary" },
        { label: "推送", actionId: "repo_push", values: {}, variant: "ghost" },
      ];
    default:
      return [];
  }
}

function getDefaultSortMode(actionId: string | undefined): SortMode {
  switch (actionId) {
    case "branch_list":
    case "worktree_list":
      return "active-first";
    case "issue_list":
    case "pr_status":
      return "number-desc";
    case "tag_list":
      return "name-desc";
    case "repo_status":
      return "status";
    default:
      return "default";
  }
}

function getSortOptions(actionId: string | undefined): Array<{ value: SortMode; label: string }> {
  switch (actionId) {
    case "issue_list":
    case "pr_status":
      return [
        { value: "number-desc", label: "编号新到旧" },
        { value: "number-asc", label: "编号旧到新" },
        { value: "name-asc", label: "标题 A-Z" },
        { value: "name-desc", label: "标题 Z-A" },
        { value: "status", label: "状态分组" },
      ];
    case "repo_status":
      return [
        { value: "status", label: "状态分组" },
        { value: "name-asc", label: "文件 A-Z" },
        { value: "name-desc", label: "文件 Z-A" },
      ];
    case "tag_list":
      return [
        { value: "name-desc", label: "Tag Z-A" },
        { value: "name-asc", label: "Tag A-Z" },
      ];
    case "branch_list":
    case "worktree_list":
      return [
        { value: "active-first", label: "当前项优先" },
        { value: "name-asc", label: "名称 A-Z" },
        { value: "name-desc", label: "名称 Z-A" },
      ];
    default:
      return [
        { value: "default", label: "默认顺序" },
        { value: "name-asc", label: "名称 A-Z" },
        { value: "name-desc", label: "名称 Z-A" },
      ];
  }
}

function shouldOpenActionDialog(action: ModuleAction, values: Record<string, string>): boolean {
  const fields = action.fields ?? [];
  if (fields.length === 0) {
    return false;
  }
  if (action.id.startsWith("pr_") || action.id === "branch_force_sync") {
    return true;
  }
  return fields.some((field) => !values[field.key]);
}

function detailMatchesItem(detailTargetId: string | undefined, item: OutputListItem | null): boolean {
  if (!detailTargetId || !item) {
    return false;
  }
  return detailTargetId === item.id || detailTargetId === item.value;
}

export function OutputPanel() {
  const { state, runAction, runActionAndReload, previewAction } = useAppContext();
  const items = state.output.items ?? [];
  const hasList = items.length > 0;
  const isManageView = Boolean(state.output.actionId && (state.output.actionId.endsWith("_list") || state.output.actionId.endsWith("_status")));
  const defaultBaseBranch = state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "";
  const [query, setQuery] = useState("");
  const [badgeFilter, setBadgeFilter] = useState("all");
  const [sortMode, setSortMode] = useState<SortMode>("default");
  const [viewMode, setViewMode] = useState<ViewMode>("raw");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [previewingId, setPreviewingId] = useState<string | null>(null);
  const [dialogAction, setDialogAction] = useState<ModuleAction | null>(null);
  const [dialogValues, setDialogValues] = useState<Record<string, string>>({});
  const lastPreviewKeyRef = useRef<string>("");

  useEffect(() => {
    setQuery("");
    setBadgeFilter("all");
    setSortMode(getDefaultSortMode(state.output.actionId));
    setViewMode(items.length > 0 ? "list" : state.output.detail ? "detail" : "raw");
    setSelectedId(null);
    setPreviewingId(null);
    setDialogAction(null);
    setDialogValues({});
    lastPreviewKeyRef.current = "";
  }, [items.length, state.output.actionId, state.output.command, state.output.detail, state.output.title]);

  const badgeOptions = useMemo(
    () =>
      Array.from(new Set(items.map((item) => item.badge).filter((badge): badge is string => Boolean(badge)))),
    [items]
  );
  const sortOptions = useMemo(() => getSortOptions(state.output.actionId), [state.output.actionId]);
  const tableConfig = useMemo(() => getTableConfig(state.output.actionId), [state.output.actionId]);
  const tableColumns = useMemo(
    () => [...tableConfig.columns.map((column) => column.width ?? "minmax(160px, 1fr)"), "minmax(140px, auto)"].join(" "),
    [tableConfig.columns]
  );

  const visibleItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const filtered = items.filter((item) => {
      if (badgeFilter !== "all" && item.badge !== badgeFilter) {
        return false;
      }
      if (!normalizedQuery) {
        return true;
      }
      const haystack = [
        item.primary,
        item.secondary,
        item.badge,
        item.value,
        item.category,
        ...Object.values(item.fields ?? {}),
        ...(item.keywords ?? []),
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return haystack.includes(normalizedQuery);
    });

    if (sortMode === "default") {
      return filtered;
    }

    const sorted = [...filtered];
    if (sortMode === "active-first") {
      sorted.sort((left, right) => {
        if (left.active === right.active) {
          return left.primary.localeCompare(right.primary);
        }
        return left.active ? -1 : 1;
      });
      return sorted;
    }

    if (sortMode === "number-desc" || sortMode === "number-asc") {
      sorted.sort((left, right) => {
        const delta = getNumberField(right) - getNumberField(left);
        return sortMode === "number-desc" ? delta : -delta;
      });
      return sorted;
    }

    if (sortMode === "status") {
      sorted.sort((left, right) => {
        const leftStatus = left.badge ?? "";
        const rightStatus = right.badge ?? "";
        if (leftStatus === rightStatus) {
          return left.primary.localeCompare(right.primary);
        }
        return leftStatus.localeCompare(rightStatus);
      });
      return sorted;
    }

    sorted.sort((left, right) =>
      sortMode === "name-asc"
        ? left.primary.localeCompare(right.primary)
        : right.primary.localeCompare(left.primary)
    );
    return sorted;
  }, [badgeFilter, items, query, sortMode]);

  const hasFilteredItems = visibleItems.length > 0;
  const sourceModule = state.output.moduleId ? state.modules.find((item) => item.id === state.output.moduleId) ?? null : null;
  const selectedItem = visibleItems.find((item) => item.id === selectedId) ?? visibleItems[0] ?? null;
  const selectedDetails = selectedItem ? describeSelectedItem(state.output.actionId, selectedItem) : [];
  const toolbarActions = sourceModule ? buildToolbarActions(state.output.actionId, selectedItem) : [];
  const sideDetail =
    state.output.detail && (!state.output.detail.targetId || detailMatchesItem(state.output.detail.targetId, selectedItem))
      ? state.output.detail
      : null;

  useEffect(() => {
    if (state.output.actionId !== "issue_list" || !selectedItem || !sourceModule) {
      return;
    }

    const issueId = selectedItem.value ?? selectedItem.primary;
    const previewKey = `issue:${issueId}`;
    if (lastPreviewKeyRef.current === previewKey && sideDetail?.targetId === issueId) {
      return;
    }

    lastPreviewKeyRef.current = previewKey;
    setPreviewingId(selectedItem.id);
    void previewAction(sourceModule.id, "issue_view", { id: issueId }).finally(() => {
      setPreviewingId((current) => (current === selectedItem.id ? null : current));
    });
  }, [previewAction, selectedItem, sideDetail?.targetId, sourceModule, state.output.actionId]);

  const performAction = (action: ModuleAction, values: Record<string, string>) => {
    if (isViewAction(action) && sourceModule) {
      return previewAction(sourceModule.id, action.id, values);
    }

    const canReload = Boolean(state.output.actionId && (state.output.actionId.endsWith("_list") || state.output.actionId.endsWith("_status")));
    return canReload && sourceModule && state.output.actionId && !isViewAction(action)
      ? runActionAndReload(sourceModule, action, values, state.output.actionId)
      : sourceModule
        ? runAction(sourceModule, action, values)
        : Promise.resolve();
  };

  const openActionDialog = (action: ModuleAction, item: OutputListItem | null, rowValues: Record<string, string>) => {
    const seedValues = { ...rowValues };
    if (item && action.id === "pr_sync") {
      if (!seedValues.title) {
        seedValues.title = getField(item, "title", item.primary);
      }
      if (!seedValues.body && sideDetail?.body && detailMatchesItem(sideDetail.targetId, item)) {
        seedValues.body = sideDetail.body;
      }
    }

    setDialogAction(action);
    setDialogValues(buildActionValues(action, seedValues, { base: defaultBaseBranch }));
  };

  const closeActionDialog = () => {
    setDialogAction(null);
    setDialogValues({});
  };

  const hasStructuredDetail = Boolean(sideDetail || selectedItem || previewingId);

  return (
    <section className="output-panel">
      <header className="output-panel__header">
        <div>
          <h2>{state.output.title}</h2>
          <p>{state.output.command}</p>
        </div>
        <span className={state.output.exitCode === 0 ? "status-pill status-pill--ok" : "status-pill status-pill--error"}>
          exit {state.output.exitCode}
        </span>
      </header>
      {isManageView && sourceModule && toolbarActions.length > 0 ? (
        <div className="output-contextbar">
          <div className="output-contextbar__meta">
            <strong>{sourceModule.title}</strong>
            <span>{selectedItem ? `当前选中: ${selectedItem.primary}` : state.output.emptyHint ?? "暂无资源"}</span>
          </div>
          <div className="output-contextbar__actions">
            {toolbarActions.map((toolbarAction) => {
              const nextAction = findModuleAction(sourceModule.actions, toolbarAction.actionId);
              if (!nextAction) {
                return null;
              }
              return (
                <Button
                  key={`toolbar-${toolbarAction.actionId}`}
                  variant={toolbarAction.variant ?? "ghost"}
                  className={toolbarAction.tone === "danger" ? "output-toolbar__action output-toolbar__action--danger" : "output-toolbar__action"}
                  disabled={toolbarAction.disabled || state.busy}
                  onClick={() => {
                    if (shouldOpenActionDialog(nextAction, toolbarAction.values)) {
                      openActionDialog(nextAction, selectedItem, toolbarAction.values);
                      return;
                    }
                    void performAction(nextAction, toolbarAction.values);
                  }}
                >
                  {toolbarAction.label}
                </Button>
              );
            })}
          </div>
        </div>
      ) : null}
      {(hasList || state.output.detail) ? (
        <div className="output-viewtabs" role="tablist" aria-label="output views">
          {hasList ? (
            <button
              type="button"
              className={viewMode === "list" ? "output-viewtabs__item output-viewtabs__item--active" : "output-viewtabs__item"}
              onClick={() => setViewMode("list")}
            >
              列表
            </button>
          ) : null}
          <button
            type="button"
            className={viewMode === "detail" ? "output-viewtabs__item output-viewtabs__item--active" : "output-viewtabs__item"}
            onClick={() => setViewMode("detail")}
            disabled={!hasStructuredDetail && !state.output.detail}
          >
            详情
          </button>
          <button
            type="button"
            className={viewMode === "raw" ? "output-viewtabs__item output-viewtabs__item--active" : "output-viewtabs__item"}
            onClick={() => setViewMode("raw")}
          >
            原始输出
          </button>
        </div>
      ) : null}
      {hasList ? (
        viewMode === "raw" ? (
          <pre className="output-panel__body">{state.output.body}</pre>
        ) : viewMode === "detail" ? (
          sideDetail ? (
            <article className="output-article">
              <header className="output-article__header">
                <div>
                  <h3>{sideDetail.primary}</h3>
                  <p>{sideDetail.secondary ?? "详情视图"}</p>
                </div>
                {sideDetail.badge ? <span className="output-list__badge">{sideDetail.badge}</span> : null}
              </header>
              {sideDetail.fields?.length ? (
                <div className="output-article__meta">
                  {sideDetail.fields.map((field) => (
                    <div key={`${field.label}-${field.value}`} className="output-detail__meta-item">
                      <span>{field.label}</span>
                      {field.href ? (
                        <a href={field.href} target="_blank" rel="noreferrer">
                          {field.value}
                        </a>
                      ) : (
                        <strong>{field.value}</strong>
                      )}
                    </div>
                  ))}
                </div>
              ) : null}
              {sideDetail.body ? <pre className="output-article__body">{sideDetail.body}</pre> : null}
              {sideDetail.url ? (
                <div className="output-detail__actions">
                  <a className="output-detail__action output-detail__action--link" href={sideDetail.url} target="_blank" rel="noreferrer">
                    在浏览器打开
                  </a>
                </div>
              ) : null}
            </article>
          ) : previewingId && selectedItem?.id === previewingId ? (
            <div className="output-empty">加载详情中...</div>
          ) : selectedItem ? (
            <article className="output-article">
              <header className="output-article__header">
                <div>
                  <h3>{selectedItem.primary}</h3>
                  <p>{selectedItem.secondary ?? "当前选中资源"}</p>
                </div>
                {selectedItem.badge ? <span className="output-list__badge">{selectedItem.badge}</span> : null}
              </header>
              <div className="output-article__meta">
                {selectedDetails.map((detail) => (
                  <div key={`${selectedItem.id}-${detail.label}`} className="output-detail__meta-item">
                    <span>{detail.label}</span>
                    {detail.href ? (
                      <a href={detail.href} target="_blank" rel="noreferrer">
                        {detail.value}
                      </a>
                    ) : (
                      <strong>{detail.value}</strong>
                    )}
                  </div>
                ))}
              </div>
              {sourceModule ? (
                <div className="output-detail__actions">
                  {buildRowActions(state.output.actionId, selectedItem).map((rowAction) => {
                    const nextAction = findModuleAction(sourceModule.actions, rowAction.actionId);
                    if (!nextAction) {
                      return null;
                    }
                    return (
                      <button
                        key={`${selectedItem.id}-article-${rowAction.actionId}`}
                        type="button"
                        className="output-detail__action"
                        disabled={rowAction.disabled || state.busy}
                        onClick={() => {
                          if (shouldOpenActionDialog(nextAction, rowAction.values)) {
                            openActionDialog(nextAction, selectedItem, rowAction.values);
                            return;
                          }
                          void performAction(nextAction, rowAction.values);
                        }}
                      >
                        {rowAction.label}
                      </button>
                    );
                  })}
                </div>
              ) : null}
            </article>
          ) : (
            <div className="output-empty">请选择一条资源</div>
          )
        ) : (
        <div className="output-listview">
          <div className="output-toolbar">
            <input
              className="ui-input output-toolbar__search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={tableConfig.searchPlaceholder}
            />
            <select
              className="ui-input ui-select output-toolbar__select"
              value={badgeFilter}
              onChange={(event) => setBadgeFilter(event.target.value)}
              disabled={badgeOptions.length === 0}
            >
              <option value="all">{tableConfig.filterLabel}</option>
              {badgeOptions.map((badge) => (
                <option key={badge} value={badge}>
                  {badge}
                </option>
              ))}
            </select>
            <select className="ui-input ui-select output-toolbar__select" value={sortMode} onChange={(event) => setSortMode(event.target.value as SortMode)}>
              {sortOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <span className="output-toolbar__count">
              {visibleItems.length} / {items.length}
            </span>
          </div>
          {hasFilteredItems ? (
            <div className="output-browser">
              <div
                className="output-table"
                role="table"
                aria-label={state.output.title}
                style={{ ["--output-grid-columns" as string]: tableColumns }}
              >
                <div className="output-table__header" role="row">
                  {tableConfig.columns.map((column) => (
                    <span key={column.key} role="columnheader">
                      {column.label}
                    </span>
                  ))}
                  <span role="columnheader">操作</span>
                </div>
                <div className="output-table__body">
                  {visibleItems.map((item) => {
                    const rowActions = buildRowActions(state.output.actionId, item);
                    const selected = selectedItem?.id === item.id;
                    return (
                      <div
                        key={item.id}
                        className={[
                          "output-table__row",
                          item.active ? "output-table__row--active" : "",
                          selected ? "output-table__row--selected" : "",
                        ]
                          .filter(Boolean)
                          .join(" ")}
                        role="row"
                        onClick={() => setSelectedId(item.id)}
                      >
                        {tableConfig.columns.map((column) => {
                          const value = column.render(item);
                          const isStatus = column.className?.includes("status");
                          const isPrimary = column.className?.includes("primary");
                          return (
                            <div
                              key={`${item.id}-${column.key}`}
                              className={["output-table__cell", column.className ?? ""].filter(Boolean).join(" ")}
                              role="cell"
                            >
                              {isStatus && item.badge ? (
                                <span className="output-list__badge">{value}</span>
                              ) : isPrimary ? (
                                <p>{value}</p>
                              ) : column.key === "url" && item.url ? (
                                <a href={item.url} target="_blank" rel="noreferrer" onClick={(event) => event.stopPropagation()}>
                                  {value}
                                </a>
                              ) : (
                                <span className={value === "-" ? "output-table__empty" : undefined}>{value}</span>
                              )}
                            </div>
                          );
                        })}
                        <div className="output-table__cell output-table__cell--actions" role="cell">
                          {rowActions.length > 0 && sourceModule ? (
                            <div className="output-list__actions">
                              {rowActions.map((rowAction) => {
                                const nextAction = findModuleAction(sourceModule.actions, rowAction.actionId);
                                if (!nextAction) {
                                  return null;
                                }
                                return (
                                  <button
                                    key={`${item.id}-${rowAction.actionId}`}
                                    type="button"
                                    className="output-list__action"
                                    disabled={rowAction.disabled || state.busy}
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      setSelectedId(item.id);
                                      if (shouldOpenActionDialog(nextAction, rowAction.values)) {
                                        openActionDialog(nextAction, item, rowAction.values);
                                        return;
                                      }
                                      void performAction(nextAction, rowAction.values);
                                    }}
                                  >
                                    {rowAction.label}
                                  </button>
                                );
                              })}
                            </div>
                          ) : (
                            <span className="output-table__empty">-</span>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
              <aside className="output-detail">
                {sideDetail ? (
                  <>
                    <header className="output-detail__header">
                      <div>
                        <h3>{sideDetail.primary}</h3>
                        <p>{sideDetail.secondary ?? "详情视图"}</p>
                      </div>
                      {sideDetail.badge ? <span className="output-list__badge">{sideDetail.badge}</span> : null}
                    </header>
                    {sideDetail.fields?.length ? (
                      <div className="output-detail__meta">
                        {sideDetail.fields.map((field) => (
                          <div key={`${field.label}-${field.value}`} className="output-detail__meta-item">
                            <span>{field.label}</span>
                            {field.href ? (
                              <a href={field.href} target="_blank" rel="noreferrer">
                                {field.value}
                              </a>
                            ) : (
                              <strong>{field.value}</strong>
                            )}
                          </div>
                        ))}
                      </div>
                    ) : null}
                    {sideDetail.body ? <pre className="output-detail__body">{sideDetail.body}</pre> : null}
                    {sideDetail.url ? (
                      <div className="output-detail__actions">
                        <a className="output-detail__action output-detail__action--link" href={sideDetail.url} target="_blank" rel="noreferrer">
                          在浏览器打开
                        </a>
                      </div>
                    ) : null}
                  </>
                ) : previewingId && selectedItem?.id === previewingId ? (
                  <div className="output-empty">加载详情中...</div>
                ) : selectedItem ? (
                  <>
                    <header className="output-detail__header">
                      <div>
                        <h3>{selectedItem.primary}</h3>
                        <p>{selectedItem.secondary ?? "当前选中资源"}</p>
                      </div>
                      {selectedItem.badge ? <span className="output-list__badge">{selectedItem.badge}</span> : null}
                    </header>
                    <div className="output-detail__meta">
                      {selectedDetails.map((detail) => (
                        <div key={`${selectedItem.id}-${detail.label}`} className="output-detail__meta-item">
                          <span>{detail.label}</span>
                          {detail.href ? (
                            <a href={detail.href} target="_blank" rel="noreferrer">
                              {detail.value}
                            </a>
                          ) : (
                            <strong>{detail.value}</strong>
                          )}
                        </div>
                      ))}
                    </div>
                    {sourceModule ? (
                      <div className="output-detail__actions">
                        {buildRowActions(state.output.actionId, selectedItem).map((rowAction) => {
                          const nextAction = findModuleAction(sourceModule.actions, rowAction.actionId);
                          if (!nextAction) {
                            return null;
                          }
                          return (
                            <button
                              key={`${selectedItem.id}-detail-${rowAction.actionId}`}
                              type="button"
                              className="output-detail__action"
                              disabled={rowAction.disabled || state.busy}
                              onClick={() => {
                                if (shouldOpenActionDialog(nextAction, rowAction.values)) {
                                  openActionDialog(nextAction, selectedItem, rowAction.values);
                                  return;
                                }
                                void performAction(nextAction, rowAction.values);
                              }}
                            >
                              {rowAction.label}
                            </button>
                          );
                        })}
                      </div>
                    ) : null}
                  </>
                ) : (
                  <div className="output-empty">请选择一条资源</div>
                )}
              </aside>
            </div>
          ) : (
            <div className="output-empty">没有匹配的结果</div>
          )}
        </div>
        )
      ) : state.output.detail ? (
        viewMode === "raw" ? (
          <pre className="output-panel__body">{state.output.body}</pre>
        ) : (
          <article className="output-article">
            <header className="output-article__header">
              <div>
                <h3>{state.output.detail.primary}</h3>
                <p>{state.output.detail.secondary ?? "详情视图"}</p>
              </div>
              {state.output.detail.badge ? <span className="output-list__badge">{state.output.detail.badge}</span> : null}
            </header>
            {state.output.detail.fields?.length ? (
              <div className="output-article__meta">
                {state.output.detail.fields.map((field) => (
                  <div key={`${field.label}-${field.value}`} className="output-detail__meta-item">
                    <span>{field.label}</span>
                    {field.href ? (
                      <a href={field.href} target="_blank" rel="noreferrer">
                        {field.value}
                      </a>
                    ) : (
                      <strong>{field.value}</strong>
                    )}
                  </div>
                ))}
              </div>
            ) : null}
            {state.output.detail.body ? <pre className="output-article__body">{state.output.detail.body}</pre> : null}
            {state.output.detail.url ? (
              <div className="output-detail__actions">
                <a className="output-detail__action output-detail__action--link" href={state.output.detail.url} target="_blank" rel="noreferrer">
                  在浏览器打开
                </a>
              </div>
            ) : null}
          </article>
        )
      ) : state.output.emptyHint ? (
        <div className="output-empty">{state.output.emptyHint}</div>
      ) : (
        <pre className="output-panel__body">{state.output.body}</pre>
      )}
      {dialogAction && sourceModule ? (
        <ActionDialog
          action={dialogAction}
          moduleId={sourceModule.id}
          catalog={state.catalog}
          values={dialogValues}
          busy={state.busy}
          onChange={setDialogValues}
          onClose={closeActionDialog}
          onSubmit={(values) => {
            void performAction(dialogAction, values);
            closeActionDialog();
          }}
        />
      ) : null}
    </section>
  );
}
