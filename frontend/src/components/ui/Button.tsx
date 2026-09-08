import type { ButtonHTMLAttributes, ReactNode } from "react";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "danger";
  size?: "sm";
  loading?: boolean;
  loadingText?: string;
  children: ReactNode;
}

export function Button({
  variant,
  size,
  loading,
  loadingText,
  children,
  className,
  disabled,
  ...rest
}: ButtonProps) {
  const classes = [
    "btn",
    variant && `btn-${variant}`,
    size && `btn-${size}`,
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button className={classes} disabled={disabled || loading} {...rest}>
      {loading ? (loadingText ?? children) : children}
    </button>
  );
}
