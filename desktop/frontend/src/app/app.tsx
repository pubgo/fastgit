import { useEffect, useRef } from "react";

import { AppProvider } from "./providers/app-provider";
import { useAppContext } from "./providers/app-context";
import { DesktopLayout } from "../layouts/desktop-layout";

function AppBootstrap() {
  const { state, refresh } = useAppContext();
  const initialRepoPath = useRef(state.repoPath);

  useEffect(() => {
    void refresh(initialRepoPath.current);
  }, [refresh]);

  return <DesktopLayout />;
}

export function App() {
  return (
    <AppProvider>
      <AppBootstrap />
    </AppProvider>
  );
}
