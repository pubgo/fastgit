import { Alert, Input as AntInput, Modal, Select, Typography } from "antd";

import type { ModuleAction, ResourceCatalog } from "../../app/types";
import { Input } from "../../components/ui/input";
import { resolveActionFieldOptions } from "./action-fields";
import { actionSubmitLabel } from "./action-meta";

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

function hasMissingRequiredField(action: ModuleAction, values: Record<string, string>): boolean {
  return (action.fields ?? []).some((field) => {
    if (!field.required) {
      return false;
    }
    return !String(values[field.key] ?? "").trim();
  });
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
  const isForceSyncAction = action.id.endsWith("_force_sync");
  const isDeleteAction = action.id.endsWith("_delete") || action.id.endsWith("_remove") || action.id.endsWith("_close") || action.id.endsWith("_discard");
  const forceSyncHint =
    action.id === "tag_force_sync"
      ? "会执行 git fetch --force <remote> refs/tags/<tag>:refs/tags/<tag>，本地同名 tag 会被远端覆盖。"
      : "会执行 git fetch --prune <remote>、git reset --hard、git clean -fd，本地未提交改动会被直接丢弃。";
  const deleteHint =
    action.id === "remote_remove"
      ? "会移除当前 remote 配置；依赖它的 tracking/default 配置需要你后续自行调整。"
      : action.id === "repo_discard_path"
        ? "会丢弃该文件本地改动；如果是未跟踪文件会直接删除。"
      : "该操作不可恢复，请确认当前选中资源正确。";
  const missingRequired = hasMissingRequiredField(action, values);

  return (
    <Modal
      open
      title={action.title}
      onCancel={onClose}
      onOk={() => onSubmit(values)}
      okText={actionSubmitLabel(action)}
      cancelText="取消"
      confirmLoading={busy}
      okButtonProps={{ disabled: busy || missingRequired, danger: isDeleteAction || isForceSyncAction }}
      destroyOnClose
      centered
      width={560}
    >
      <div className="action-dialog__body">
        <Typography.Paragraph className="action-dialog__description">{action.description}</Typography.Paragraph>
        {isForceSyncAction ? <Alert type="warning" showIcon message="危险操作" description={forceSyncHint} /> : null}
        {isDeleteAction ? <Alert type="warning" showIcon message="删除确认" description={deleteHint} /> : null}
        <div className="action-dialog__fields">
          {(action.fields ?? []).map((field) => {
            const options = resolveActionFieldOptions(moduleId, action.id, field.key, catalog);
            const value = values[field.key] ?? "";
            return (
              <label key={field.key} className="action-field">
                <span>
                  {field.label}
                  {field.required ? " *" : ""}
                </span>
                {field.key.toLowerCase().includes("body") ? (
                  <AntInput.TextArea
                    className="ui-textarea"
                    placeholder={field.placeholder || field.label}
                    value={value}
                    autoSize={{ minRows: 4, maxRows: 12 }}
                    onChange={(event) =>
                      onChange({
                        ...values,
                        [field.key]: event.target.value,
                      })
                    }
                  />
                ) : options.length > 0 ? (
                  <Select
                    className="ui-select"
                    value={value ? value : undefined}
                    placeholder={field.placeholder || `选择${field.label}`}
                    allowClear={!field.required}
                    options={options.map((option) => ({
                      label: option.label,
                      value: option.value,
                    }))}
                    onChange={(nextValue) =>
                      onChange({
                        ...values,
                        [field.key]: String(nextValue ?? ""),
                      })
                    }
                  />
                ) : (
                  <Input
                    placeholder={field.placeholder || field.label}
                    value={value}
                    onChange={(event) =>
                      onChange({
                        ...values,
                        [field.key]: event.target.value,
                      })
                    }
                  />
                )}
              </label>
            );
          })}
        </div>
      </div>
    </Modal>
  );
}
