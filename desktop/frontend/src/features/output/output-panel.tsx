import { useAppContext } from "../../app/providers/app-context";

export function OutputPanel() {
  const { state } = useAppContext();

  return (
    <section className="output-panel">
      <header className="output-panel__header">
        <div>
          <h2>{state.output.title}</h2>
          <p>{state.output.command}</p>
        </div>
        <span className={state.output.exitCode === 0 ? "status-pill status-pill--ok" : "status-pill status-pill--error"}>
          exit {state.output.exitCode}
        </span>
      </header>
      <pre className="output-panel__body">{state.output.body}</pre>
    </section>
  );
}
