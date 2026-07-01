import { FormEvent } from "react";

import type { ModuleAction, ResourceCatalog } from "../../app/types";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { resolveActionFieldOptions } from "./action-fields";

interface ActionDialogProps {
  action: ModuleAction;
  moduleId: string;
  catalog: ResourceCatalog;
  values: Record<string, string>;
  busy?: boolean;
  onChange(next: Record<string, string>): void;
  onClose(): void;
  onSubmit(values: Record<string, string>): void;
}

export function ActionDialog({
  action,
  moduleId,
  catalog,
  values,
  busy = false,
  onChange,
  onClose,
  onSubmit,
}: ActionDialogProps) {
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit(values);
  };

  return (
    <div className="action-dialog__backdrop" onClick={onClose}>
      <section
        className="action-dialog"
        role="dialog"
        aria-modal="true"
        aria-label={action.title}
        onClick={(event) => event.stopPropagation()}
      >
        <header className="action-dialog__header">
          <div>
            <h3>{action.title}</h3>
            <p>{action.description}</p>
          </div>
          <button type="button" className="action-dialog__close" onClick={onClose} aria-label="close dialog">
            ×
          </button>
        </header>
        <form onSubmit={handleSubmit} className="action-dialog__form">
          <div className="action-dialog__fields">
            {(action.fields ?? []).map((field) => (
              <label key={field.key} className="action-field">
                <span>{field.label}</span>
                {field.key.toLowerCase().includes("body") ? (
                  <textarea
                    className="ui-textarea"
                    name={field.key}
                    placeholder={field.placeholder || field.label}
                    value={values[field.key] ?? ""}
                    required={Boolean(field.required)}
                    onChange={(event) =>
                      onChange({
                        ...values,
                        [field.key]: event.target.value,
                      })
                    }
                  />
                ) : (() => {
                    const options = resolveActionFieldOptions(moduleId, action.id, field.key, catalog);
                    if (options.length > 0) {
                      return (
                        <select
                          className="ui-input ui-select"
                          name={field.key}
                          value={values[field.key] ?? ""}
                          required={Boolean(field.required)}
                          onChange={(event) =>
                            onChange({
                              ...values,
                              [field.key]: event.target.value,
                            })
                          }
                        >
                          <option value="">{field.placeholder || `选择${field.label}`}</option>
                          {options.map((option) => (
                            <option key={`${field.key}-${option.value}`} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      );
                    }

                    return (
                      <Input
                        name={field.key}
                        placeholder={field.placeholder || field.label}
                        value={values[field.key] ?? ""}
                        required={Boolean(field.required)}
                        onChange={(event) =>
                          onChange({
                            ...values,
                            [field.key]: event.target.value,
                          })
                        }
                      />
                    );
                  })()}
              </label>
            ))}
          </div>
          <footer className="action-dialog__footer">
            <Button type="button" variant="ghost" onClick={onClose} disabled={busy}>
              取消
            </Button>
            <Button type="submit" variant="primary" disabled={busy}>
              执行
            </Button>
          </footer>
        </form>
      </section>
    </div>
  );
}
