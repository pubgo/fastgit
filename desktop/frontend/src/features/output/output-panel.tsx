import { useAppContext } from "../../app/providers/app-context";

export function OutputPanel() {
  const { state } = useAppContext();
  const hasList = Boolean(state.output.items && state.output.items.length > 0);

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
      {hasList ? (
        <ul className="output-list" role="list">
          {state.output.items?.map((item) => (
            <li key={item.id} className={item.active ? "output-list__item output-list__item--active" : "output-list__item"}>
              <div className="output-list__main">
                <p>{item.primary}</p>
                {item.secondary ? (
                  item.url ? (
                    <a href={item.url} target="_blank" rel="noreferrer">
                      {item.secondary}
                    </a>
                  ) : (
                    <span>{item.secondary}</span>
                  )
                ) : null}
              </div>
              {item.badge ? <span className="output-list__badge">{item.badge}</span> : null}
            </li>
          ))}
        </ul>
      ) : state.output.emptyHint ? (
        <div className="output-empty">{state.output.emptyHint}</div>
      ) : (
        <pre className="output-panel__body">{state.output.body}</pre>
      )}
    </section>
  );
}
