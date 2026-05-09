"use client";

import { useEffect } from "react";

// Only active in development — tree-shaken in production builds.
export function ReactScanInit() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "development") return;
    import("react-scan").then(({ scan }) => {
      scan({ enabled: true, log: false });
    });
  }, []);
  return null;
}
