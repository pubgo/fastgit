import { useState } from "react";

import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useAppContext } from "../../app/providers/app-context";

export function RepoSwitcher() {
  const { state, switchRepo } = useAppContext();
  const [draft, setDraft] = useState("");

  const value = draft || state.repoPath;

  return (
    <section className="workspace-panel">
      <header>
        <h2>Workspace</h2>
        <p>{state.busy ? "运行中..." : "准备就绪"}</p>
      </header>
      <div className="workspace-panel__row">
        <Input
          value={value}
          onChange={(event) => setDraft(event.target.value)}
          placeholder="/path/to/repository"
        />
        <Button variant="primary" onClick={() => void switchRepo(value)}>
          切换仓库
        </Button>
      </div>
      <p className="workspace-panel__status">{state.repoStatus}</p>
    </section>
  );
}
