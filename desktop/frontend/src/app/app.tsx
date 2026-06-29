import { useEffect } from "react";

import { AppProvider } from "./providers/app-provider";
import { useAppContext } from "./providers/app-context";
import { DesktopLayout } from "../layouts/desktop-layout";

function AppBootstrap() {
  const { refresh } = useAppContext();

  useEffect(() => {
    void refresh();
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
