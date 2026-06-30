import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { BackendService } from "../../services/backend";
import { filterModulesByMenu } from "../../lib/module-groups";
import { loadPrefs, loadWorkspaceTabs, savePrefs, saveWorkspaceTabs } from "../../lib/persistence";
import { AppContext } from "./app-context";
import type { AppState, DesktopModule, ModuleAction, OperationOutput, SidebarMenuType } from "../types";

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
    modules: [],
    selectedModuleId: null,
    openedModuleIds: [],
    selectedMenu: prefs.selectedMenu ?? "source",
    modulePaneWidth: clampPaneWidth(prefs.modulePaneWidth ?? 280),
    modulePaneCollapsed: prefs.modulePaneCollapsed ?? false,
    busy: false,
    output: initialOutput,
  };
}

export function AppProvider({ children }: AppProviderProps) {
  const [state, setState] = useState<AppState>(createInitialState);

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

          return {
            ...prev,
            repoPath,
            repoNamespaces: prioritizeRepo(mergeRepoNamespaces(prev.repoNamespaces, preferred, repoPath), repoPath),
            repoStatus: repoPath ? `当前项目: ${repoPath}` : "请先选择项目命名空间",
            modules,
            selectedModuleId: nextSelected,
            openedModuleIds: upsertOpened(nextOpened, nextSelected),
          };
        });
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
    [setBusy]
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
        const result = await backend.runAction({
          moduleID: module.id,
          actionID: action.id,
          values,
        });
        setState((prev) => ({
          ...prev,
          output: {
            title: `${module.title} / ${action.title}`,
            command: result.command,
            exitCode: result.exitCode,
            body: result.output || "(no output)",
          },
        }));
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
    [setBusy, state.repoPath]
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
    });
  }, [state.modulePaneCollapsed, state.modulePaneWidth, state.repoNamespaces, state.repoPath, state.selectedMenu]);

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
      addRepo,
      switchRepo,
      removeRepo,
      runAction,
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
      addRepo,
      switchRepo,
      removeRepo,
      runAction,
    ]
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}
