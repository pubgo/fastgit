import type { LucideIcon } from "lucide-react";
import { GitFork, LayoutGrid, RefreshCcw, Tags, UsersRound } from "lucide-react";

import type { SidebarMenuType } from "../../app/types";
import { Button } from "../../components/ui/button";
import { cn } from "../../lib/classnames";
import { useAppContext } from "../../app/providers/app-context";

interface MenuItem {
  id: SidebarMenuType;
  label: string;
  icon: LucideIcon;
}

const items: MenuItem[] = [
  { id: "source", label: "源码", icon: GitFork },
  { id: "collaboration", label: "协作", icon: UsersRound },
  { id: "release", label: "发布", icon: Tags },
  { id: "all", label: "全部", icon: LayoutGrid },
];

export function SidebarMenu() {
  const { state, setSelectedMenu, refresh } = useAppContext();

  return (
    <aside className="sidebar-menu-panel">
      <div className="sidebar-menu-panel__group">
        {items.map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.id}
              className={cn(
                "sidebar-menu-panel__item",
                state.selectedMenu === item.id && "sidebar-menu-panel__item--active"
              )}
              onClick={() => setSelectedMenu(item.id)}
              title={item.label}
              type="button"
            >
              <Icon size={18} strokeWidth={2} />
              <span>{item.label}</span>
            </button>
          );
        })}
      </div>

      <div className="sidebar-menu-panel__footer">
        <Button variant="ghost" className="sidebar-menu-panel__refresh" onClick={() => void refresh()}>
          <RefreshCcw size={14} />
        </Button>
        <span>{state.modulePaneCollapsed ? "已收起" : state.busy ? "运行中" : "空闲"}</span>
      </div>
    </aside>
  );
}
