import type { SVGProps } from "react";

interface PaladinLogoProps extends SVGProps<SVGSVGElement> {
  size?: number;
  variant?: "color" | "mono-white" | "mono-black";
}

export function PaladinLogoMark({
  size = 40,
  variant = "color",
  ...props
}: PaladinLogoProps) {
  const isColor = variant === "color";
  const isWhite = variant === "mono-white";

  const shieldFill  = isColor ? "#FF3131" : isWhite ? "#ffffff" : "#111111";
  const bladeDark   = isColor ? "#8B6200" : isWhite ? "#888888" : "#333333";
  const bladeMid    = isColor ? "#FFB800" : isWhite ? "#bbbbbb" : "#666666";
  const bladeMain   = isColor ? "#FFE600" : isWhite ? "#dddddd" : "#888888";
  const bladeHi     = isColor ? "#FFF8A0" : isWhite ? "#ffffff" : "#aaaaaa";
  const bladeTip    = isColor ? "#ffffff" : isWhite ? "#ffffff" : "#cccccc";
  const guardColor  = isColor ? "#FFE600" : isWhite ? "#cccccc" : "#555555";
  const guardDark   = isColor ? "#B8A000" : isWhite ? "#999999" : "#333333";
  const eyeOuter   = isColor ? "#ffffff" : isWhite ? "#111111" : "#ffffff";
  const eyeIris    = isColor ? "#4DAAFF" : isWhite ? "#888888" : "#666666";
  const eyePupil   = isColor ? "#111111" : isWhite ? "#ffffff" : "#111111";
  const eyeGlint   = isColor ? "#00FF88" : isWhite ? "#aaaaaa" : "#888888";
  const nodeColor  = isColor ? "#00FF88" : isWhite ? "#aaaaaa" : "#666666";
  const ink        = isWhite ? "#ffffff" : "#111111";
  const sw         = size < 28 ? 1.5 : 2.5;

  return (
    <svg
      width={size}
      height={Math.round(size * 1.18)}
      viewBox="0 0 50 59"
      fill="none"
      style={{ imageRendering: "pixelated" }}
      aria-label="Paladin AI logo mark"
      {...props}
    >
      {/* ── Drop shadow ── */}
      <path
        d="M5 3 L45 3 L49 8 L49 33 C49 46 25 56 25 56 C25 56 1 46 1 33 L1 8 Z"
        fill={ink}
        transform="translate(3 3)"
        opacity={isWhite ? 0 : 0.9}
      />

      {/* ── Shield body ── */}
      <path
        d="M5 3 L45 3 L49 8 L49 33 C49 46 25 56 25 56 C25 56 1 46 1 33 L1 8 Z"
        fill={shieldFill}
        stroke={ink}
        strokeWidth={sw}
        strokeLinejoin="miter"
      />

      {/* ── Inner shield bevel ── */}
      <path
        d="M9 7 L41 7 L44 11 L44 32 C44 43 25 52 25 52 C25 52 6 43 6 32 L6 11 Z"
        fill="none"
        stroke={isColor ? "#FF6B6B" : bladeHi}
        strokeWidth={1.5}
        opacity={0.35}
      />

      {/* ══════════════════════════════════
          PIXEL-ART SWORD BLADE
          Layers: dark silhouette → warm base → bright face → highlight streak → tip flash
          ══════════════════════════════════ */}

      {/* Layer 1 — dark outer silhouette (widest) */}
      <polygon
        points="25,0 19,26 31,26"
        fill={bladeDark}
        stroke={ink}
        strokeWidth={1.5}
        strokeLinejoin="miter"
      />

      {/* Layer 2 — warm mid tone (right half) */}
      <polygon points="25,1 25,25.5 30,25.5" fill={bladeMid} />

      {/* Layer 3 — bright yellow face (right of centre) */}
      <polygon points="25,1 26,25.5 30,25.5" fill={bladeMain} />

      {/* Layer 4 — highlight streak (sharp edge, right side) */}
      <polygon points="25,0.5 27,25.5 29,25.5" fill={bladeHi} opacity={0.9} />

      {/* Layer 5 — hot-white tip flash (top 6px) */}
      <polygon points="25,0 24,6 26,6" fill={bladeTip} opacity={0.95} />

      {/* Layer 6 — left dark face (adds depth) */}
      <polygon points="25,1 20,25.5 24,25.5" fill={bladeDark} opacity={0.55} />

      {/* ── Crossguard ── */}
      {/* shadow */}
      <rect x={10} y={28} width={30} height={8} fill={ink} opacity={0.5} />
      {/* main */}
      <rect x={10} y={26} width={30} height={8} fill={guardColor} stroke={ink} strokeWidth={sw} />
      {/* top bevel highlight */}
      <rect x={11} y={27} width={28} height={2} fill={bladeHi} opacity={0.5} />
      {/* bottom shadow */}
      <rect x={11} y={31} width={28} height={2} fill={guardDark} opacity={0.6} />
      {/* guard notch at blade junction */}
      <rect x={23} y={26} width={4} height={8} fill={guardDark} opacity={0.4} />

      {/* ── Eye / orb at crossguard centre ── */}
      {/* outer glow ring */}
      <ellipse cx={25} cy={30} rx={8} ry={5} fill={eyeOuter} opacity={0.2} />
      {/* sclera */}
      <ellipse cx={25} cy={30} rx={6.5} ry={4} fill={eyeOuter} stroke={ink} strokeWidth={1.2} />
      {/* iris */}
      <ellipse cx={25} cy={30} rx={3.5} ry={2.8} fill={eyeIris} />
      {/* pupil */}
      <circle cx={25} cy={30} r={1.8} fill={eyePupil} />
      {/* glint */}
      <circle cx={26.2} cy={28.9} r={0.9} fill={eyeGlint} />

      {/* ── Circuit nodes on guard ends ── */}
      <rect x={6}  y={27} width={6} height={6} fill={nodeColor} stroke={ink} strokeWidth={1} />
      <rect x={38} y={27} width={6} height={6} fill={nodeColor} stroke={ink} strokeWidth={1} />
      {/* node inner detail */}
      <rect x={7.5}  y={28.5} width={3} height={3} fill={ink} opacity={0.4} />
      <rect x={39.5} y={28.5} width={3} height={3} fill={ink} opacity={0.4} />

      {/* ── Sword handle ── */}
      {/* grip wrap pattern */}
      <rect x={23} y={34} width={4} height={13} fill={guardDark} stroke={ink} strokeWidth={sw - 0.5} />
      <rect x={23} y={35} width={4} height={2}  fill={bladeMain} opacity={0.5} />
      <rect x={23} y={39} width={4} height={2}  fill={bladeMain} opacity={0.5} />
      <rect x={23} y={43} width={4} height={2}  fill={bladeMain} opacity={0.5} />

      {/* ── Pommel ── */}
      <rect x={20} y={47} width={10} height={7} fill={guardColor} stroke={ink} strokeWidth={sw} />
      <rect x={21} y={48} width={8}  height={2} fill={bladeHi} opacity={0.5} />
    </svg>
  );
}

export function PaladinWordmark({
  size = 36,
  variant = "color",
  className,
}: {
  size?: number;
  variant?: PaladinLogoProps["variant"];
  className?: string;
}) {
  const textColor = variant === "mono-white" ? "#ffffff" : "#111111";
  return (
    <span className={`inline-flex items-center gap-2 ${className ?? ""}`}>
      <PaladinLogoMark size={size} variant={variant} />
      <span
        style={{
          fontFamily: '"Press Start 2P", monospace',
          fontSize: Math.max(10, size * 0.3),
          letterSpacing: "0.05em",
          color: textColor,
          lineHeight: 1,
        }}
      >
        PALADIN.AI
      </span>
    </span>
  );
}

export default PaladinLogoMark;
