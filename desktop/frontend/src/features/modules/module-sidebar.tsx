import { PanelRightClose } from "lucide-react";

import { Button } from "../../components/ui/button";
import { cn } from "../../lib/classnames";
import { useAppContext } from "../../app/providers/app-context";

const menuTitles: Record<string, string> = {
  source: "源码与仓库",
  collaboration: "协作与评审",
  release: "发布与版本",
  all: "全部模块",
};

export function ModuleSidebar() {
  const { state, filteredModules, setSelectedModule, setModulePaneCollapsed, refresh } = useAppContext();
  const menuTitle = menuTitles[state.selectedMenu] ?? "模块";

  return (
    <aside className="module-pane">
      <div className="module-pane__header">
        <div>
          <h2>{menuTitle}</h2>
          <p>
            {filteredModules.length} 个模块
          </p>
        </div>
        <div className="module-pane__actions">
          <Button variant="ghost" onClick={() => void refresh()}>
            刷新
          </Button>
          <Button variant="ghost" className="module-pane__collapse" onClick={() => setModulePaneCollapsed(true)}>
            <PanelRightClose size={14} />
          </Button>
        </div>
      </div>

      <div className="module-pane__list">
        {filteredModules.map((module) => (
          <button
            key={module.id}
            className={cn(
              "module-pane__item",
              state.selectedModuleId === module.id && "module-pane__item--active"
            )}
            onClick={() => setSelectedModule(module.id)}
            type="button"
          >
            <strong>{module.title}</strong>
            <span>{module.description}</span>
          </button>
        ))}
        {filteredModules.length === 0 && <div className="module-pane__empty">当前菜单下暂无模块</div>}
      </div>
    </aside>
  );
}
