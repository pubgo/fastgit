import type { ButtonHTMLAttributes, PropsWithChildren } from "react";

import { cn } from "../../lib/classnames";

type Variant = "default" | "primary" | "ghost";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

export function Button({ variant = "default", className, children, ...props }: PropsWithChildren<ButtonProps>) {
  return (
    <button
      className={cn("ui-button", `ui-button--${variant}`, className)}
      {...props}
    >
      {children}
    </button>
  );
}
