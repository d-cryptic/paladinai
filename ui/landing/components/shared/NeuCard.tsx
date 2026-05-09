"use client";

import { motion, type HTMLMotionProps } from "framer-motion";
import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

type NeuCardProps = {
  children: ReactNode;
  className?: string;
  color?: string;
  shadowColor?: string;
  hoverable?: boolean;
  delay?: number;
} & Omit<HTMLMotionProps<"div">, "color">;

export function NeuCard({
  children,
  className,
  color,
  shadowColor,
  hoverable = true,
  delay = 0,
  style,
  ...rest
}: NeuCardProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 24 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-60px" }}
      transition={{ duration: 0.45, delay, ease: "easeOut" }}
      whileHover={
        hoverable
          ? {
              x: 2,
              y: 2,
              boxShadow: `3px 3px 0 ${shadowColor ?? "#111111"}`,
            }
          : undefined
      }
      style={{
        background: color ?? "#ffffff",
        border: "2.5px solid #111111",
        boxShadow: `5px 5px 0 ${shadowColor ?? "#111111"}`,
        borderRadius: 0,
        ...style,
      }}
      className={cn("relative", className)}
      {...rest}
    >
      {children}
    </motion.div>
  );
}

export default NeuCard;
