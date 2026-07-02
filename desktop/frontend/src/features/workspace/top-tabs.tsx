import { Suspense, lazy, useEffect, useMemo, useRef, useState } from "react";
import type { DragEvent, MouseEvent } from "react";
import { Input as AntInput, Select } from "antd";
import { Plus, Settings2, X } from "lucide-react";

import { useAppContext } from "../../app/providers/app-context";
import logoIcon from "../../assets/brand/fastgit-logo-icon.svg";
import type { ModuleAction } from "../../app/types";

const RepoSwitcher = lazy(async () => {
  const module = await import("./repo-switcher");
  return { default: module.RepoSwitcher };
});

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

function findLandingAction(actions: ModuleAction[]): ModuleAction | null {
  return actions.find((action) => (action.id.endsWith("_list") || action.id.endsWith("_status")) && (action.fields?.length ?? 0) === 0) ?? null;
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
    runAction,
  } = useAppContext();
  const projectLabels = useMemo(() => buildProjectLabels(state.repoNamespaces), [state.repoNamespaces]);
  const repoName = state.repoPath ? projectLabels.get(state.repoPath) ?? basename(state.repoPath) : "workspace";
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; moduleId: string } | null>(null);
  const [repoPickerOpen, setRepoPickerOpen] = useState(false);
  const [repoQuery, setRepoQuery] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const settingsPanelRef = useRef<HTMLDivElement | null>(null);
  const settingsButtonRef = useRef<HTMLButtonElement | null>(null);
  const repoPickerRef = useRef<HTMLDivElement | null>(null);
  const repoButtonRef = useRef<HTMLButtonElement | null>(null);
  const openedModules = state.openedModuleIds
    .map((id) => state.modules.find((module) => module.id === id))
    .filter((module): module is NonNullable<typeof module> => module !== undefined);
  const openedCount = openedModules.length;
  const currentBaseBranch = state.projectSettings[state.repoPath]?.defaultBaseBranch ?? "";
  const currentDefaultRemote = state.projectSettings[state.repoPath]?.defaultRemote ?? "";
  const githubSummary = state.githubAuthStatus?.configured
    ? `GitHub 已连接 · ${state.githubAuthStatus.source === "session" ? "会话 Token" : "环境变量"}`
    : "GitHub 未连接";
  const filteredRepos = useMemo(() => {
    const keyword = repoQuery.trim().toLowerCase();
    if (!keyword) {
      return state.repoNamespaces;
    }
    return state.repoNamespaces.filter((repoPath) => {
      const label = projectLabels.get(repoPath) ?? basename(repoPath);
      return repoPath.toLowerCase().includes(keyword) || label.toLowerCase().includes(keyword);
    });
  }, [projectLabels, repoQuery, state.repoNamespaces]);
  const repoOptions = useMemo(
    () =>
      state.repoNamespaces.map((repoPath) => ({
        value: repoPath,
        label: projectLabels.get(repoPath) ?? basename(repoPath),
      })),
    [projectLabels, state.repoNamespaces]
  );

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
        if (!repoPickerOpen) {
          return;
        }
      }
      const target = event.target as Node | null;
      if (!target) {
        return;
      }
      if (repoButtonRef.current?.contains(target) || repoPickerRef.current?.contains(target)) {
        return;
      }
      if (settingsButtonRef.current?.contains(target) || settingsPanelRef.current?.contains(target)) {
        return;
      }
      setRepoPickerOpen(false);
      setSettingsOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenu(null);
        setRepoPickerOpen(false);
        setSettingsOpen(false);
      }
    };
    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [repoPickerOpen, settingsOpen]);

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

  const onModuleTabSelect = (moduleId: string) => {
    const module = openedModules.find((item) => item.id === moduleId);
    if (!module) {
      return;
    }
    setSelectedModule(module.id);
    const landingAction = findLandingAction(module.actions);
    if (landingAction) {
      void runAction(module, landingAction, {});
    }
  };

  return (
    <header className="tabs-header app-drag-region">
      <div className="tabs-repo app-no-drag-region">
        <button
          ref={repoButtonRef}
          type="button"
          className="tabs-repo__button"
          onClick={() => {
            setRepoPickerOpen((value) => !value);
            setRepoQuery("");
          }}
          aria-label="open project switcher"
        >
          <img className="tabs-repo__logo" src={logoIcon} alt="fastgit logo" />
          <span>{repoName}</span>
          {currentBaseBranch ? <small>{currentBaseBranch}</small> : null}
        </button>
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
              onClick={() => onModuleTabSelect(module.id)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onModuleTabSelect(module.id);
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
        <Select
          className="tabs-project-select"
          value={state.repoPath || undefined}
          options={repoOptions}
          placeholder={state.repoNamespaces.length === 0 ? "未配置项目" : "选择项目"}
          allowClear
          showSearch
          optionFilterProp="label"
          onChange={(next) => void switchRepo(String(next ?? ""))}
        />

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
          <Suspense fallback={<div className="tabs-project-panel__empty">加载设置...</div>}>
            <RepoSwitcher />
          </Suspense>
        </div>
      )}

      {repoPickerOpen && (
        <div
          ref={repoPickerRef}
          className="tabs-project-panel app-no-drag-region"
          onPointerDown={(event) => event.stopPropagation()}
        >
          <div className="tabs-project-panel__header">
            <strong>快速切换项目</strong>
            <span>{filteredRepos.length} / {state.repoNamespaces.length}</span>
          </div>
          <div className="tabs-project-panel__summary">
            <strong>{repoName}</strong>
            <span>{state.repoPath || "未选择项目"}</span>
            <div className="tabs-project-panel__summary-meta">
              <span>{currentBaseBranch ? `base ${currentBaseBranch}` : "未设置默认 base"}</span>
              <span>{currentDefaultRemote ? `remote ${currentDefaultRemote}` : "未设置默认 remote"}</span>
              <span>{githubSummary}</span>
            </div>
            <div className="tabs-project-panel__summary-actions">
              <button
                type="button"
                className="tabs-project-panel__summary-button"
                onClick={() => {
                  setRepoPickerOpen(false);
                  setSettingsOpen(true);
                }}
              >
                打开项目设置
              </button>
            </div>
          </div>
          <AntInput
            className="tabs-project-panel__search"
            value={repoQuery}
            onChange={(event) => setRepoQuery(event.target.value)}
            placeholder="搜索项目..."
            autoFocus
          />
          <div className="tabs-project-panel__list">
            {filteredRepos.map((repoPath) => {
              const active = repoPath === state.repoPath;
              const baseBranch = state.projectSettings[repoPath]?.defaultBaseBranch ?? "";
              const defaultRemote = state.projectSettings[repoPath]?.defaultRemote ?? "";
              return (
                <button
                  key={repoPath}
                  type="button"
                  className={active ? "tabs-project-item tabs-project-item--active" : "tabs-project-item"}
                  onClick={() => {
                    void switchRepo(repoPath);
                    setRepoPickerOpen(false);
                    setRepoQuery("");
                  }}
                >
                  <span className="tabs-project-item__title">{projectLabels.get(repoPath) ?? basename(repoPath)}</span>
                  <span className="tabs-project-item__path">{repoPath}</span>
                  <span className="tabs-project-item__meta">
                    {active ? "当前项目" : "切换到该项目"}
                    {baseBranch ? ` · base ${baseBranch}` : ""}
                    {defaultRemote ? ` · remote ${defaultRemote}` : ""}
                  </span>
                </button>
              );
            })}
            {filteredRepos.length === 0 ? <div className="tabs-project-panel__empty">没有匹配的项目</div> : null}
          </div>
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
