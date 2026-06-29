import type { DesktopModule, SidebarMenuType } from "../app/types";

export function menuForModule(moduleId: string): SidebarMenuType {
  switch (moduleId) {
    case "repo":
    case "branch":
    case "worktree":
      return "source";
    case "issue":
    case "pr":
      return "collaboration";
    case "tag":
      return "release";
    default:
      return "all";
  }
}

export function filterModulesByMenu(modules: DesktopModule[], menu: SidebarMenuType): DesktopModule[] {
  if (menu === "all") {
    return modules;
  }
  return modules.filter((module) => menuForModule(module.id) === menu);
}
