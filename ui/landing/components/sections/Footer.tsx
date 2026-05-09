import { Star, ExternalLink } from "lucide-react";

const GITHUB_URL = "https://github.com/d-cryptic/paladinai";

const COLUMNS = [
  {
    title: "PRODUCT",
    links: [
      { label: "Features", href: "#features" },
      { label: "How It Works", href: "#how-it-works" },
      { label: "Architecture", href: "#architecture" },
      { label: "Stack", href: "#stack" },
    ],
  },
  {
    title: "RESOURCES",
    links: [
      { label: "GitHub", href: GITHUB_URL },
      { label: "Documentation", href: `${GITHUB_URL}#readme` },
      { label: "Issues", href: `${GITHUB_URL}/issues` },
      { label: "Releases", href: `${GITHUB_URL}/releases` },
    ],
  },
  {
    title: "LEGAL",
    links: [
      { label: "License", href: `${GITHUB_URL}/blob/main/LICENSE` },
      { label: "Commons Clause", href: "https://commonsclause.com/" },
      { label: "Code of Conduct", href: `${GITHUB_URL}` },
      { label: "Security", href: `${GITHUB_URL}/security` },
    ],
  },
];

export function Footer() {
  return (
    <footer
      className="relative px-5 sm:px-8 pt-14 pb-8 overflow-hidden"
      style={{
        background: "#111111",
        borderTop: "3px solid #FFE600",
      }}
    >
      <div className="max-w-6xl mx-auto grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-10">
        {/* Brand */}
        <div className="sm:col-span-2 lg:col-span-1">
          <div className="flex items-center gap-2">
            <Star size={18} fill="#FF3131" stroke="#FF3131" aria-hidden />
            <span className="font-pixel text-white text-sm tracking-wider">
              PALADIN.AI
            </span>
          </div>
          <p className="font-mono text-white/70 mt-4 text-sm leading-relaxed">
            The AI that watches your systems. Open source incident response,
            powered by graphs and vectors.
          </p>
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 mt-5 font-pixel text-[10px] text-[#FFE600] hover:text-white transition-colors break-all"
          >
            <ExternalLink size={14} className="shrink-0" />
            <span className="break-all">github.com/d-cryptic/paladinai</span>
          </a>
        </div>

        {/* Link columns */}
        {COLUMNS.map((c) => (
          <div key={c.title}>
            <h3 className="font-pixel text-[#FFE600] text-xs tracking-wider">
              [ {c.title} ]
            </h3>
            <ul className="mt-5 space-y-3">
              {c.links.map((l) => (
                <li key={l.label}>
                  <a
                    href={l.href}
                    target={l.href.startsWith("http") ? "_blank" : undefined}
                    rel={l.href.startsWith("http") ? "noopener noreferrer" : undefined}
                    className="font-mono text-white/70 text-sm hover:text-[#00FF88] transition-colors"
                  >
                    {l.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>

      {/* Bottom bar */}
      <div className="max-w-6xl mx-auto mt-12 pt-6 flex flex-col sm:flex-row items-center justify-between gap-3 border-t-2 border-[#FFE600]/30">
        <p className="font-pixel text-[9px] text-white/60 tracking-wider text-center sm:text-left leading-relaxed">
          © 2025 PALADIN AI — APACHE 2.0 + COMMONS CLAUSE
        </p>
        <div className="flex items-center gap-4 font-pixel text-[9px] text-white/60">
          <span className="flex items-center gap-1.5">
            <span className="w-2 h-2 bg-[#00FF88] inline-block" /> ONLINE
          </span>
          <span>v0.1.0</span>
          <span className="text-[#FFE600]">★ PIXELS</span>
        </div>
      </div>
    </footer>
  );
}

export default Footer;
