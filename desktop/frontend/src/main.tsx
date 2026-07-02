import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "antd/dist/reset.css";
import "./styles/index.css";

const container = document.getElementById("app");
if (!container) {
  throw new Error("#app not found");
}

const root = createRoot(container);

function renderFatal(error: unknown): void {
  const message = error instanceof Error ? `${error.name}: ${error.message}` : String(error);
  const stack = error instanceof Error && error.stack ? `\n\n${error.stack}` : "";
  root.render(
    <div
      style={{
        height: "100%",
        padding: "16px",
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
        whiteSpace: "pre-wrap",
        color: "#10203a",
        background: "#f2f6fc",
      }}
    >
      {`启动失败\n${message}${stack}`}
    </div>
  );
}

async function bootstrap() {
  try {
    const { App } = await import("./app/app");
    root.render(
      <StrictMode>
        <App />
      </StrictMode>
    );
  } catch (error) {
    console.error("[fastgit] bootstrap failed", error);
    renderFatal(error);
  }
}

void bootstrap();
