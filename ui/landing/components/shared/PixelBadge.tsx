import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

export type BadgeColor = "red" | "yellow" | "green" | "blue" | "purple" | "black" | "white";

const COLOR_MAP: Record<BadgeColor, { bg: string; text: string }> = {
  red: { bg: "#FF3131", text: "#ffffff" },
  yellow: { bg: "#FFE600", text: "#111111" },
  green: { bg: "#00FF88", text: "#111111" },
  blue: { bg: "#4DAAFF", text: "#111111" },
  purple: { bg: "#A855F7", text: "#ffffff" },
  black: { bg: "#111111", text: "#FFE600" },
  white: { bg: "#ffffff", text: "#111111" },
};

type PixelBadgeProps = {
  label: string;
  color?: BadgeColor;
  className?: string;
  icon?: ReactNode;
};

export function PixelBadge({ label, color = "yellow", className, icon }: PixelBadgeProps) {
  const c = COLOR_MAP[color];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-pixel text-[10px] leading-none uppercase tracking-wider",
        "px-2.5 py-2 select-none",
        className,
      )}
      style={{
        background: c.bg,
        color: c.text,
        border: "2.5px solid #111111",
        boxShadow: "3px 3px 0 #111111",
      }}
    >
      {icon ? <span aria-hidden>{icon}</span> : null}
      {label}
    </span>
  );
}

export default PixelBadge;
