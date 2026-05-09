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
  const bladeFill   = isColor ? "#FFE600" : isWhite ? "#cccccc" : "#444444";
  const eyeFill     = isColor ? "#ffffff" : isWhite ? "#111111" : "#ffffff";
  const circuitDot  = isColor ? "#00FF88" : isWhite ? "#aaaaaa" : "#666666";
  const ink         = isColor || !isWhite ? "#111111" : "#ffffff";
  const sw          = size < 28 ? 1.5 : 2.5;

  return (
    <svg
      width={size}
      height={Math.round(size * 1.18)}
      viewBox="0 0 50 59"
      fill="none"
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
        stroke={bladeFill}
        strokeWidth={1.5}
        opacity={0.4}
      />

      {/* ── Sword blade (upper) ── */}
      <rect
        x={22} y={4} width={6} height={23}
        fill={bladeFill}
        stroke={ink}
        strokeWidth={sw - 0.5}
      />
      {/* blade tip (triangle top) */}
      <polygon
        points="25,0 22,4 28,4"
        fill={bladeFill}
        stroke={ink}
        strokeWidth={sw - 0.5}
        strokeLinejoin="miter"
      />

      {/* ── Guard / crossbar ── */}
      <rect
        x={10} y={26} width={30} height={8}
        fill={bladeFill}
        stroke={ink}
        strokeWidth={sw}
      />

      {/* ── Eye in the guard centre ── */}
      <ellipse cx={25} cy={30} rx={7} ry={4}
        fill={eyeFill} stroke={ink} strokeWidth={1.5} />
      {/* pupil */}
      <circle cx={25} cy={30} r={2.5} fill={ink} />
      {/* iris glint */}
      <circle cx={26.2} cy={28.9} r={0.9} fill={circuitDot} />

      {/* ── Sword handle (below guard) ── */}
      <rect
        x={23} y={34} width={4} height={14}
        fill={bladeFill}
        stroke={ink}
        strokeWidth={sw - 0.5}
      />

      {/* ── Pommel ── */}
      <rect
        x={20} y={47} width={10} height={7}
        fill={bladeFill}
        stroke={ink}
        strokeWidth={sw}
      />

      {/* ── Circuit nodes on guard ends ── */}
      <rect x={7}  y={27} width={5} height={5} fill={circuitDot} stroke={ink} strokeWidth={1} />
      <rect x={38} y={27} width={5} height={5} fill={circuitDot} stroke={ink} strokeWidth={1} />
    </svg>
  );
}

/** Full wordmark: logo mark + "PALADIN.AI" text */
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
