import { useEffect, useRef } from "react";
import { ConfigProvider } from "antd";

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
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#4f7fc4",
          colorInfo: "#4f7fc4",
          borderRadius: 10,
          fontFamily: '"Manrope", "Avenir Next", sans-serif',
        },
        components: {
          Button: { controlHeight: 36 },
          Input: { controlHeight: 36 },
          Select: { controlHeight: 36 },
          Modal: { borderRadiusLG: 14 },
        },
      }}
    >
      <AppProvider>
        <AppBootstrap />
      </AppProvider>
    </ConfigProvider>
  );
}
