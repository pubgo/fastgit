import { useEffect, useMemo, useState } from "react";
import type { DragEvent, MouseEvent } from "react";
import { FolderGit2, Plus, X } from "lucide-react";

import { useAppContext } from "../../app/providers/app-context";

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? "workspace";
}

export function TopTabs() {
  const {
    state,
    addModuleTab,
    closeModuleTab,
    closeOtherModuleTabs,
    closeTabsToRight,
    reorderModuleTabs,
    setSelectedModule,
  } = useAppContext();
  const repoName = basename(state.repoPath);
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; moduleId: string } | null>(null);
  const openedModules = state.openedModuleIds
    .map((id) => state.modules.find((module) => module.id === id))
    .filter((module): module is NonNullable<typeof module> => module !== undefined);
  const openedCount = openedModules.length;

  const menuModule = useMemo(() => {
    if (!menu) {
      return null;
    }
    return openedModules.find((module) => module.id === menu.moduleId) ?? null;
  }, [menu, openedModules]);
  const menuIndex = menuModule ? openedModules.findIndex((module) => module.id === menuModule.id) : -1;
  const hasOthers = openedCount > 1;
  const hasRight = menuIndex >= 0 && menuIndex < openedCount - 1;

  useEffect(() => {
    const onPointerDown = () => setMenu(null);
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenu(null);
      }
    };
    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, []);

  const handleDragStart = (event: DragEvent<HTMLDivElement>, moduleId: string) => {
    setDraggedId(moduleId);
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", moduleId);
  };

  const handleDragOver = (event: DragEvent<HTMLDivElement>, moduleId: string) => {
    event.preventDefault();
    if (draggedId && draggedId !== moduleId) {
      setDragOverId(moduleId);
    }
  };

  const handleDrop = (event: DragEvent<HTMLDivElement>, moduleId: string) => {
    event.preventDefault();
    if (!draggedId || draggedId === moduleId) {
      return;
    }
    reorderModuleTabs(draggedId, moduleId);
    setDragOverId(null);
    setDraggedId(null);
  };

  const handleContextMenu = (event: MouseEvent<HTMLElement>, moduleId: string) => {
    event.preventDefault();
    event.stopPropagation();
    setMenu({
      x: event.clientX,
      y: event.clientY,
      moduleId,
    });
  };

  const runMenuAction = (action: () => void) => {
    action();
    setMenu(null);
  };

  return (
    <header className="tabs-header app-drag-region">
      <div className="tabs-repo app-no-drag-region">
        <FolderGit2 size={14} />
        <span>{repoName}</span>
      </div>

      <div className="tabs-track app-no-drag-region">
        {openedModules.map((module) => {
          const active = state.selectedModuleId === module.id;
          return (
            <div
              key={module.id}
              className={[
                "tabs-track__item",
                active ? "tabs-track__item--active" : "",
                dragOverId === module.id ? "tabs-track__item--drag-over" : "",
              ]
                .filter(Boolean)
                .join(" ")}
              onClick={() => setSelectedModule(module.id)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  setSelectedModule(module.id);
                }
              }}
              onContextMenu={(event) => handleContextMenu(event, module.id)}
              draggable={openedCount > 1}
              onDragStart={(event) => handleDragStart(event, module.id)}
              onDragOver={(event) => handleDragOver(event, module.id)}
              onDrop={(event) => handleDrop(event, module.id)}
              onDragEnd={() => {
                setDraggedId(null);
                setDragOverId(null);
              }}
              role="button"
              tabIndex={0}
              aria-label={`switch to ${module.title}`}
              aria-pressed={active}
            >
              <span>{module.title}</span>
              <button
                className="tabs-track__close"
                onClick={(event) => {
                  event.stopPropagation();
                  closeModuleTab(module.id);
                }}
                onContextMenu={(event) => handleContextMenu(event, module.id)}
                type="button"
                aria-label={`close ${module.title}`}
              >
                <X size={13} />
              </button>
            </div>
          );
        })}
      </div>

      <button className="tabs-add app-no-drag-region" onClick={addModuleTab} type="button" aria-label="new tab">
        <Plus size={14} />
      </button>

      {menu && menuModule && (
        <div
          className="tabs-menu app-no-drag-region"
          style={{ left: `${menu.x}px`, top: `${menu.y}px` }}
          onPointerDown={(event) => event.stopPropagation()}
        >
          <button
            type="button"
            className="tabs-menu__item"
            onClick={() => runMenuAction(() => closeModuleTab(menuModule.id))}
            disabled={!hasOthers}
          >
            关闭标签
          </button>
          <button
            type="button"
            className="tabs-menu__item"
            onClick={() => runMenuAction(() => closeOtherModuleTabs(menuModule.id))}
            disabled={!hasOthers}
          >
            关闭其他标签
          </button>
          <button
            type="button"
            className="tabs-menu__item"
            onClick={() => runMenuAction(() => closeTabsToRight(menuModule.id))}
            disabled={!hasRight}
          >
            关闭右侧标签
          </button>
        </div>
      )}
    </header>
  );
}
