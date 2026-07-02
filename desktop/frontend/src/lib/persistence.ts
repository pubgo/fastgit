import type { ProjectSettings, SidebarMenuType } from "../app/types";

const PREFS_KEY = "fastgit.desktop.prefs.v1";

export interface DesktopPrefs {
  selectedMenu: SidebarMenuType;
  modulePaneWidth: number;
  modulePaneCollapsed: boolean;
  repoNamespaces: string[];
  selectedRepoPath: string;
  projectSettings: Record<string, ProjectSettings>;
}

interface WorkspaceTabs {
  openedModuleIds: string[];
  selectedModuleId: string | null;
}

function canUseStorage(): boolean {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

function normalizeProjectSettings(input: unknown): Record<string, ProjectSettings> | undefined {
  if (!input || typeof input !== "object") {
    return undefined;
  }

  const entries = Object.entries(input as Record<string, unknown>)
    .map(([repoPath, value]) => {
      if (!repoPath.trim() || !value || typeof value !== "object") {
        return null;
      }
      const defaultBaseBranch = String((value as Partial<ProjectSettings>).defaultBaseBranch ?? "").trim();
      const defaultRemote = String((value as Partial<ProjectSettings>).defaultRemote ?? "").trim();
      return [repoPath.trim(), { defaultBaseBranch, defaultRemote }] as const;
    })
    .filter((entry): entry is readonly [string, ProjectSettings] => entry !== null);

  return Object.fromEntries(entries);
}

export function loadPrefs(): Partial<DesktopPrefs> {
  if (!canUseStorage()) {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(PREFS_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as Partial<DesktopPrefs>;
    if (!parsed || typeof parsed !== "object") {
      return {};
    }

    const repoNamespaces = Array.isArray(parsed.repoNamespaces)
      ? parsed.repoNamespaces.map((value) => String(value).trim()).filter(Boolean)
      : undefined;
    const selectedRepoPath =
      typeof parsed.selectedRepoPath === "string" ? parsed.selectedRepoPath.trim() : undefined;
    const projectSettings = normalizeProjectSettings(parsed.projectSettings);

    return {
      ...parsed,
      repoNamespaces,
      selectedRepoPath,
      projectSettings,
    };
  } catch {
    return {};
  }
}

export function savePrefs(prefs: DesktopPrefs): void {
  if (!canUseStorage()) {
    return;
  }

  window.localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
}

function workspaceTabsKey(repoPath: string): string {
  return `fastgit.desktop.tabs:${repoPath}`;
}

export function loadWorkspaceTabs(repoPath: string): WorkspaceTabs | null {
  if (!canUseStorage() || !repoPath) {
    return null;
  }

  try {
    const raw = window.localStorage.getItem(workspaceTabsKey(repoPath));
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<WorkspaceTabs>;
    if (!parsed || typeof parsed !== "object") {
      return null;
    }

    return {
      openedModuleIds: Array.isArray(parsed.openedModuleIds)
        ? parsed.openedModuleIds.map((value) => String(value))
        : [],
      selectedModuleId: parsed.selectedModuleId ? String(parsed.selectedModuleId) : null,
    };
  } catch {
    return null;
  }
}

export function saveWorkspaceTabs(repoPath: string, tabs: WorkspaceTabs): void {
  if (!canUseStorage() || !repoPath) {
    return;
  }

  window.localStorage.setItem(workspaceTabsKey(repoPath), JSON.stringify(tabs));
}
