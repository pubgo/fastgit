import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { BackendService } from "../../services/backend";
import { filterModulesByMenu } from "../../lib/module-groups";
import { loadPrefs, loadWorkspaceTabs, savePrefs, saveWorkspaceTabs } from "../../lib/persistence";
import { AppContext } from "./app-context";
import type {
  AppState,
  DesktopModule,
  ModuleAction,
  OperationOutput,
  OutputDetail,
  OutputListItem,
  ProjectSettings,
  ResourceCatalog,
  SidebarMenuType,
} from "../types";

interface AppProviderProps {
  children: ReactNode;
}

const initialOutput: OperationOutput = {
  title: "Welcome",
  command: "-",
  exitCode: 0,
  body: "请选择左侧模块并执行动作。",
};

const backend = new BackendService();

const githubRowPattern = /^#(\d+)\s+\[([^\]]+)\]\s+(.+)$/;
const githubHeaderPattern = /^#(\d+)\s+\[([^\]]+)\]$/;
const statusRowPattern = /^(.{2})\s+(.+)$/;

function createEmptyCatalog(): ResourceCatalog {
  return {
    branches: [],
    issues: [],
    tags: [],
    worktrees: [],
    prs: [],
    repoStatus: [],
  };
}

function parseBranchItems(body: string): OutputListItem[] {
  const lines = body.split(/\r?\n/).map((line) => line.trimEnd());
  const items: OutputListItem[] = [];

  lines.forEach((line, index) => {
    const trimmed = line.trim();
    if (!trimmed) {
      return;
    }

    const active = trimmed.startsWith("* ");
    const primary = active ? trimmed.slice(2).trim() : trimmed.replace(/^-+\s+/, "");
    if (!primary) {
      return;
    }

    items.push({
      id: `${primary}-${index}`,
      primary,
      active,
      badge: active ? "current" : undefined,
      value: primary,
      keywords: [primary, active ? "current" : ""].filter(Boolean),
      fields: {
        branch: primary,
        status: active ? "current" : "local",
      },
    });
  });

  return items;
}

function parseTagItems(body: string): OutputListItem[] {
  return body
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((tag, index) => ({
      id: `${tag}-${index}`,
      primary: tag,
      value: tag,
      keywords: [tag],
      fields: {
        tag,
      },
    }));
}

function parseIssueLikeItems(body: string): OutputListItem[] {
  return body
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .flatMap((line, index) => {
      const match = line.match(githubRowPattern);
      if (!match) {
        return [];
      }
      return [
        {
          id: `${match[1]}-${index}`,
          primary: `#${match[1]} ${match[3]}`,
          badge: match[2],
          category: match[2],
          value: match[1],
          keywords: [match[1], match[2], match[3]],
          fields: {
            number: match[1],
            title: match[3],
            state: match[2],
          },
        },
      ];
    });
}

function parsePrStatusItems(body: string): OutputListItem[] {
  const lines = body.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  if (lines.length === 0) {
    return [];
  }
  const header = lines[0];
  const match = header.match(/^#(\d+)\s+\[([^\]]+)\]$/);
  if (!match) {
    return [];
  }

  const title = lines[1] ?? `PR #${match[1]}`;
  const url = lines.find((line) => /^https?:\/\//.test(line));

  return [
    {
      id: `pr-${match[1]}`,
      primary: `#${match[1]} ${title}`,
      badge: match[2],
      category: match[2],
      secondary: url,
      url,
      value: match[1],
      keywords: [match[1], match[2], title, url ?? ""].filter(Boolean),
      fields: {
        number: match[1],
        title,
        state: match[2],
        url: url ?? "",
      },
    },
  ];
}

function parsePrStatusDetail(body: string): OutputDetail | null {
  const lines = body.split(/\r?\n/);
  const header = lines[0]?.trim() ?? "";
  const title = lines[1]?.trim() ?? "";
  const url = lines.find((line, index) => index > 0 && /^https?:\/\//.test(line.trim()))?.trim();
  const match = header.match(githubHeaderPattern);
  if (!match || !title) {
    return null;
  }

  const base = lines.find((line) => line.startsWith("base: "))?.slice("base: ".length).trim();
  const head = lines.find((line) => line.startsWith("head: "))?.slice("head: ".length).trim();
  const draft = lines.find((line) => line.startsWith("draft: "))?.slice("draft: ".length).trim();
  const bodyStartIndex = lines.findIndex((line, index) => index > 1 && line.trim() === "");
  const detailBody =
    bodyStartIndex >= 0
      ? lines.slice(bodyStartIndex + 1).join("\n").trim()
      : "";

  return {
    targetId: `pr-${match[1]}`,
    primary: `#${match[1]} ${title}`,
    secondary: url,
    badge: match[2],
    url,
    body: detailBody || undefined,
    fields: [
      { label: "PR", value: `#${match[1]}` },
      { label: "状态", value: match[2] },
      ...(base ? [{ label: "Base", value: base }] : []),
      ...(head ? [{ label: "Head", value: head }] : []),
      ...(draft ? [{ label: "Draft", value: draft }] : []),
      ...(url ? [{ label: "链接", value: url, href: url }] : []),
    ],
  };
}

function parseIssueDetail(body: string): OutputDetail | null {
  const lines = body.split(/\r?\n/);
  const header = lines[0]?.trim() ?? "";
  const title = lines[1]?.trim() ?? "";
  const url = lines.find((line, index) => index > 0 && /^https?:\/\//.test(line.trim()))?.trim();
  const match = header.match(githubHeaderPattern);
  if (!match || !title) {
    return null;
  }

  const bodyStartIndex = lines.findIndex((line, index) => index > 1 && line.trim() === "");
  const detailBody =
    bodyStartIndex >= 0
      ? lines.slice(bodyStartIndex + 1).join("\n").trim()
      : lines.slice(url ? 3 : 2).join("\n").trim();

  return {
    targetId: match[1],
    primary: `#${match[1]} ${title}`,
    secondary: url,
    badge: match[2],
    url,
    body: detailBody || undefined,
    fields: [
      { label: "Issue", value: `#${match[1]}` },
      { label: "状态", value: match[2] },
      ...(url ? [{ label: "链接", value: url, href: url }] : []),
    ],
  };
}

function parseUrlActionDetail(actionID: string, body: string): OutputDetail | null {
  const urlMatch = body.match(/https?:\/\/\S+/);
  if (!urlMatch) {
    return null;
  }
  const url = urlMatch[0];

  switch (actionID) {
    case "issue_create":
      return {
        primary: "Issue 创建成功",
        secondary: url,
        badge: "created",
        url,
        fields: [{ label: "链接", value: url, href: url }],
      };
    case "pr_create":
      return {
        primary: body.startsWith("PR already exists:") ? "PR 已存在" : "PR 创建成功",
        secondary: url,
        badge: body.startsWith("PR already exists:") ? "exists" : "created",
        url,
        fields: [{ label: "链接", value: url, href: url }],
      };
    case "pr_sync":
      return {
        primary: "PR 已同步",
        secondary: url,
        badge: "updated",
        url,
        fields: [{ label: "链接", value: url, href: url }],
      };
    default:
      return null;
  }
}

function parseMergeDetail(body: string): OutputDetail | null {
  const match = body.match(/^merged=(true|false)\s+message=(.+)$/);
  if (!match) {
    return null;
  }
  return {
    primary: match[1] === "true" ? "PR 合并完成" : "PR 合并未完成",
    badge: match[1] === "true" ? "merged" : "pending",
    body: match[2],
    fields: [
      { label: "结果", value: match[1] === "true" ? "merged" : "not merged" },
      { label: "消息", value: match[2] },
    ],
  };
}

function parseRepoStatusItems(body: string): OutputListItem[] {
  return body
    .split(/\r?\n/)
    .map((line) => line.trimEnd())
    .filter(Boolean)
    .flatMap((line, index) => {
      const match = line.match(statusRowPattern);
      if (!match) {
        return [];
      }
      const code = match[1].replace(/\s/g, "·");
      return [
        {
          id: `repo-${index}`,
          primary: match[2],
          badge: code,
          category: code,
          keywords: [code, match[2]],
          fields: {
            path: match[2],
            status: code,
          },
        },
      ];
    });
}

function parseWorktreeItems(body: string, repoPath: string): OutputListItem[] {
  const lines = body.split(/\r?\n/);
  const items: OutputListItem[] = [];

  let path = "";
  let branch = "";
  let head = "";

  const push = () => {
    if (!path) {
      return;
    }
    const details = [branch, head ? head.slice(0, 8) : ""].filter(Boolean).join(" · ");
    items.push({
      id: `${path}-${items.length}`,
      primary: path,
      secondary: details || undefined,
      active: path === repoPath,
      badge: path === repoPath ? "current" : undefined,
      value: branch || path,
      keywords: [path, branch, head, path === repoPath ? "current" : ""].filter(Boolean),
      fields: {
        path,
        branch: branch || "-",
        commit: head ? head.slice(0, 8) : "-",
        status: path === repoPath ? "current" : "attached",
      },
    });
  };

  lines.forEach((rawLine) => {
    const line = rawLine.trim();
    if (!line) {
      return;
    }
    if (line.startsWith("worktree ")) {
      push();
      path = line.slice("worktree ".length).trim();
      branch = "";
      head = "";
      return;
    }
    if (line.startsWith("branch ")) {
      const ref = line.slice("branch ".length).trim();
      branch = ref.replace("refs/heads/", "");
      return;
    }
    if (line.startsWith("HEAD ")) {
      head = line.slice("HEAD ".length).trim();
    }
  });

  push();
  return items;
}

function updateCatalog(catalog: ResourceCatalog, actionID: string, items?: OutputListItem[]): ResourceCatalog {
  if (!items) {
    return catalog;
  }

  switch (actionID) {
    case "branch_list":
      return { ...catalog, branches: items };
    case "issue_list":
      return { ...catalog, issues: items };
    case "tag_list":
      return { ...catalog, tags: items };
    case "worktree_list":
      return { ...catalog, worktrees: items };
    case "pr_status":
      return { ...catalog, prs: items };
    case "repo_status":
      return { ...catalog, repoStatus: items };
    default:
      return catalog;
  }
}

function findModuleAction(modules: DesktopModule[], moduleID: string, actionID: string): { module: DesktopModule; action: ModuleAction } | null {
  const module = modules.find((item) => item.id === moduleID);
  const action = module?.actions?.find((item) => item.id === actionID);
  if (!module || !action) {
    return null;
  }
  return { module, action };
}

function relatedCatalogRefreshes(moduleId: string, actionId: string): Array<{ moduleId: string; actionId: string }> {
  switch (actionId) {
    case "branch_create":
    case "branch_delete":
    case "branch_checkout":
    case "branch_force_sync":
      return [
        { moduleId: "branch", actionId: "branch_list" },
        { moduleId: "repo", actionId: "repo_status" },
      ];
    case "worktree_create":
    case "worktree_remove":
      return [
        { moduleId: "worktree", actionId: "worktree_list" },
        { moduleId: "branch", actionId: "branch_list" },
      ];
    case "issue_create":
      return [{ moduleId: "issue", actionId: "issue_list" }];
    case "tag_publish":
    case "tag_push":
      return [{ moduleId: "tag", actionId: "tag_list" }];
    case "repo_pull":
    case "repo_push":
      return [{ moduleId: "repo", actionId: "repo_status" }];
    case "pr_create":
    case "pr_sync":
    case "pr_merge":
      return [{ moduleId: "pr", actionId: "pr_status" }];
    default:
      return [];
  }
}

function toStructuredOutput(actionID: string, body: string, repoPath: string): Pick<OperationOutput, "items" | "detail" | "emptyHint"> {
  const normalized = body.trim();
  switch (actionID) {
    case "branch_list": {
      const items = parseBranchItems(normalized);
      return { items, emptyHint: items.length === 0 ? "暂无分支" : undefined };
    }
    case "tag_list": {
      const items = parseTagItems(normalized);
      return { items, emptyHint: normalized === "no tags" ? "暂无标签" : items.length === 0 ? "暂无标签" : undefined };
    }
    case "worktree_list": {
      const items = parseWorktreeItems(normalized, repoPath);
      return {
        items,
        emptyHint: normalized === "no worktree" ? "暂无 worktree" : items.length === 0 ? "暂无 worktree" : undefined,
      };
    }
    case "issue_list": {
      const items = parseIssueLikeItems(normalized);
      return {
        items,
        emptyHint: normalized === "no open issues" ? "暂无 open issues" : items.length === 0 ? "暂无 open issues" : undefined,
      };
    }
    case "pr_status": {
      const items = parsePrStatusItems(normalized);
      return {
        items,
        detail: items.length > 0 ? parsePrStatusDetail(normalized) ?? undefined : undefined,
        emptyHint: normalized === "no open PR for current branch" ? "当前分支暂无 open PR" : undefined,
      };
    }
    case "repo_status": {
      const items = parseRepoStatusItems(normalized);
      return {
        items,
        emptyHint: normalized === "working tree clean" ? "工作区干净" : undefined,
      };
    }
    case "issue_view":
      return {
        detail: parseIssueDetail(normalized) ?? undefined,
      };
    case "issue_create":
    case "pr_create":
    case "pr_sync":
      return {
        detail: parseUrlActionDetail(actionID, normalized) ?? undefined,
      };
    case "pr_merge":
      return {
        detail: parseMergeDetail(normalized) ?? undefined,
      };
    default:
      return {};
  }
}

function normalizeRepoPath(path: string): string {
  return path.trim();
}

function mergeRepoNamespaces(existing: string[], ...paths: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];

  const append = (value: string) => {
    const normalized = normalizeRepoPath(value);
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    result.push(normalized);
  };

  existing.forEach(append);
  paths.forEach(append);

  return result;
}

function prioritizeRepo(existing: string[], path: string): string[] {
  const target = normalizeRepoPath(path);
  if (!target) {
    return existing;
  }
  return [target, ...existing.filter((item) => normalizeRepoPath(item) !== target)];
}

function upsertOpened(opened: string[], moduleId: string | null): string[] {
  if (!moduleId) {
    return opened;
  }
  return opened.includes(moduleId) ? opened : [...opened, moduleId];
}

function clampPaneWidth(width: number): number {
  return Math.max(220, Math.min(420, width));
}

function fallbackSelection(modules: DesktopModule[], selectedMenu: SidebarMenuType, opened: string[]): string | null {
  const filtered = filterModulesByMenu(modules, selectedMenu);
  const lastOpened = opened[opened.length - 1];
  return lastOpened ?? filtered[0]?.id ?? modules[0]?.id ?? null;
}

function createInitialState(): AppState {
  const prefs = loadPrefs();
  const selectedRepoPath = normalizeRepoPath(prefs.selectedRepoPath ?? "");
  const repoNamespaces = prioritizeRepo(
    mergeRepoNamespaces(prefs.repoNamespaces ?? [], selectedRepoPath),
    selectedRepoPath
  );

  return {
    repoPath: selectedRepoPath,
    repoNamespaces,
    repoStatus: "加载中...",
    githubAuthStatus: null,
    projectSettings: prefs.projectSettings ?? {},
    modules: [],
    selectedModuleId: null,
    openedModuleIds: [],
    selectedMenu: prefs.selectedMenu ?? "source",
    modulePaneWidth: clampPaneWidth(prefs.modulePaneWidth ?? 280),
    modulePaneCollapsed: prefs.modulePaneCollapsed ?? false,
    busy: false,
    output: initialOutput,
    catalog: createEmptyCatalog(),
  };
}

export function AppProvider({ children }: AppProviderProps) {
  const [state, setState] = useState<AppState>(createInitialState);

  const refreshGitHubAuthStatus = useCallback(async () => {
    try {
      const status = await backend.getGitHubAuthStatus();
      setState((prev) => ({
        ...prev,
        githubAuthStatus: status,
      }));
    } catch (error) {
      setState((prev) => ({
        ...prev,
        githubAuthStatus: {
          configured: false,
          source: "none",
          message: `GitHub 状态读取失败: ${String(error)}`,
        },
      }));
    }
  }, []);

  const applyResultToState = useCallback(
    (
      module: DesktopModule,
      action: ModuleAction,
      result: { command: string; exitCode: number; output: string },
      repoPath: string,
      overrides?: Partial<OperationOutput>
    ) => {
      const structured = result.exitCode === 0 ? toStructuredOutput(action.id, result.output || "", repoPath) : {};
      setState((prev) => ({
        ...prev,
        output: {
          title: `${module.title} / ${action.title}`,
          command: result.command,
          exitCode: result.exitCode,
          body: result.output || "(no output)",
          moduleId: module.id,
          actionId: action.id,
          items: structured.items,
          detail: structured.detail,
          emptyHint: structured.emptyHint,
          ...overrides,
        },
        catalog: result.exitCode === 0 ? updateCatalog(prev.catalog, action.id, structured.items) : prev.catalog,
      }));
    },
    []
  );

  const requestAction = useCallback(async (moduleId: string, actionId: string, values: Record<string, string>) => {
    return backend.runAction({
      moduleID: moduleId,
      actionID: actionId,
      values,
    });
  }, []);

  const setBusy = useCallback((busy: boolean) => {
    setState((prev) => ({ ...prev, busy }));
  }, []);

  const refresh = useCallback(
    async (preferredRepoPath?: string) => {
      const preferred = normalizeRepoPath(preferredRepoPath ?? "");
      setBusy(true);
      try {
        if (preferred) {
          await backend.setRepoRoot(preferred);
        }

        const [repoPathRaw, modules] = await Promise.all([backend.getRepoRoot(), backend.getModules()]);
        const repoPath = normalizeRepoPath(repoPathRaw);

        setState((prev) => {
          const storedTabs = loadWorkspaceTabs(repoPath);
          const filtered = filterModulesByMenu(modules, prev.selectedMenu);
          const selectedCandidate = storedTabs?.selectedModuleId ?? prev.selectedModuleId;
          const nextSelected =
            selectedCandidate && filtered.some((module) => module.id === selectedCandidate)
              ? selectedCandidate
              : filtered[0]?.id ?? modules[0]?.id ?? null;
          const availableIDs = new Set(modules.map((module) => module.id));
          const openedCandidate = storedTabs?.openedModuleIds ?? prev.openedModuleIds;
          const nextOpened = openedCandidate.filter((id) => availableIDs.has(id));
          const repoChanged = prev.repoPath !== repoPath;

          return {
            ...prev,
            repoPath,
            repoNamespaces: prioritizeRepo(mergeRepoNamespaces(prev.repoNamespaces, preferred, repoPath), repoPath),
            repoStatus: repoPath ? `当前项目: ${repoPath}` : "请先选择项目命名空间",
            modules,
            selectedModuleId: nextSelected,
            openedModuleIds: upsertOpened(nextOpened, nextSelected),
            catalog: repoChanged ? createEmptyCatalog() : prev.catalog,
          };
        });
        void refreshGitHubAuthStatus();
      } catch (error) {
        const message = String(error);
        setState((prev) => ({
          ...prev,
          output: {
            title: preferred ? "Set Project" : "Load Error",
            command: preferred ? "SetProjectRoot" : "bootstrap",
            exitCode: 1,
            body: message,
          },
          repoStatus: preferred ? `项目切换失败: ${message}` : `加载失败: ${message}`,
        }));
      } finally {
        setBusy(false);
      }
    },
    [refreshGitHubAuthStatus, setBusy]
  );

  const setSelectedModule = useCallback((moduleId: string) => {
    setState((prev) => ({
      ...prev,
      selectedModuleId: moduleId,
      openedModuleIds: upsertOpened(prev.openedModuleIds, moduleId),
    }));
  }, []);

  const setSelectedMenu = useCallback((menu: SidebarMenuType) => {
    setState((prev) => {
      if (prev.selectedMenu === menu) {
        return {
          ...prev,
          modulePaneCollapsed: !prev.modulePaneCollapsed,
        };
      }

      const filtered = filterModulesByMenu(prev.modules, menu);
      const nextSelected =
        prev.selectedModuleId && filtered.some((module) => module.id === prev.selectedModuleId)
          ? prev.selectedModuleId
          : filtered[0]?.id ?? prev.modules[0]?.id ?? null;
      return {
        ...prev,
        selectedMenu: menu,
        selectedModuleId: nextSelected,
        openedModuleIds: upsertOpened(prev.openedModuleIds, nextSelected),
        modulePaneCollapsed: false,
      };
    });
  }, []);

  const closeModuleTab = useCallback((moduleId: string) => {
    setState((prev) => {
      const nextOpened = prev.openedModuleIds.filter((id) => id !== moduleId);
      if (prev.selectedModuleId !== moduleId) {
        return {
          ...prev,
          openedModuleIds: nextOpened,
        };
      }

      const fallback = fallbackSelection(prev.modules, prev.selectedMenu, nextOpened);
      return {
        ...prev,
        openedModuleIds: nextOpened,
        selectedModuleId: fallback,
      };
    });
  }, []);

  const closeOtherModuleTabs = useCallback((moduleId: string) => {
    setState((prev) => {
      if (!prev.openedModuleIds.includes(moduleId)) {
        return prev;
      }
      return {
        ...prev,
        openedModuleIds: [moduleId],
        selectedModuleId: moduleId,
      };
    });
  }, []);

  const closeTabsToRight = useCallback((moduleId: string) => {
    setState((prev) => {
      const index = prev.openedModuleIds.indexOf(moduleId);
      if (index < 0) {
        return prev;
      }

      const nextOpened = prev.openedModuleIds.slice(0, index + 1);
      const selectedExists = prev.selectedModuleId ? nextOpened.includes(prev.selectedModuleId) : false;
      const nextSelected = selectedExists
        ? prev.selectedModuleId
        : fallbackSelection(prev.modules, prev.selectedMenu, nextOpened);
      return {
        ...prev,
        openedModuleIds: nextOpened,
        selectedModuleId: nextSelected,
      };
    });
  }, []);

  const reorderModuleTabs = useCallback((sourceId: string, targetId: string) => {
    if (sourceId === targetId) {
      return;
    }

    setState((prev) => {
      const sourceIndex = prev.openedModuleIds.indexOf(sourceId);
      const targetIndex = prev.openedModuleIds.indexOf(targetId);
      if (sourceIndex < 0 || targetIndex < 0) {
        return prev;
      }

      const nextOpened = [...prev.openedModuleIds];
      nextOpened.splice(sourceIndex, 1);
      const nextTargetIndex = nextOpened.indexOf(targetId);
      nextOpened.splice(nextTargetIndex, 0, sourceId);
      return {
        ...prev,
        openedModuleIds: nextOpened,
      };
    });
  }, []);

  const addModuleTab = useCallback(() => {
    setState((prev) => {
      const scope = filterModulesByMenu(prev.modules, prev.selectedMenu);
      const candidate = scope.find((module) => !prev.openedModuleIds.includes(module.id)) ?? prev.modules[0];
      if (!candidate) {
        return prev;
      }
      return {
        ...prev,
        selectedModuleId: candidate.id,
        openedModuleIds: upsertOpened(prev.openedModuleIds, candidate.id),
        modulePaneCollapsed: false,
      };
    });
  }, []);

  const setModulePaneWidth = useCallback((width: number) => {
    setState((prev) => ({
      ...prev,
      modulePaneWidth: clampPaneWidth(width),
    }));
  }, []);

  const setModulePaneCollapsed = useCallback((collapsed: boolean) => {
    setState((prev) => ({
      ...prev,
      modulePaneCollapsed: collapsed,
    }));
  }, []);

  const switchRepo = useCallback(
    async (path: string) => {
      const next = normalizeRepoPath(path);
      if (!next || next === state.repoPath) {
        return;
      }

      setState((prev) => ({
        ...prev,
        repoNamespaces: prioritizeRepo(mergeRepoNamespaces(prev.repoNamespaces, next), next),
      }));
      await refresh(next);
    },
    [refresh, state.repoPath]
  );

  const addRepo = useCallback(async (path: string) => {
    const next = normalizeRepoPath(path);
    if (!next) {
      return;
    }

    setState((prev) => ({
      ...prev,
      repoNamespaces: mergeRepoNamespaces(prev.repoNamespaces, next),
      repoStatus:
        next === prev.repoPath
          ? `当前项目: ${prev.repoPath}`
          : `已添加项目命名空间: ${next}`,
    }));
  }, []);

  const removeRepo = useCallback(
    async (path: string) => {
      const target = normalizeRepoPath(path);
      if (!target) {
        return;
      }

      const nextNamespaces = state.repoNamespaces.filter((repoPath) => repoPath !== target);
      if (nextNamespaces.length === state.repoNamespaces.length) {
        return;
      }

      if (state.repoPath === target) {
        if (nextNamespaces.length === 0) {
          return;
        }

        setState((prev) => ({
          ...prev,
          repoNamespaces: nextNamespaces,
        }));
        await refresh(nextNamespaces[0]);
        return;
      }

      setState((prev) => ({
        ...prev,
        repoNamespaces: nextNamespaces,
      }));
    },
    [refresh, state.repoNamespaces, state.repoPath]
  );

  const setGitHubToken = useCallback(async (token: string) => {
    await backend.setGitHubToken(token);
    await refreshGitHubAuthStatus();
    setState((prev) => ({
      ...prev,
      repoStatus: token.trim() ? "GitHub Token 已更新（当前会话）" : prev.repoStatus,
    }));
  }, [refreshGitHubAuthStatus]);

  const updateProjectSettings = useCallback((patch: Partial<ProjectSettings>) => {
    setState((prev) => {
      const repoPath = prev.repoPath.trim();
      if (!repoPath) {
        return prev;
      }
      const current = prev.projectSettings[repoPath] ?? { defaultBaseBranch: "" };
      return {
        ...prev,
        projectSettings: {
          ...prev.projectSettings,
          [repoPath]: {
            ...current,
            ...patch,
          },
        },
        repoStatus:
          "defaultBaseBranch" in patch
            ? `项目默认 base branch 已更新: ${patch.defaultBaseBranch || "(未设置)"}`
            : prev.repoStatus,
      };
    });
  }, []);

  const runAction = useCallback(
    async (module: DesktopModule, action: ModuleAction, values: Record<string, string>) => {
      if (!state.repoPath) {
        setState((prev) => ({
          ...prev,
          output: {
            title: `${module.title} / ${action.title}`,
            command: "RunAction",
            exitCode: 1,
            body: "请先选择项目命名空间。",
          },
        }));
        return;
      }

      setBusy(true);
      try {
        const result = await requestAction(module.id, action.id, values);
        applyResultToState(module, action, result, state.repoPath);

        if (result.exitCode === 0) {
          const refreshes = relatedCatalogRefreshes(module.id, action.id);
          await Promise.all(
            refreshes.map(async (refreshTarget) => {
              const followup = findModuleAction(state.modules, refreshTarget.moduleId, refreshTarget.actionId);
              if (!followup) {
                return;
              }
              const followupResult = await requestAction(followup.module.id, followup.action.id, {});
              if (followupResult.exitCode !== 0) {
                return;
              }
              const structured = toStructuredOutput(followup.action.id, followupResult.output || "", state.repoPath);
              setState((prev) => ({
                ...prev,
                catalog: updateCatalog(prev.catalog, followup.action.id, structured.items),
              }));
            })
          );
        }
      } catch (error) {
        setState((prev) => ({
          ...prev,
          output: {
            title: `${module.title} / ${action.title}`,
            command: "RunAction",
            exitCode: 1,
            body: String(error),
          },
        }));
      } finally {
        setBusy(false);
      }
    },
    [applyResultToState, requestAction, setBusy, state.modules, state.repoPath]
  );

  const runActionAndReload = useCallback(
    async (module: DesktopModule, action: ModuleAction, values: Record<string, string>, reloadActionId: string) => {
      if (!state.repoPath) {
        return;
      }

      const reloadTarget = findModuleAction(state.modules, module.id, reloadActionId) ?? findModuleAction(state.modules, state.output.moduleId ?? module.id, reloadActionId);
      if (!reloadTarget) {
        await runAction(module, action, values);
        return;
      }

      setBusy(true);
      try {
        const result = await requestAction(module.id, action.id, values);
        const actionStructured = result.exitCode === 0 ? toStructuredOutput(action.id, result.output || "", state.repoPath) : {};
        if (result.exitCode !== 0) {
          applyResultToState(module, action, result, state.repoPath);
          return;
        }

        const reloadResult = await requestAction(reloadTarget.module.id, reloadTarget.action.id, {});
        if (reloadResult.exitCode === 0) {
          const reloadStructured = toStructuredOutput(reloadTarget.action.id, reloadResult.output || "", state.repoPath);
          applyResultToState(reloadTarget.module, reloadTarget.action, reloadResult, state.repoPath, {
            detail: actionStructured.detail ?? reloadStructured.detail,
          });
        } else {
          applyResultToState(module, action, result, state.repoPath);
        }
      } catch (error) {
        setState((prev) => ({
          ...prev,
          output: {
            title: `${module.title} / ${action.title}`,
            command: "RunAction",
            exitCode: 1,
            body: String(error),
          },
        }));
      } finally {
        setBusy(false);
      }
    },
    [applyResultToState, requestAction, runAction, setBusy, state.modules, state.output.moduleId, state.repoPath]
  );

  const previewAction = useCallback(
    async (moduleId: string, actionId: string, values: Record<string, string> = {}) => {
      if (!state.repoPath) {
        return;
      }

      const target = findModuleAction(state.modules, moduleId, actionId);
      if (!target) {
        return;
      }

      try {
        const result = await requestAction(moduleId, actionId, values);
        if (result.exitCode !== 0) {
          return;
        }

        const structured = toStructuredOutput(actionId, result.output || "", state.repoPath);
        if (!structured.detail) {
          return;
        }

        setState((prev) => ({
          ...prev,
          output: {
            ...prev.output,
            command: prev.output.command,
            detail: structured.detail,
          },
        }));
      } catch {
        return;
      }
    },
    [requestAction, state.modules, state.repoPath]
  );

  const prefetchAction = useCallback(
    async (moduleId: string, actionId: string, values: Record<string, string> = {}) => {
      if (!state.repoPath) {
        return;
      }

      if (!findModuleAction(state.modules, moduleId, actionId)) {
        return;
      }

      try {
        const result = await backend.runAction({
          moduleID: moduleId,
          actionID: actionId,
          values,
        });
        if (result.exitCode !== 0) {
          return;
        }

        const structured = toStructuredOutput(actionId, result.output || "", state.repoPath);
        if (!structured.items) {
          return;
        }

        setState((prev) => ({
          ...prev,
          catalog: updateCatalog(prev.catalog, actionId, structured.items),
        }));
      } catch {
        return;
      }
    },
    [state.modules, state.repoPath]
  );

  const selectedModule = useMemo(() => {
    if (!state.selectedModuleId) {
      return null;
    }
    return state.modules.find((module) => module.id === state.selectedModuleId) ?? null;
  }, [state.modules, state.selectedModuleId]);

  const filteredModules = useMemo(
    () => filterModulesByMenu(state.modules, state.selectedMenu),
    [state.modules, state.selectedMenu]
  );

  useEffect(() => {
    savePrefs({
      selectedMenu: state.selectedMenu,
      modulePaneWidth: state.modulePaneWidth,
      modulePaneCollapsed: state.modulePaneCollapsed,
      repoNamespaces: state.repoNamespaces,
      selectedRepoPath: state.repoPath,
      projectSettings: state.projectSettings,
    });
  }, [state.modulePaneCollapsed, state.modulePaneWidth, state.projectSettings, state.repoNamespaces, state.repoPath, state.selectedMenu]);

  useEffect(() => {
    if (!state.repoPath) {
      return;
    }

    const availableIDs = new Set(state.modules.map((module) => module.id));
    const openedModuleIds = state.openedModuleIds.filter((id) => availableIDs.has(id));
    const selectedModuleId =
      state.selectedModuleId && availableIDs.has(state.selectedModuleId) ? state.selectedModuleId : null;

    saveWorkspaceTabs(state.repoPath, {
      openedModuleIds,
      selectedModuleId,
    });
  }, [state.modules, state.openedModuleIds, state.repoPath, state.selectedModuleId]);

  const value = useMemo(
    () => ({
      state,
      selectedModule,
      filteredModules,
      setSelectedModule,
      closeModuleTab,
      closeOtherModuleTabs,
      closeTabsToRight,
      reorderModuleTabs,
      addModuleTab,
      setSelectedMenu,
      setModulePaneWidth,
      setModulePaneCollapsed,
      refresh,
      refreshGitHubAuthStatus,
      addRepo,
      switchRepo,
      removeRepo,
      setGitHubToken,
      updateProjectSettings,
      runAction,
      runActionAndReload,
      previewAction,
      prefetchAction,
    }),
    [
      state,
      selectedModule,
      filteredModules,
      setSelectedModule,
      closeModuleTab,
      closeOtherModuleTabs,
      closeTabsToRight,
      reorderModuleTabs,
      addModuleTab,
      setSelectedMenu,
      setModulePaneWidth,
      setModulePaneCollapsed,
      refresh,
      refreshGitHubAuthStatus,
      addRepo,
      switchRepo,
      removeRepo,
      setGitHubToken,
      updateProjectSettings,
      runAction,
      runActionAndReload,
      previewAction,
      prefetchAction,
    ]
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}
