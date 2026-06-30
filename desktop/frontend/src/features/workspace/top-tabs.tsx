import { useEffect, useMemo, useRef, useState } from "react";
import type { DragEvent, MouseEvent } from "react";
import { Plus, Settings2, X } from "lucide-react";

import { useAppContext } from "../../app/providers/app-context";
import logoIcon from "../../assets/brand/fastgit-logo-icon.svg";
import { RepoSwitcher } from "./repo-switcher";

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? "workspace";
}

function pathSegments(path: string): string[] {
  return path.split(/[\\/]/).filter(Boolean);
}

function buildProjectLabels(paths: string[]): Map<string, string> {
  const labels = new Map<string, string>();

  for (const path of paths) {
    const segments = pathSegments(path);
    if (segments.length === 0) {
      labels.set(path, path);
      continue;
    }

    let label = segments[segments.length - 1];
    for (let depth = 1; depth <= segments.length; depth += 1) {
      const candidate = segments.slice(-depth).join("/");
      const duplicated = paths.some((otherPath) => {
        if (otherPath === path) {
          return false;
        }
        const otherSegments = pathSegments(otherPath);
        return otherSegments.slice(-depth).join("/") === candidate;
      });
      if (!duplicated) {
        label = candidate;
        break;
      }
      label = candidate;
    }

    labels.set(path, label);
  }

  return labels;
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
    switchRepo,
  } = useAppContext();
  const projectLabels = useMemo(() => buildProjectLabels(state.repoNamespaces), [state.repoNamespaces]);
  const repoName = state.repoPath ? projectLabels.get(state.repoPath) ?? basename(state.repoPath) : "workspace";
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; moduleId: string } | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const settingsPanelRef = useRef<HTMLDivElement | null>(null);
  const settingsButtonRef = useRef<HTMLButtonElement | null>(null);
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
    const onPointerDown = (event: PointerEvent) => {
      setMenu(null);
      if (!settingsOpen) {
        return;
      }
      const target = event.target as Node | null;
      if (!target) {
        return;
      }
      if (settingsButtonRef.current?.contains(target) || settingsPanelRef.current?.contains(target)) {
        return;
      }
      setSettingsOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenu(null);
        setSettingsOpen(false);
      }
    };
    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [settingsOpen]);

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
        <img className="tabs-repo__logo" src={logoIcon} alt="fastgit logo" />
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

      <div className="tabs-actions app-no-drag-region">
        <select
          className="ui-input tabs-project-select"
          value={state.repoPath}
          title={state.repoPath || "未选择项目"}
          onChange={(event) => void switchRepo(event.target.value)}
        >
          <option value="">{state.repoNamespaces.length === 0 ? "未配置项目" : "选择项目"}</option>
          {state.repoNamespaces.map((repoPath) => (
            <option key={repoPath} value={repoPath}>
              {projectLabels.get(repoPath) ?? basename(repoPath)}
            </option>
          ))}
        </select>

        <button
          ref={settingsButtonRef}
          className="tabs-settings"
          onClick={() => setSettingsOpen((value) => !value)}
          type="button"
          aria-label="open project settings"
        >
          <Settings2 size={14} />
        </button>

        <button className="tabs-add" onClick={addModuleTab} type="button" aria-label="new tab">
          <Plus size={14} />
        </button>
      </div>

      {settingsOpen && (
        <div
          ref={settingsPanelRef}
          className="tabs-settings-panel app-no-drag-region"
          onPointerDown={(event) => event.stopPropagation()}
        >
          <RepoSwitcher />
        </div>
      )}

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
