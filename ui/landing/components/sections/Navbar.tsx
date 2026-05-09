"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Menu, X } from "lucide-react";
import { PaladinLogoMark } from "@/components/shared/PaladinLogo";

const NAV_LINKS = [
  { label: "Features", href: "#features" },
  { label: "How It Works", href: "#how-it-works" },
  { label: "Architecture", href: "#architecture" },
  { label: "Stack", href: "#stack" },
];

const GITHUB_URL = "https://github.com/d-cryptic/paladinai";

export function Navbar() {
  const [open, setOpen] = useState(false);

  return (
    <header
      className="fixed top-0 left-0 right-0 z-50"
      style={{
        background: "#111111",
        borderBottom: "3px solid #111111",
        boxShadow: "0 4px 0 #111111",
      }}
    >
      <div className="max-w-7xl mx-auto px-5 sm:px-8 h-16 flex items-center justify-between">
        {/* Logo */}
        <a href="#" className="flex items-center gap-2.5 hover:opacity-90 transition-opacity">
          <PaladinLogoMark size={32} variant="color" />
          <span className="font-pixel text-white text-xs sm:text-sm tracking-wider">
            PALADIN.AI
          </span>
        </a>

        {/* Desktop nav */}
        <nav className="hidden md:flex items-center gap-7">
          {NAV_LINKS.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className="font-pixel text-[10px] uppercase tracking-wider text-white hover:text-[#FFE600] transition-colors"
            >
              {l.label}
            </a>
          ))}
        </nav>

        {/* Right CTA */}
        <div className="hidden md:block">
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="neu-btn neu-btn-primary !py-2 !px-3 !text-[10px]"
          >
            [GITHUB]
          </a>
        </div>

        {/* Mobile menu button */}
        <button
          type="button"
          className="md:hidden text-white p-2"
          aria-label="Toggle menu"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
        >
          {open ? <X size={22} /> : <Menu size={22} />}
        </button>
      </div>

      <AnimatePresence>
        {open && (
          <motion.div
            key="mobile-menu"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.25 }}
            className="md:hidden overflow-hidden"
            style={{ background: "#111111", borderTop: "2px solid #FFE600" }}
          >
            <nav className="flex flex-col px-6 py-5 gap-4">
              {NAV_LINKS.map((l) => (
                <a
                  key={l.href}
                  href={l.href}
                  onClick={() => setOpen(false)}
                  className="font-pixel text-[11px] uppercase text-white hover:text-[#FFE600]"
                >
                  {l.label}
                </a>
              ))}
              <a
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="neu-btn neu-btn-primary self-start !text-[10px]"
              >
                [GITHUB]
              </a>
            </nav>
          </motion.div>
        )}
      </AnimatePresence>
    </header>
  );
}

export default Navbar;
