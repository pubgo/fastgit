import type {
  ActionRunRequest,
  CommandResult,
  DesktopModule,
  GitHubAuthStatus,
  ModuleAction,
} from "../../bindings/fastgitdesktop/models";

export type { ActionRunRequest, CommandResult, DesktopModule, GitHubAuthStatus, ModuleAction };

export type SidebarMenuType = "source" | "collaboration" | "release" | "all";

export interface OutputListItem {
  id: string;
  primary: string;
  secondary?: string;
  badge?: string;
  active?: boolean;
  url?: string;
  value?: string;
  category?: string;
  keywords?: string[];
  fields?: Record<string, string>;
}

export interface OutputDetailField {
  label: string;
  value: string;
  href?: string;
}

export interface OutputDetail {
  targetId?: string;
  primary: string;
  secondary?: string;
  badge?: string;
  url?: string;
  body?: string;
  fields?: OutputDetailField[];
}

export interface OperationOutput {
  title: string;
  command: string;
  exitCode: number;
  body: string;
  moduleId?: string;
  actionId?: string;
  items?: OutputListItem[];
  detail?: OutputDetail;
  emptyHint?: string;
}

export interface ResourceCatalog {
  branches: OutputListItem[];
  issues: OutputListItem[];
  tags: OutputListItem[];
  worktrees: OutputListItem[];
  prs: OutputListItem[];
  repoStatus: OutputListItem[];
}

export interface ProjectSettings {
  defaultBaseBranch: string;
}

export interface AppState {
  repoPath: string;
  repoNamespaces: string[];
  repoStatus: string;
  githubAuthStatus: GitHubAuthStatus | null;
  projectSettings: Record<string, ProjectSettings>;
  modules: DesktopModule[];
  selectedModuleId: string | null;
  openedModuleIds: string[];
  selectedMenu: SidebarMenuType;
  modulePaneWidth: number;
  modulePaneCollapsed: boolean;
  busy: boolean;
  output: OperationOutput;
  catalog: ResourceCatalog;
}
