import { FormEvent } from "react";

import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useAppContext } from "../../app/providers/app-context";

function collectValues(form: HTMLFormElement): Record<string, string> {
  const payload = new FormData(form);
  const values: Record<string, string> = {};

  for (const [key, raw] of payload.entries()) {
    values[key] = String(raw).trim();
  }
  return values;
}

export function ModuleActions() {
  const { selectedModule, runAction } = useAppContext();

  if (!selectedModule) {
    return (
      <section className="module-actions">
        <div className="empty-state">未找到模块，请刷新。</div>
      </section>
    );
  }

  const actions = selectedModule.actions ?? [];

  const onSubmit = (action: (typeof actions)[number]) => (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const values = collectValues(event.currentTarget);
    void runAction(selectedModule, action, values);
  };

  return (
    <section className="module-actions">
      <header className="module-actions__header">
        <h2>{selectedModule.title}</h2>
        <p>{selectedModule.description}</p>
      </header>

      <div className="module-actions__grid">
        {actions.map((action, index) => (
          <article key={action.id} className="action-card" style={{ animationDelay: `${index * 60}ms` }}>
            <div className="action-card__meta">
              <h3>{action.title}</h3>
              <p>{action.description}</p>
            </div>
            <form onSubmit={onSubmit(action)} className="action-card__form">
              <div className="action-card__fields">
                {(action.fields ?? []).map((field) => (
                  <label key={field.key} className="action-field">
                    <span>{field.label}</span>
                    <Input
                      name={field.key}
                      placeholder={field.placeholder || field.label}
                      defaultValue={field.default || ""}
                      required={Boolean(field.required)}
                    />
                  </label>
                ))}
              </div>
              <div className="action-card__footer">
                <Button type="submit">执行</Button>
              </div>
            </form>
          </article>
        ))}
      </div>
    </section>
  );
}
