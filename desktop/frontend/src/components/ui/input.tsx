import type { InputHTMLAttributes } from "react";
import { Input as AntInput } from "antd";

import { cn } from "../../lib/classnames";

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <AntInput className={cn("ui-input", className)} {...props} />;
}
