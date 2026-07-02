import type { ButtonHTMLAttributes, PropsWithChildren } from "react";
import { Button as AntButton } from "antd";
import type { ButtonProps as AntButtonProps } from "antd";

import { cn } from "../../lib/classnames";

type Variant = "default" | "primary" | "ghost";

interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "type"> {
  variant?: Variant;
  type?: "button" | "submit" | "reset";
}

function mapVariant(variant: Variant): Pick<AntButtonProps, "type" | "ghost"> {
  if (variant === "primary") {
    return { type: "primary", ghost: false };
  }
  if (variant === "ghost") {
    return { type: "default", ghost: true };
  }
  return { type: "default", ghost: false };
}

export function Button({ variant = "default", className, type = "button", children, ...props }: PropsWithChildren<ButtonProps>) {
  const mapped = mapVariant(variant);
  return (
    <AntButton
      type={mapped.type}
      ghost={mapped.ghost}
      htmlType={type}
      className={cn("ui-button", `ui-button--${variant}`, className)}
      {...props}
    >
      {children}
    </AntButton>
  );
}
