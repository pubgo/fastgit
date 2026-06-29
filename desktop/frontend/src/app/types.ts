import type {
  ActionRunRequest,
  CommandResult,
  DesktopModule,
  ModuleAction,
} from "../../bindings/fastgitdesktop/models";

export type { ActionRunRequest, CommandResult, DesktopModule, ModuleAction };

export type SidebarMenuType = "source" | "collaboration" | "release" | "all";

export interface OperationOutput {
  title: string;
  command: string;
  exitCode: number;
  body: string;
}

export interface AppState {
  repoPath: string;
  repoStatus: string;
  modules: DesktopModule[];
  selectedModuleId: string | null;
  openedModuleIds: string[];
  selectedMenu: SidebarMenuType;
  modulePaneWidth: number;
  modulePaneCollapsed: boolean;
  busy: boolean;
  output: OperationOutput;
}
