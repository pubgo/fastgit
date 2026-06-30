import { useState } from "react";
import type { CSSProperties, MouseEvent } from "react";

import { useAppContext } from "../app/providers/app-context";
import { ModuleSidebar } from "../features/modules/module-sidebar";
import { SidebarMenu } from "../features/modules/sidebar-menu";
import { ModuleActions } from "../features/modules/module-actions";
import { OutputPanel } from "../features/output/output-panel";
import { TopTabs } from "../features/workspace/top-tabs";

export function DesktopLayout() {
  const { state, setModulePaneWidth } = useAppContext();
  const [isDragging, setIsDragging] = useState(false);

  const onResizeStart = (event: MouseEvent<HTMLDivElement>) => {
    if (window.innerWidth <= 900 || state.modulePaneCollapsed) {
      return;
    }

    setIsDragging(true);
    const startX = event.clientX;
    const startWidth = state.modulePaneWidth;

    const onMove = (moveEvent: globalThis.MouseEvent) => {
      const delta = moveEvent.clientX - startX;
      setModulePaneWidth(startWidth + delta);
    };

    const onUp = () => {
      setIsDragging(false);
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };

    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const bodyStyle: CSSProperties = {
    "--module-pane-width": `${state.modulePaneWidth}px`,
  } as CSSProperties;

  return (
    <main className="desktop-shell">
      <TopTabs />
      <section
        className={[
          "desktop-body",
          isDragging ? "desktop-body--dragging" : "",
          state.modulePaneCollapsed ? "desktop-body--pane-collapsed" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        style={bodyStyle}
      >
        <SidebarMenu />
        <div className="module-pane-wrap">
          <ModuleSidebar />
          <div
            className="module-pane-resizer"
            onMouseDown={onResizeStart}
            role="separator"
            aria-orientation="vertical"
            aria-label="resize sidebar"
          />
        </div>
        <div className="desktop-main">
          <ModuleActions />
          <OutputPanel />
        </div>
      </section>
    </main>
  );
}
