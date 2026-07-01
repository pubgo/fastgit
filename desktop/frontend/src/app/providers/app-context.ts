import { createContext, useContext } from "react";

import type { ActionRunRequest, AppState, DesktopModule, ModuleAction, ProjectSettings, SidebarMenuType } from "../types";

export interface AppContextValue {
  state: AppState;
  selectedModule: DesktopModule | null;
  filteredModules: DesktopModule[];
  setSelectedModule(moduleId: string): void;
  closeModuleTab(moduleId: string): void;
  closeOtherModuleTabs(moduleId: string): void;
  closeTabsToRight(moduleId: string): void;
  reorderModuleTabs(sourceId: string, targetId: string): void;
  addModuleTab(): void;
  setSelectedMenu(menu: SidebarMenuType): void;
  setModulePaneWidth(width: number): void;
  setModulePaneCollapsed(collapsed: boolean): void;
  refresh(preferredRepoPath?: string): Promise<void>;
  refreshGitHubAuthStatus(): Promise<void>;
  addRepo(path: string): Promise<void>;
  switchRepo(path: string): Promise<void>;
  removeRepo(path: string): Promise<void>;
  setGitHubToken(token: string): Promise<void>;
  updateProjectSettings(patch: Partial<ProjectSettings>): void;
  runAction(module: DesktopModule, action: ModuleAction, values: ActionRunRequest["values"]): Promise<void>;
  runActionAndReload(
    module: DesktopModule,
    action: ModuleAction,
    values: ActionRunRequest["values"],
    reloadActionId: string
  ): Promise<void>;
  previewAction(moduleId: string, actionId: string, values?: ActionRunRequest["values"]): Promise<void>;
  prefetchAction(moduleId: string, actionId: string, values?: ActionRunRequest["values"]): Promise<void>;
}

export const AppContext = createContext<AppContextValue | null>(null);

export function useAppContext(): AppContextValue {
  const value = useContext(AppContext);
  if (!value) {
    throw new Error("useAppContext must be used within AppProvider");
  }
  return value;
}
