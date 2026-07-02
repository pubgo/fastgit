import { Suspense, lazy, useEffect, useMemo, useRef, useState } from "react";
import { Alert, Checkbox, Input as AntInput, Modal, Segmented, Select, Table } from "antd";
import type { TableColumnsType } from "antd";

import { useAppContext } from "../../app/providers/app-context";
import type { ModuleAction, OutputListItem } from "../../app/types";
import { Button } from "../../components/ui/button";
import { buildActionValues } from "../actions/action-fields";
import { isViewAction } from "../actions/action-meta";

const ActionDialog = lazy(async () => {
  const module = await import("../actions/action-dialog");
  return { default: module.ActionDialog };
});

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
  tone?: "default" | "danger";
};
type ToolbarAction = RowAction & {
  variant?: "primary" | "ghost";
  tone?: "default" | "danger";
};
type BatchAction = {
  label: string;
  actionId: string;
  jobs: Array<{ label: string; values: Record<string, string> }>;
  tone?: "default" | "danger";
  requiresReset?: boolean;
  description?: string;
};

type DetailEntry = {
  label: string;
  value: string;
  href?: string;
};

type BadgeTone = "neutral" | "info" | "success" | "warning" | "danger";

const SIMPLE_MODE = true;

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

function getBoolField(item: OutputListItem, key: string): boolean {
  const value = (item.fields?.[key] ?? "").trim().toLowerCase();
  return value === "1" || value === "true" || value === "yes";
}

function editablePushURL(item: OutputListItem): string {
  const fetchURL = getField(item, "fetch", "");
  const pushURL = getField(item, "push", "");
  if (!pushURL || pushURL === "-" || pushURL === fetchURL) {
    return "";
  }
  return pushURL;
}

function isBatchSelectableAction(actionId: string | undefined): boolean {
  return actionId === "remote_list" || actionId === "branch_list" || actionId === "tag_list" || actionId === "worktree_list" || actionId === "issue_list" || actionId === "pr_list" || actionId === "repo_status";
}

function formatFilterValue(actionId: string | undefined, value: string): string {
  if (actionId === "repo_status") {
    switch (value) {
      case "staged":
        return "已暂存";
      case "unstaged":
        return "未暂存";
      case "mixed":
        return "已暂存 + 未暂存";
      case "untracked":
        return "未跟踪";
      case "conflict":
        return "冲突";
      case "clean":
        return "干净";
      default:
        return value;
    }
  }

  if (actionId !== "branch_list") {
    return value;
  }

  switch (value) {
    case "in-sync":
      return "已同步";
    case "ahead":
      return "本地领先";
    case "behind":
      return "落后远端";
    case "diverged":
      return "已分叉";
    case "no-upstream":
      return "未跟踪";
    case "missing-upstream":
      return "远端缺失";
    case "invalid-upstream":
      return "跟踪无效";
    default:
      return value;
  }
}

function pickBadgeTone(actionId: string | undefined, category: string, text: string): BadgeTone {
  const normalizedCategory = category.trim().toLowerCase();
  const normalizedText = text.trim().toLowerCase();
  const combined = `${normalizedCategory} ${normalizedText}`;

  if (actionId === "repo_status") {
    if (normalizedCategory === "conflict" || combined.includes("conflict") || combined.includes("u")) {
      return "danger";
    }
    if (normalizedCategory === "mixed" || normalizedCategory === "unstaged" || normalizedCategory === "untracked") {
      return "warning";
    }
    if (normalizedCategory === "staged") {
      return "info";
    }
    if (normalizedCategory === "clean") {
      return "success";
    }
  }

  if (actionId === "branch_list") {
    switch (normalizedCategory) {
      case "in-sync":
        return "success";
      case "ahead":
      case "behind":
      case "diverged":
      case "no-upstream":
      case "missing-upstream":
        return "warning";
      case "invalid-upstream":
        return "danger";
      default:
        break;
    }
  }

  if (actionId === "issue_list" || actionId === "pr_list" || actionId === "pr_status") {
    if (combined.includes("merged") || combined.includes("closed")) {
      return "success";
    }
    if (combined.includes("draft")) {
      return "warning";
    }
    if (combined.includes("open")) {
      return "info";
    }
  }

  if (combined.includes("error") || combined.includes("failed") || combined.includes("danger")) {
    return "danger";
  }
  if (combined.includes("warn") || combined.includes("behind") || combined.includes("missing")) {
    return "warning";
  }
  if (combined.includes("ok") || combined.includes("success")) {
    return "success";
  }
  if (combined.includes("open") || combined.includes("current")) {
    return "info";
  }
  return "neutral";
}

function badgeClassName(actionId: string | undefined, item: OutputListItem | null, text: string): string {
  const category = item?.fields?.status_category || item?.category || item?.fields?.sync || item?.fields?.state || "";
  const tone = pickBadgeTone(actionId, category, text);
  return `output-list__badge output-list__badge--${tone}`;
}

function getTableConfig(actionId: string | undefined): {
  columns: ColumnDef[];
  searchPlaceholder: string;
  filterLabel: string;
} {
  switch (actionId) {
    case "remote_list":
      return {
        searchPlaceholder: "搜索 remote 名称、协议或 URL...",
        filterLabel: "全部协议",
        columns: [
          {
            key: "name",
            label: "Remote",
            width: "minmax(160px, 0.9fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "name", item.primary),
          },
          {
            key: "transport",
            label: "协议",
            width: "minmax(100px, 0.6fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "transport", item.badge ?? "-"),
          },
          {
            key: "role",
            label: "角色",
            width: "minmax(100px, 0.7fr)",
            render: (item) => getField(item, "status") === "default" ? "默认" : "扩展",
          },
          {
            key: "fetch",
            label: "Fetch URL",
            width: "minmax(280px, 1.7fr)",
            render: (item) => getField(item, "fetch"),
          },
          {
            key: "push",
            label: "Push URL",
            width: "minmax(280px, 1.7fr)",
            render: (item) => getField(item, "push"),
          },
        ],
      };
    case "branch_list":
      return {
        searchPlaceholder: "搜索分支名、upstream、remote 或同步状态...",
        filterLabel: "全部同步状态",
        columns: [
          {
            key: "branch",
            label: "分支",
            width: "minmax(240px, 1.6fr)",
            className: "output-table__cell--primary",
            render: (item) => getField(item, "branch", item.primary),
          },
          {
            key: "upstream",
            label: "Upstream",
            width: "minmax(240px, 1.6fr)",
            render: (item) => getField(item, "upstream"),
          },
          {
            key: "remote",
            label: "Remote",
            width: "minmax(120px, 0.8fr)",
            render: (item) => getField(item, "remote"),
          },
          {
            key: "sync",
            label: "同步状态",
            width: "minmax(140px, 0.9fr)",
            className: "output-table__cell--status",
            render: (item) => getField(item, "sync_label", item.badge ?? "-"),
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
          {
            key: "url",
            label: "链接",
            width: "minmax(240px, 1.4fr)",
            render: (item) => item.url ?? getField(item, "url", "-"),
          },
        ],
      };
    case "pr_list":
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
    case "remote_list":
      return [
        { label: "Remote", value: getField(item, "name", item.value ?? item.primary) },
        { label: "角色", value: getField(item, "status") === "default" ? "默认 remote" : "附加 remote" },
        { label: "协议", value: getField(item, "transport", item.badge ?? "-") },
        { label: "Fetch", value: getField(item, "fetch") },
        { label: "Push", value: getField(item, "push") },
        { label: "状态", value: getField(item, "status", item.active ? "default" : "secondary") },
      ];
    case "branch_list":
      return [
        { label: "分支", value: getField(item, "branch", item.value ?? item.primary) },
        { label: "本地状态", value: getField(item, "status", item.active ? "current" : "local") },
        { label: "Remote", value: getField(item, "remote") },
        { label: "Upstream", value: getField(item, "upstream") },
        { label: "同步状态", value: getField(item, "sync_label", item.badge ?? "-") },
        { label: "Ahead", value: getField(item, "ahead", "0") },
        { label: "Behind", value: getField(item, "behind", "0") },
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
        ...(item.url || getField(item, "url", "") ? [{ label: "链接", value: item.url ?? getField(item, "url"), href: item.url ?? getField(item, "url") }] : []),
      ];
    case "tag_list":
      return [{ label: "Tag", value: getField(item, "tag", item.value ?? item.primary) }];
    case "repo_status":
      return [
        { label: "文件", value: getField(item, "path", item.primary) },
        { label: "状态", value: getField(item, "status", item.badge ?? "-") },
        { label: "类型", value: formatFilterValue("repo_status", getField(item, "status_category", item.category ?? "-")) },
        { label: "Staging", value: getField(item, "staging", "-").replace(/\s/g, "·") },
        { label: "Worktree", value: getField(item, "worktree", "-").replace(/\s/g, "·") },
      ];
    case "pr_status":
    case "pr_list":
      return [
        { label: "PR", value: `#${getField(item, "number", item.value ?? item.primary)}` },
        { label: "标题", value: getField(item, "title", item.primary) },
        { label: "状态", value: getField(item, "state", item.badge ?? "-") },
        { label: "Base", value: getField(item, "base") },
        { label: "Head", value: getField(item, "head") },
        { label: "Draft", value: getField(item, "draft") },
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

function buildRowActions(actionId: string | undefined, item: OutputListItem, defaultRemote = ""): RowAction[] {
  switch (actionId) {
    case "remote_list":
      return [
        { label: "抓取", actionId: "remote_fetch", values: { name: item.value ?? item.primary } },
        {
          label: "编辑",
          actionId: "remote_update",
          values: {
            name: item.value ?? item.primary,
            url: getField(item, "fetch", ""),
            push_url: editablePushURL(item),
          },
        },
        { label: "重命名", actionId: "remote_rename", values: { name: item.value ?? item.primary, new_name: item.value ?? item.primary } },
        { label: "删除", actionId: "remote_remove", values: { name: item.value ?? item.primary }, tone: "danger" },
      ];
    case "branch_list":
      return [
        { label: "对齐远端", actionId: "branch_force_sync", values: { name: item.value ?? item.primary, remote: getField(item, "remote", "") }, tone: "danger" },
        { label: "切换", actionId: "branch_checkout", values: { name: item.value ?? item.primary }, disabled: item.active },
        { label: "删除", actionId: "branch_delete", values: { name: item.value ?? item.primary }, disabled: item.active, tone: "danger" },
      ];
    case "issue_list":
      return [
        { label: "查看", actionId: "issue_view", values: { id: item.value ?? item.primary } },
        { label: "关闭", actionId: "issue_close", values: { id: item.value ?? item.primary }, tone: "danger" },
      ];
    case "tag_list":
      return [
        { label: "推送", actionId: "tag_push", values: { name: item.value ?? item.primary, remote: defaultRemote } },
        { label: "对齐远端", actionId: "tag_force_sync", values: { name: item.value ?? item.primary, remote: defaultRemote }, tone: "danger" },
      ];
    case "worktree_list":
      return [{ label: "删除", actionId: "worktree_remove", values: { target: item.value ?? item.primary }, disabled: item.active }];
    case "pr_status":
      return [
        { label: "同步", actionId: "pr_sync", values: {} },
        { label: "合并", actionId: "pr_merge", values: { method: "squash" } },
      ];
    case "pr_list":
      return [
        { label: "查看", actionId: "pr_view", values: { id: item.value ?? item.primary } },
        { label: "关闭", actionId: "pr_close", values: { id: item.value ?? item.primary }, tone: "danger" },
      ];
    case "repo_status":
      return [
        {
          label: "暂存",
          actionId: "repo_stage_path",
          values: { path: getField(item, "path", item.value ?? item.primary) },
          disabled: !getBoolField(item, "can_stage"),
        },
        {
          label: "撤销暂存",
          actionId: "repo_unstage_path",
          values: { path: getField(item, "path", item.value ?? item.primary) },
          disabled: !getBoolField(item, "can_unstage"),
        },
        {
          label: "丢弃",
          actionId: "repo_discard_path",
          values: { path: getField(item, "path", item.value ?? item.primary) },
          disabled: !getBoolField(item, "can_discard"),
          tone: "danger",
        },
      ];
    default:
      return [];
  }
}

function buildToolbarActions(actionId: string | undefined, item: OutputListItem | null, defaultRemote = ""): ToolbarAction[] {
  switch (actionId) {
    case "remote_list":
      return [
        { label: "刷新列表", actionId: "remote_list", values: {}, variant: "ghost" },
        { label: "抓取全部", actionId: "remote_fetch_all", values: {}, variant: "ghost" },
        { label: "添加 Remote", actionId: "remote_add", values: {}, variant: "primary" },
        ...(item
          ? [
              { label: "抓取 Remote", actionId: "remote_fetch", values: { name: item.value ?? item.primary }, variant: "ghost" as const },
              {
                label: "编辑 Remote",
                actionId: "remote_update",
                values: {
                  name: item.value ?? item.primary,
                  url: getField(item, "fetch", ""),
                  push_url: editablePushURL(item),
                },
                variant: "ghost" as const,
              },
              { label: "重命名", actionId: "remote_rename", values: { name: item.value ?? item.primary, new_name: item.value ?? item.primary }, variant: "ghost" as const },
              { label: "删除 Remote", actionId: "remote_remove", values: { name: item.value ?? item.primary }, variant: "ghost" as const, tone: "danger" as const },
            ]
          : []),
      ];
    case "branch_list":
      return [
        { label: "刷新列表", actionId: "branch_list", values: {}, variant: "ghost" },
        { label: "新建分支", actionId: "branch_create", values: {}, variant: "primary" },
        ...(item
          ? [
              { label: "强制对齐远端", actionId: "branch_force_sync", values: { name: item.value ?? item.primary, remote: getField(item, "remote", "") }, variant: "ghost" as const, tone: "danger" as const },
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
        ...(item
          ? [
              { label: "查看详情", actionId: "issue_view", values: { id: item.value ?? item.primary }, variant: "ghost" as const },
              { label: "关闭 Issue", actionId: "issue_close", values: { id: item.value ?? item.primary }, variant: "ghost" as const, tone: "danger" as const },
            ]
          : []),
      ];
    case "pr_list":
      return [
        { label: "刷新列表", actionId: "pr_list", values: {}, variant: "ghost" },
        { label: "创建 PR", actionId: "pr_create", values: {}, variant: "primary" },
        ...(item
          ? [
              { label: "查看 PR", actionId: "pr_view", values: { id: item.value ?? item.primary }, variant: "ghost" as const },
              { label: "关闭 PR", actionId: "pr_close", values: { id: item.value ?? item.primary }, variant: "ghost" as const, tone: "danger" as const },
            ]
          : []),
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
        ...(item
          ? [
              { label: "推送 Tag", actionId: "tag_push", values: { name: item.value ?? item.primary, remote: defaultRemote }, variant: "ghost" as const },
              { label: "强制对齐远端", actionId: "tag_force_sync", values: { name: item.value ?? item.primary, remote: defaultRemote }, variant: "ghost" as const, tone: "danger" as const },
            ]
          : []),
      ];
    case "repo_status":
      return [
        { label: "刷新状态", actionId: "repo_status", values: {}, variant: "ghost" },
        { label: "拉取", actionId: "repo_pull", values: { remote: defaultRemote }, variant: "primary" },
        { label: "推送", actionId: "repo_push", values: { remote: defaultRemote }, variant: "ghost" },
        ...(item
          ? [
              { label: "暂存文件", actionId: "repo_stage_path", values: { path: getField(item, "path", item.value ?? item.primary) }, variant: "ghost" as const, disabled: !getBoolField(item, "can_stage") },
              { label: "撤销暂存", actionId: "repo_unstage_path", values: { path: getField(item, "path", item.value ?? item.primary) }, variant: "ghost" as const, disabled: !getBoolField(item, "can_unstage") },
              { label: "丢弃文件", actionId: "repo_discard_path", values: { path: getField(item, "path", item.value ?? item.primary) }, variant: "ghost" as const, disabled: !getBoolField(item, "can_discard"), tone: "danger" as const },
            ]
          : []),
        { label: "强制对齐当前分支", actionId: "repo_force_sync", values: { remote: defaultRemote }, variant: "ghost", tone: "danger" },
      ];
    default:
      return [];
  }
}

function buildBatchActions(actionId: string | undefined, items: OutputListItem[], defaultRemote = ""): BatchAction[] {
  switch (actionId) {
    case "remote_list":
      return [
        {
          label: `批量抓取 (${items.length})`,
          actionId: "remote_fetch",
          jobs: items.map((item) => ({
            label: item.primary,
            values: { name: item.value ?? item.primary },
          })),
        },
        {
          label: `批量删除 (${items.length})`,
          actionId: "remote_remove",
          jobs: items.map((item) => ({
            label: item.primary,
            values: { name: item.value ?? item.primary },
          })),
          tone: "danger",
          description: "会移除选中的 remote 配置。",
        },
      ];
    case "branch_list": {
      const deletable = items.filter((item) => !item.active);
      const syncable = items.filter((item) => Boolean(getField(item, "remote", "")));
      return [
        {
          label: `批量删除 (${deletable.length})`,
          actionId: "branch_delete",
          jobs: deletable.map((item) => ({
            label: item.primary,
            values: { name: item.value ?? item.primary },
          })),
          tone: "danger",
          description: "会删除选中的本地分支，当前分支会自动跳过。",
        },
        {
          label: `批量对齐远端 (${syncable.length})`,
          actionId: "branch_force_sync",
          jobs: syncable.map((item) => ({
            label: item.primary,
            values: { name: item.value ?? item.primary, remote: getField(item, "remote", "") },
          })),
          tone: "danger",
          requiresReset: true,
          description: "会丢弃分支本地改动并强制对齐远端 tracking 分支。",
        },
      ];
    }
    case "tag_list":
      return [
        {
          label: `批量推送 (${items.length})`,
          actionId: "tag_push",
          jobs: items.map((item) => ({
            label: item.primary,
            values: { name: item.value ?? item.primary, remote: defaultRemote },
          })),
        },
        {
          label: `批量对齐远端 (${items.length})`,
          actionId: "tag_force_sync",
          jobs: items.map((item) => ({
            label: item.primary,
            values: { name: item.value ?? item.primary, remote: defaultRemote },
          })),
          tone: "danger",
          requiresReset: true,
          description: "会用远端同名 tag 覆盖本地 tag。",
        },
      ];
    case "worktree_list": {
      const removable = items.filter((item) => !item.active);
      return [
        {
          label: `批量删除 (${removable.length})`,
          actionId: "worktree_remove",
          jobs: removable.map((item) => ({
            label: item.fields?.path ?? item.primary,
            values: { target: item.value ?? item.primary },
          })),
          tone: "danger",
          description: "会删除选中的 worktree，当前项目 worktree 会自动跳过。",
        },
      ];
    }
    case "issue_list":
      return [
        {
          label: `批量关闭 (${items.length})`,
          actionId: "issue_close",
          jobs: items.map((item) => ({
            label: item.primary,
            values: { id: item.value ?? item.primary },
          })),
          tone: "danger",
          description: "会关闭选中的 issue。",
        },
      ];
    case "pr_list":
      return [
        {
          label: `批量关闭 (${items.length})`,
          actionId: "pr_close",
          jobs: items.map((item) => ({
            label: item.primary,
            values: { id: item.value ?? item.primary },
          })),
          tone: "danger",
          description: "会关闭选中的 PR。",
        },
      ];
    case "repo_status": {
      const stageable = items.filter((item) => getBoolField(item, "can_stage"));
      const unstageable = items.filter((item) => getBoolField(item, "can_unstage"));
      const discardable = items.filter((item) => getBoolField(item, "can_discard"));
      return [
        {
          label: `批量暂存 (${stageable.length})`,
          actionId: "repo_stage_path",
          jobs: stageable.map((item) => ({
            label: item.primary,
            values: { path: getField(item, "path", item.value ?? item.primary) },
          })),
        },
        {
          label: `批量撤销暂存 (${unstageable.length})`,
          actionId: "repo_unstage_path",
          jobs: unstageable.map((item) => ({
            label: item.primary,
            values: { path: getField(item, "path", item.value ?? item.primary) },
          })),
        },
        {
          label: `批量丢弃 (${discardable.length})`,
          actionId: "repo_discard_path",
          jobs: discardable.map((item) => ({
            label: item.primary,
            values: { path: getField(item, "path", item.value ?? item.primary) },
          })),
          tone: "danger",
          requiresReset: true,
          description: "会丢弃选中文件本地改动；未跟踪文件会直接删除。",
        },
      ];
    }
    default:
      return [];
  }
}

function getDefaultSortMode(actionId: string | undefined): SortMode {
  switch (actionId) {
    case "remote_list":
      return "name-asc";
    case "branch_list":
    case "worktree_list":
      return "active-first";
    case "issue_list":
    case "pr_list":
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
    case "remote_list":
      return [
        { value: "name-asc", label: "名称 A-Z" },
        { value: "name-desc", label: "名称 Z-A" },
        { value: "status", label: "协议分组" },
      ];
    case "issue_list":
    case "pr_list":
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
  if (
    action.id.startsWith("pr_") ||
    action.id.endsWith("_force_sync") ||
    action.id.endsWith("_rename") ||
    action.id.endsWith("_update") ||
    action.id.endsWith("_delete") ||
    action.id.endsWith("_remove") ||
    action.id.endsWith("_close") ||
    action.id.endsWith("_discard")
  ) {
    return true;
  }
  return fields.some((field) => field.required && !values[field.key]);
}

function detailMatchesItem(detailTargetId: string | undefined, item: OutputListItem | null): boolean {
  if (!detailTargetId || !item) {
    return false;
  }
  return detailTargetId === item.id || detailTargetId === item.value;
}

function buildDefaultPRBody(branch: string, base: string): string {
  return [
    "## Summary",
    `- branch: ${branch || "-"}`,
    `- base: ${base || "-"}`,
    "",
    "## Changes",
    "- ",
    "",
    "## Verification",
    "- ",
  ].join("\n");
}

export function OutputPanel() {
  const { state, runAction, runBatchActions, runActionAndReload, previewAction, updateProjectSettings } = useAppContext();
  const items = state.output.items ?? [];
  const hasList = items.length > 0;
  const isManageView = Boolean(state.output.actionId && (state.output.actionId.endsWith("_list") || state.output.actionId.endsWith("_status")));
  const defaultBaseBranch = state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "";
  const projectDefaultRemote = state.projectSettings[state.repoPath]?.defaultRemote ?? "";
  const defaultRemote =
    projectDefaultRemote || state.catalog.remotes.find((item) => item.active)?.value || state.catalog.remotes[0]?.value || "origin";
  const [query, setQuery] = useState("");
  const [badgeFilter, setBadgeFilter] = useState("all");
  const [sortMode, setSortMode] = useState<SortMode>("default");
  const [viewMode, setViewMode] = useState<ViewMode>("raw");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [previewingId, setPreviewingId] = useState<string | null>(null);
  const [dialogAction, setDialogAction] = useState<ModuleAction | null>(null);
  const [dialogValues, setDialogValues] = useState<Record<string, string>>({});
  const [batchAction, setBatchAction] = useState<BatchAction | null>(null);
  const [batchConfirm, setBatchConfirm] = useState("");
  const lastPreviewKeyRef = useRef<string>("");

  useEffect(() => {
    setQuery("");
    setBadgeFilter("all");
    setSortMode(getDefaultSortMode(state.output.actionId));
    setViewMode(items.length > 0 ? "list" : state.output.detail ? "detail" : "raw");
    setSelectedId(null);
    setSelectedIds([]);
    setPreviewingId(null);
    setDialogAction(null);
    setDialogValues({});
    setBatchAction(null);
    setBatchConfirm("");
    lastPreviewKeyRef.current = "";
  }, [items.length, state.output.actionId, state.output.command, state.output.title]);

  const badgeOptions = useMemo(
    () =>
      Array.from(new Set(items.map((item) => item.category ?? item.badge).filter((badge): badge is string => Boolean(badge)))),
    [items]
  );
  const sortOptions = useMemo(() => getSortOptions(state.output.actionId), [state.output.actionId]);
  const tableConfig = useMemo(() => getTableConfig(state.output.actionId), [state.output.actionId]);

  const visibleItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const filtered = items.filter((item) => {
      if (badgeFilter !== "all" && (item.category ?? item.badge) !== badgeFilter) {
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
        const leftStatus = left.category ?? left.badge ?? "";
        const rightStatus = right.category ?? right.badge ?? "";
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
  const sourceModule = useMemo(() => {
    const moduleFromOutput = state.output.moduleId ? state.modules.find((item) => item.id === state.output.moduleId) ?? null : null;
    if (moduleFromOutput) {
      return moduleFromOutput;
    }
    if (!isManageView || !state.selectedModuleId) {
      return null;
    }
    return state.modules.find((item) => item.id === state.selectedModuleId) ?? null;
  }, [isManageView, state.modules, state.output.moduleId, state.selectedModuleId]);
  const showOutputHeader = !isManageView || !sourceModule;
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectedBatchItems = useMemo(() => items.filter((item) => selectedSet.has(item.id)), [items, selectedSet]);
  const supportsBatchSelection = isBatchSelectableAction(state.output.actionId);
  const bulkActions = useMemo(
    () => (supportsBatchSelection ? buildBatchActions(state.output.actionId, selectedBatchItems, defaultRemote) : []),
    [defaultRemote, selectedBatchItems, state.output.actionId, supportsBatchSelection]
  );
  const allVisibleSelected = hasFilteredItems && visibleItems.every((item) => selectedSet.has(item.id));
  const selectedItem = visibleItems.find((item) => item.id === selectedId) ?? visibleItems[0] ?? null;
  const selectedDetails = selectedItem ? describeSelectedItem(state.output.actionId, selectedItem) : [];
  const toolbarActions = sourceModule ? buildToolbarActions(state.output.actionId, selectedItem, defaultRemote) : [];
  const visibleToolbarActions = useMemo(() => {
    if (!SIMPLE_MODE || !isManageView) {
      return toolbarActions;
    }

    const picked = new Map<string, ToolbarAction>();
    const add = (action: ToolbarAction | undefined) => {
      if (!action) {
        return;
      }
      picked.set(action.actionId, action);
    };

    add(toolbarActions.find((action) => action.actionId === state.output.actionId));
    toolbarActions.filter((action) => action.variant === "primary").forEach(add);
    if (state.output.actionId === "repo_status") {
      add(toolbarActions.find((action) => action.actionId === "repo_pull"));
      add(toolbarActions.find((action) => action.actionId === "repo_push"));
    }
    if (state.output.actionId === "branch_list") {
      add(toolbarActions.find((action) => action.actionId === "branch_create"));
      add(toolbarActions.find((action) => action.actionId === "branch_force_sync"));
    }
    if (state.output.actionId === "tag_list") {
      add(toolbarActions.find((action) => action.actionId === "tag_publish"));
      add(toolbarActions.find((action) => action.actionId === "tag_force_sync"));
    }

    return Array.from(picked.values()).slice(0, 4);
  }, [isManageView, state.output.actionId, toolbarActions]);
  const sideDetail =
    state.output.detail && (!state.output.detail.targetId || detailMatchesItem(state.output.detail.targetId, selectedItem))
      ? state.output.detail
      : null;
  const selectedRemoteName = state.output.actionId === "remote_list" ? selectedItem?.value ?? selectedItem?.primary ?? "" : "";
  const selectedRemoteIsDefault = Boolean(selectedRemoteName && projectDefaultRemote && selectedRemoteName === projectDefaultRemote);
  const currentBranch = state.catalog.branches.find((item) => item.active) ?? null;
  const contextMetaText =
    state.output.actionId === "repo_status"
      ? selectedItem
        ? `当前选中文件: ${selectedItem.primary}`
        : `默认 remote: ${projectDefaultRemote || defaultRemote || "-"}`
      : state.output.actionId === "branch_list"
        ? selectedItem
          ? `当前选中分支: ${selectedItem.primary}`
          : state.output.emptyHint ?? "暂无分支"
        : state.output.actionId === "tag_list"
          ? selectedItem
            ? `当前选中 tag: ${selectedItem.primary}`
            : state.output.emptyHint ?? "暂无标签"
          : state.output.actionId === "worktree_list"
            ? selectedItem
              ? `当前选中 worktree: ${selectedItem.fields?.path ?? selectedItem.primary}`
              : state.output.emptyHint ?? "暂无 worktree"
            : state.output.actionId === "issue_list"
              ? selectedItem
                ? `当前选中 issue: #${selectedItem.fields?.number ?? selectedItem.value ?? "-"}`
                : state.output.emptyHint ?? "暂无 issue"
              : state.output.actionId === "pr_list"
                ? selectedItem
                  ? `当前选中 PR: #${selectedItem.fields?.number ?? selectedItem.value ?? "-"}`
                  : state.output.emptyHint ?? "暂无 PR"
              : state.output.actionId === "pr_status"
                ? selectedItem
                  ? `当前分支 PR: #${selectedItem.fields?.number ?? selectedItem.value ?? "-"}`
                  : state.output.emptyHint ?? "当前分支暂无 PR"
                : selectedItem
                  ? `当前选中: ${selectedItem.primary}`
                  : state.output.emptyHint ?? "暂无资源";
  const manageSummaryCards =
    state.output.actionId === "remote_list"
      ? [
          { label: "Remote 总数", value: `${items.length}` },
          { label: "项目默认 Remote", value: projectDefaultRemote || "未设置" },
          { label: "当前选中", value: selectedRemoteName || "未选择" },
          {
            label: "角色",
            value: selectedItem ? (getField(selectedItem, "status") === "default" ? "默认 remote" : "附加 remote") : "请选择一条 remote",
          },
          { label: "Fetch URL", value: selectedItem ? getField(selectedItem, "fetch") : "请选择一条 remote" },
          { label: "Push URL", value: selectedItem ? getField(selectedItem, "push") : "请选择一条 remote" },
        ]
      : state.output.actionId === "branch_list"
        ? [
            { label: "分支总数", value: `${items.length}` },
            { label: "当前分支", value: currentBranch?.primary || "-" },
            { label: "项目默认 Remote", value: projectDefaultRemote || defaultRemote || "-" },
            { label: "当前 Upstream", value: currentBranch?.fields?.upstream || "未配置 upstream" },
            { label: "选中分支", value: selectedItem?.primary || "未选择" },
            { label: "同步状态", value: selectedItem?.fields?.sync_label || currentBranch?.fields?.sync_label || "未知" },
          ]
        : state.output.actionId === "tag_list"
          ? [
              { label: "Tag 总数", value: `${items.length}` },
              { label: "项目默认 Remote", value: projectDefaultRemote || defaultRemote || "-" },
              { label: "当前选中", value: selectedItem?.primary || "未选择" },
              { label: "推送目标", value: defaultRemote || "未设置 remote" },
            ]
          : state.output.actionId === "worktree_list"
            ? [
                { label: "Worktree 总数", value: `${items.length}` },
                { label: "当前项目路径", value: state.repoPath || "-" },
                { label: "当前选中", value: selectedItem?.fields?.path || "未选择" },
                { label: "目标分支", value: selectedItem?.fields?.branch || "-" },
              ]
          : state.output.actionId === "issue_list"
            ? [
                { label: "Issue 总数", value: `${items.length}` },
                { label: "当前选中", value: selectedItem?.primary || "未选择" },
                { label: "Issue 状态", value: selectedItem?.fields?.state || "open" },
                { label: "详情载入", value: previewingId ? "读取中" : sideDetail ? "已同步" : "待选择" },
              ]
          : state.output.actionId === "pr_list"
            ? [
                { label: "PR 总数", value: `${items.length}` },
                { label: "当前选中", value: selectedItem?.primary || "未选择" },
                { label: "Base", value: selectedItem?.fields?.base || "-" },
                { label: "Head", value: selectedItem?.fields?.head || "-" },
                { label: "Draft", value: selectedItem?.fields?.draft || "-" },
                { label: "状态", value: selectedItem?.fields?.state || "unknown" },
              ]
          : state.output.actionId === "pr_status"
            ? [
                { label: "当前分支", value: currentBranch?.primary || "-" },
                { label: "PR 编号", value: selectedItem?.fields?.number ? `#${selectedItem.fields.number}` : "暂无 PR" },
                { label: "Base", value: selectedItem?.fields?.base || "-" },
                { label: "Head", value: selectedItem?.fields?.head || currentBranch?.primary || "-" },
                { label: "Draft", value: selectedItem?.fields?.draft || "-" },
                { label: "状态", value: selectedItem?.fields?.state || "unknown" },
              ]
      : state.output.actionId === "repo_status"
        ? [
            { label: "文件总数", value: `${items.length}` },
            { label: "已暂存", value: `${items.filter((item) => item.category === "staged" || item.category === "mixed").length}` },
            { label: "未暂存", value: `${items.filter((item) => item.category === "unstaged" || item.category === "mixed").length}` },
            { label: "未跟踪", value: `${items.filter((item) => item.category === "untracked").length}` },
            { label: "当前分支", value: currentBranch?.primary || "-" },
            { label: "项目默认 Remote", value: projectDefaultRemote || defaultRemote || "-" },
            { label: "Upstream", value: currentBranch?.fields?.upstream || "未配置 upstream" },
            { label: "同步状态", value: currentBranch?.fields?.sync_label || "未知" },
          ]
        : [];

  useEffect(() => {
    if (!selectedItem || !sourceModule) {
      return;
    }

    if (state.output.actionId === "issue_list") {
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
      return;
    }

    if (state.output.actionId === "pr_list") {
      const prID = selectedItem.value ?? selectedItem.primary;
      const previewKey = `pr:${prID}`;
      if (lastPreviewKeyRef.current === previewKey && (sideDetail?.targetId === `pr-${prID}` || sideDetail?.targetId === prID)) {
        return;
      }

      lastPreviewKeyRef.current = previewKey;
      setPreviewingId(selectedItem.id);
      void previewAction(sourceModule.id, "pr_view", { id: prID }).finally(() => {
        setPreviewingId((current) => (current === selectedItem.id ? null : current));
      });
    }
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
    const branchName = currentBranch?.primary || "";
    if (item && action.id === "pr_sync") {
      if (!seedValues.title) {
        seedValues.title = getField(item, "title", item.primary);
      }
      if (!seedValues.body && sideDetail?.body && detailMatchesItem(sideDetail.targetId, item)) {
        seedValues.body = sideDetail.body;
      }
    }
    if (action.id === "pr_create") {
      if (!seedValues.title && branchName) {
        seedValues.title = `Update ${branchName}`;
      }
      if (!seedValues.body) {
        seedValues.body = buildDefaultPRBody(branchName, defaultBaseBranch || "main");
      }
    }
    if (action.id === "pr_sync") {
      if (!seedValues.title && branchName) {
        seedValues.title = `Update ${branchName}`;
      }
      if (!seedValues.body) {
        seedValues.body = buildDefaultPRBody(branchName, defaultBaseBranch || "main");
      }
    }

    setDialogAction(action);
    setDialogValues(buildActionValues(action, seedValues, { base: defaultBaseBranch, remote: defaultRemote }));
  };

  const closeActionDialog = () => {
    setDialogAction(null);
    setDialogValues({});
  };

  const toggleAllVisibleSelection = () => {
    setSelectedIds((current) => {
      const currentSet = new Set(current);
      if (allVisibleSelected) {
        return current.filter((id) => !visibleItems.some((item) => item.id === id));
      }
      visibleItems.forEach((item) => currentSet.add(item.id));
      return Array.from(currentSet);
    });
  };

  const closeBatchDialog = () => {
    setBatchAction(null);
    setBatchConfirm("");
  };

  const executeBatchAction = async (nextBatchAction: BatchAction, confirmValue = "") => {
    if (!sourceModule) {
      return;
    }
    const nextAction = findModuleAction(sourceModule.actions, nextBatchAction.actionId);
    if (!nextAction || nextBatchAction.jobs.length === 0) {
      return;
    }

    const jobs = nextBatchAction.jobs.map((job) => ({
      label: job.label,
      values: nextBatchAction.requiresReset ? { ...job.values, confirm: confirmValue } : job.values,
    }));
    await runBatchActions(sourceModule, nextAction, jobs, state.output.actionId);
    setSelectedIds([]);
  };

  const hasStructuredDetail = Boolean(sideDetail || selectedItem || previewingId);
  const viewOptions = [
    ...(hasList ? [{ label: "列表", value: "list" as ViewMode }] : []),
    { label: "详情", value: "detail" as ViewMode, disabled: !hasStructuredDetail && !state.output.detail },
    ...(!SIMPLE_MODE || !isManageView ? [{ label: "原始输出", value: "raw" as ViewMode }] : []),
  ];
  const filterOptions = [
    { label: tableConfig.filterLabel, value: "all" },
    ...badgeOptions.map((badge) => ({
      label: formatFilterValue(state.output.actionId, badge),
      value: badge,
    })),
  ];
  const sortSelectOptions = sortOptions.map((option) => ({
    label: option.label,
    value: option.value,
  }));
  const actionColumnWidth = state.output.actionId === "repo_status" ? 320 : 220;
  const tableClassName = `output-ant-table output-ant-table--${state.output.actionId ?? "generic"}`;
  const antTableColumns: TableColumnsType<OutputListItem> = [
    ...tableConfig.columns.map((column) => ({
      title: column.label,
      key: column.key,
      ellipsis: true,
      render: (_: unknown, item: OutputListItem) => {
        const value = column.render(item);
        const isStatus = column.className?.includes("status");
        const isPrimary = column.className?.includes("primary");
        if (isStatus) {
          return <span className={badgeClassName(state.output.actionId, item, value)}>{value}</span>;
        }
        if (isPrimary) {
          return <p className="output-ant-table__primary">{value}</p>;
        }
        if (column.key === "url" && item.url) {
          return (
            <a href={item.url} target="_blank" rel="noreferrer" onClick={(event) => event.stopPropagation()}>
              {value}
            </a>
          );
        }
        return <span className={value === "-" ? "output-table__empty" : undefined}>{value}</span>;
      },
    })),
    {
      title: "操作",
      key: "actions",
      width: actionColumnWidth,
      render: (_: unknown, item: OutputListItem) => {
        const rowActions = buildRowActions(state.output.actionId, item, defaultRemote);
        if (!(rowActions.length > 0 && sourceModule)) {
          return <span className="output-table__empty">-</span>;
        }
        const rowActionsClassName =
          state.output.actionId === "repo_status"
            ? "output-list__actions output-list__actions--repo"
            : "output-list__actions";
        return (
          <div className={rowActionsClassName}>
            {rowActions.map((rowAction) => {
              const nextAction = findModuleAction(sourceModule.actions, rowAction.actionId);
              if (!nextAction) {
                return null;
              }
              return (
                <Button
                  key={`${item.id}-${rowAction.actionId}`}
                  variant="ghost"
                  className={rowAction.tone === "danger" ? "output-list__action output-list__action--danger" : "output-list__action"}
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
                </Button>
              );
            })}
          </div>
        );
      },
    },
  ];
  const tableRowSelection = supportsBatchSelection
    ? {
        selectedRowKeys: selectedIds,
        preserveSelectedRowKeys: true,
        onChange: (keys: Array<string | number>) => setSelectedIds(keys.map((key) => String(key))),
      }
    : undefined;

  return (
    <section className="output-panel min-h-0 overflow-hidden">
      {showOutputHeader ? (
        <header className="output-panel__header">
          <div>
            <h2>{state.output.title}</h2>
            <p>{state.output.command}</p>
          </div>
          <span className={state.output.exitCode === 0 ? "status-pill status-pill--ok" : "status-pill status-pill--error"}>
            exit {state.output.exitCode}
          </span>
        </header>
      ) : null}
      {isManageView && sourceModule && visibleToolbarActions.length > 0 ? (
        <div className="output-contextbar">
          <div className="output-contextbar__meta">
            <strong>{state.output.title}</strong>
            <span>{contextMetaText}</span>
          </div>
          <div className="output-contextbar__actions">
            {state.output.actionId === "remote_list" && selectedRemoteName ? (
              <Button
                variant={selectedRemoteIsDefault ? "primary" : "ghost"}
                disabled={state.busy}
                onClick={() => updateProjectSettings({ defaultRemote: selectedRemoteName })}
              >
                {selectedRemoteIsDefault ? "项目默认 Remote" : "设为项目默认"}
              </Button>
            ) : null}
            {visibleToolbarActions.map((toolbarAction) => {
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
      {!SIMPLE_MODE && manageSummaryCards.length > 0 ? (
        <div className="output-summary-grid">
          {manageSummaryCards.map((card) => (
            <article key={`${state.output.actionId}-${card.label}`} className="output-summary-card">
              <span>{card.label}</span>
              <strong>{card.value}</strong>
            </article>
          ))}
        </div>
      ) : null}
      {(hasList || state.output.detail) ? (
        <Segmented<ViewMode> className="output-viewtabs" value={viewMode} options={viewOptions} onChange={(next) => setViewMode(next)} />
      ) : null}
      <div className="output-content">
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
                {sideDetail.badge ? <span className={badgeClassName(state.output.actionId, selectedItem, sideDetail.badge)}>{sideDetail.badge}</span> : null}
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
                {selectedItem.badge ? <span className={badgeClassName(state.output.actionId, selectedItem, selectedItem.badge)}>{selectedItem.badge}</span> : null}
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
                  {buildRowActions(state.output.actionId, selectedItem, defaultRemote).map((rowAction) => {
                    const nextAction = findModuleAction(sourceModule.actions, rowAction.actionId);
                    if (!nextAction) {
                      return null;
                    }
                    return (
                      <Button
                        key={`${selectedItem.id}-article-${rowAction.actionId}`}
                        type="button"
                        variant="ghost"
                        className={rowAction.tone === "danger" ? "output-detail__action output-detail__action--danger" : "output-detail__action"}
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
                      </Button>
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
          {supportsBatchSelection && selectedBatchItems.length > 0 ? (
            <div className="output-selectionbar">
              <div className="output-selectionbar__meta">
                <label className="output-selectionbar__toggle" htmlFor="output-select-all-toggle">
                  <Checkbox id="output-select-all-toggle" checked={allVisibleSelected} onChange={() => toggleAllVisibleSelection()} disabled={!hasFilteredItems} />
                  <span>本页全选</span>
                </label>
                <span className="output-selectionbar__count">已选 {selectedBatchItems.length} 项</span>
                {selectedBatchItems.length > 0 ? (
                  <button type="button" className="output-selectionbar__clear" onClick={() => setSelectedIds([])}>
                    清空选择
                  </button>
                ) : null}
              </div>
              <div className="output-selectionbar__actions">
                {bulkActions.map((item) => {
                  const disabled = item.jobs.length === 0 || state.busy;
                  return (
                    <Button
                      key={`batch-${item.actionId}`}
                      variant="ghost"
                      className={item.tone === "danger" ? "output-toolbar__action output-toolbar__action--danger" : "output-toolbar__action"}
                      disabled={disabled}
                      onClick={() => {
                        if (item.tone === "danger" || item.requiresReset) {
                          setBatchAction(item);
                          setBatchConfirm("");
                          return;
                        }
                        void executeBatchAction(item);
                      }}
                    >
                      {item.label}
                    </Button>
                  );
                })}
              </div>
            </div>
          ) : null}
          <div className="output-toolbar">
            <AntInput
              className="output-toolbar__search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={tableConfig.searchPlaceholder}
            />
            <Select
              className="output-toolbar__select"
              value={badgeFilter}
              onChange={(next) => setBadgeFilter(next)}
              disabled={badgeOptions.length === 0}
              options={filterOptions}
            />
            <Select
              className="output-toolbar__select"
              value={sortMode}
              onChange={(next) => setSortMode(next as SortMode)}
              options={sortSelectOptions}
            />
            <span className="output-toolbar__count">
              {visibleItems.length} / {items.length}
            </span>
          </div>
          {hasFilteredItems ? (
            <div className="output-browser">
              <div className="output-table-wrap">
                <Table<OutputListItem>
                  className={tableClassName}
                  size="small"
                  rowKey={(item) => item.id}
                  columns={antTableColumns}
                  dataSource={visibleItems}
                  pagination={false}
                  scroll={{ x: "max-content", y: 420 }}
                  rowSelection={tableRowSelection}
                  rowClassName={(item) =>
                    [
                      "output-table-row",
                      item.active ? "output-table-row--active" : "",
                      selectedItem?.id === item.id ? "output-table-row--selected" : "",
                    ]
                      .filter(Boolean)
                      .join(" ")
                  }
                  onRow={(item) => ({
                    onClick: () => setSelectedId(item.id),
                  })}
                />
              </div>
              <aside className="output-detail">
                {sideDetail ? (
                  <>
                    <header className="output-detail__header">
                      <div>
                        <h3>{sideDetail.primary}</h3>
                        <p>{sideDetail.secondary ?? "详情视图"}</p>
                      </div>
                      {sideDetail.badge ? <span className={badgeClassName(state.output.actionId, selectedItem, sideDetail.badge)}>{sideDetail.badge}</span> : null}
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
                      {selectedItem.badge ? <span className={badgeClassName(state.output.actionId, selectedItem, selectedItem.badge)}>{selectedItem.badge}</span> : null}
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
                        {buildRowActions(state.output.actionId, selectedItem, defaultRemote).map((rowAction) => {
                          const nextAction = findModuleAction(sourceModule.actions, rowAction.actionId);
                          if (!nextAction) {
                            return null;
                          }
                          return (
                            <Button
                              key={`${selectedItem.id}-detail-${rowAction.actionId}`}
                              type="button"
                              variant="ghost"
                              className={rowAction.tone === "danger" ? "output-detail__action output-detail__action--danger" : "output-detail__action"}
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
                            </Button>
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
              {state.output.detail.badge ? <span className={badgeClassName(state.output.actionId, null, state.output.detail.badge)}>{state.output.detail.badge}</span> : null}
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
      </div>
      {dialogAction && sourceModule ? (
        <Suspense fallback={null}>
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
        </Suspense>
      ) : null}
      {batchAction ? (
        <Modal
          open
          title={batchAction.label}
          onCancel={closeBatchDialog}
          onOk={() => {
            void executeBatchAction(batchAction, batchConfirm.trim());
            closeBatchDialog();
          }}
          okText="执行批量操作"
          cancelText="取消"
          confirmLoading={state.busy}
          okButtonProps={{
            danger: batchAction.tone === "danger",
            disabled: state.busy || batchAction.jobs.length === 0 || (batchAction.requiresReset && batchConfirm.trim().toUpperCase() !== "RESET"),
          }}
          destroyOnClose
          centered
          width={560}
        >
          <div className="action-dialog__body">
            <p className="action-dialog__description">{batchAction.description ?? "会按顺序执行当前选择的资源操作。"}</p>
            {batchAction.tone === "danger" ? (
              <Alert
                type="warning"
                showIcon
                message={batchAction.requiresReset ? "危险操作" : "批量确认"}
                description={batchAction.requiresReset ? "该批量操作会丢弃本地数据，请输入 RESET 确认。" : "该批量操作不可恢复，请确认当前选择正确。"}
              />
            ) : null}
            <div className="action-dialog__fields">
              <label className="action-field">
                <span>批量目标</span>
                <div className="output-batch__targets">
                  {batchAction.jobs.map((job) => (
                    <span key={`${batchAction.actionId}-${job.label}`} className="output-batch__target">
                      {job.label}
                    </span>
                  ))}
                </div>
              </label>
              {batchAction.requiresReset ? (
                <label className="action-field">
                  <span>确认文本</span>
                  <AntInput
                    value={batchConfirm}
                    onChange={(event) => setBatchConfirm(event.target.value)}
                    placeholder="输入 RESET"
                  />
                </label>
              ) : null}
            </div>
          </div>
        </Modal>
      ) : null}
    </section>
  );
}
