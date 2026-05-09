"use client";

import { motion } from "framer-motion";
import { cn } from "@/lib/utils";
import type { ElementType } from "react";

type GlitchTextProps = {
  text: string;
  className?: string;
  as?: "h1" | "h2" | "h3" | "p" | "span";
  delay?: number;
};

export function GlitchText({
  text,
  className,
  as = "h1",
  delay = 0,
}: GlitchTextProps) {
  const Tag = motion[as as keyof typeof motion] as ElementType;

  return (
    <Tag
      initial={{ opacity: 0, y: 16 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-80px" }}
      transition={{ duration: 0.5, delay }}
      className={cn("font-pixel relative inline-block", className)}
    >
      <span className="glitch-text inline-block" data-text={text}>
        {text}
      </span>
    </Tag>
  );
}

export default GlitchText;
