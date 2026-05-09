"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { motion, useAnimation, AnimatePresence } from "framer-motion";

/* ─── Action pool ────────────────────────────────────────────── */
const ACTIONS = [
  { id: "slash",    text: "SLASH!",             color: "#FF3131", pose: "attack"  },
  { id: "defend",   text: "SHIELD UP!",          color: "#4DAAFF", pose: "block"   },
  { id: "debug",    text: "FIXING BUG...",       color: "#00FF88", pose: "idle"    },
  { id: "alert",    text: "ALERT RESOLVED!",     color: "#FFE600", pose: "victory" },
  { id: "scale",    text: "SCALING PODS!",       color: "#A855F7", pose: "attack"  },
  { id: "deploy",   text: "DEPLOYING v2.1!",     color: "#FF7A00", pose: "victory" },
  { id: "monitor",  text: "MONITORING...",        color: "#4DAAFF", pose: "idle"    },
  { id: "rollback", text: "ROLLBACK!",            color: "#FF3131", pose: "attack"  },
  { id: "incident", text: "INCIDENT\nCONTAINED!",color: "#00FF88", pose: "victory" },
  { id: "patch",    text: "PATCH APPLIED!",       color: "#FFE600", pose: "block"   },
  { id: "reboot",   text: "REBOOTING SRV...",    color: "#A855F7", pose: "idle"    },
  { id: "victory",  text: "VICTORY!",             color: "#FFE600", pose: "victory" },
] as const;

type ActionId   = typeof ACTIONS[number]["id"];
type PoseId     = "idle" | "attack" | "block" | "victory" | "walk";

/* ─── Bug "enemy" type ───────────────────────────────────────── */
interface Bug {
  id: number;
  x: number;
  label: string;
  color: string;
  dying: boolean;
}

const BUG_LABELS = ["ERR 503", "OOM", "SEGFAULT", "DEADLOCK", "TIMEOUT", "NullPtr", "404", "PANIC"];
const BUG_COLORS = ["#FF3131", "#FF7A00", "#A855F7", "#FF3131", "#FF7A00"];

/* ─── Pixel-art paladin sprite ───────────────────────────────── */
function PaladinSprite({ pose, facing }: { pose: PoseId; facing: 1 | -1 }) {
  const attacking = pose === "attack";
  const blocking  = pose === "block";
  const victory   = pose === "victory";

  // sword tip position offset
  const swordTY = attacking ? -18 : victory ? -12 : 0;
  const swordRot = attacking ? -40 : victory ? -20 : 0;
  // shield push
  const shieldTX = blocking ? -6 : 0;

  // walking leg oscillation is handled by parent
  return (
    <svg
      width={96}
      height={148}
      viewBox="0 0 96 148"
      fill="none"
      style={{ imageRendering: "pixelated", overflow: "visible" }}
    >
      {/* ── Shadow ── */}
      <ellipse cx={48} cy={144} rx={32} ry={6} fill="rgba(0,0,0,0.35)" />

      {/* ── Helmet dome ── */}
      <rect x={28} y={4}  width={40} height={6}  fill="#FFE600" stroke="#111" strokeWidth={2} />
      <rect x={22} y={10} width={52} height={24} fill="#cccccc" stroke="#111" strokeWidth={2} />
      {/* side panels */}
      <rect x={22} y={14} width={10} height={16} fill="#FF3131" stroke="#111" strokeWidth={1.5} />
      <rect x={64} y={14} width={10} height={16} fill="#FF3131" stroke="#111" strokeWidth={1.5} />
      {/* visor */}
      <rect x={32} y={14} width={32} height={14} fill="#0a0a1e" stroke="#111" strokeWidth={1.5} />
      <rect x={34} y={16} width={28} height={3}  fill="#4DAAFF" opacity={0.9} />
      <rect x={34} y={21} width={28} height={3}  fill="#00FF88" opacity={0.7} />
      {/* eyes */}
      <rect x={36} y={17} width={5} height={5} fill="#00FF88" />
      <rect x={55} y={17} width={5} height={5} fill="#00FF88" />
      {/* chin */}
      <rect x={28} y={34} width={40} height={8}  fill="#aaa" stroke="#111" strokeWidth={2} />
      {/* circuit nodes */}
      <rect x={16} y={18} width={8} height={8} fill="#00FF88" stroke="#111" strokeWidth={1} />
      <rect x={72} y={18} width={8} height={8} fill="#00FF88" stroke="#111" strokeWidth={1} />

      {/* ── Neck ── */}
      <rect x={36} y={42} width={24} height={8} fill="#999" stroke="#111" strokeWidth={1.5} />

      {/* ── Left pauldron ── */}
      <rect x={12} y={44} width={22} height={12} fill="#FF3131" stroke="#111" strokeWidth={2} />
      {/* ── Right pauldron ── */}
      <rect x={62} y={44} width={22} height={12} fill="#FF3131" stroke="#111" strokeWidth={2} />

      {/* ── Chest armor ── */}
      <rect x={22} y={50} width={52} height={36} fill="#ccc" stroke="#111" strokeWidth={2} />
      {/* cross insignia */}
      <rect x={44} y={52} width={8}  height={32} fill="#FFE600" />
      <rect x={28} y={62} width={40} height={8}  fill="#FFE600" />
      <rect x={28} y={52} width={16} height={28} fill="#aaa" />
      <rect x={52} y={52} width={16} height={28} fill="#aaa" />
      {/* circuit dots */}
      <rect x={30} y={54} width={5} height={5} fill="#00FF88" />
      <rect x={61} y={54} width={5} height={5} fill="#00FF88" />

      {/* ── Shield (left arm) ── */}
      <motion.g
        animate={{ x: shieldTX }}
        transition={{ type: "spring", stiffness: 500, damping: 20 }}
      >
        <rect x={0}  y={52} width={24} height={36} fill="#4DAAFF" stroke="#111" strokeWidth={2} />
        <rect x={3}  y={55} width={18} height={30} fill="#FF3131" />
        {/* shield cross */}
        <rect x={10} y={56} width={4}  height={28} fill="#FFE600" />
        <rect x={4}  y={66} width={16} height={4}  fill="#FFE600" />
        {/* shield emblem */}
        <rect x={11} y={60} width={6} height={6} fill="#111" />
      </motion.g>

      {/* ── Sword arm (right) ── */}
      <motion.g
        animate={{ y: swordTY, rotate: swordRot }}
        transition={{ type: "spring", stiffness: 400, damping: 18 }}
        style={{ originX: 80, originY: 60 }}
      >
        {/* blade tip */}
        <polygon points="80,0 74,8 86,8" fill="#FFE600" stroke="#111" strokeWidth={1.5} />
        {/* blade */}
        <rect x={74} y={8}  width={12} height={44} fill="#FFE600" stroke="#111" strokeWidth={1.5} />
        {/* crossguard */}
        <rect x={64} y={50} width={32} height={8}  fill="#aaa"    stroke="#111" strokeWidth={2} />
        {/* handle */}
        <rect x={76} y={58} width={8}  height={18} fill="#888"    stroke="#111" strokeWidth={1.5} />
        {/* pommel */}
        <rect x={72} y={76} width={16} height={8}  fill="#ccc"    stroke="#111" strokeWidth={2} />
      </motion.g>

      {/* ── Belt ── */}
      <rect x={22} y={86} width={52} height={10} fill="#333" stroke="#111" strokeWidth={2} />
      <rect x={40} y={87} width={16} height={8}  fill="#FFE600" stroke="#111" strokeWidth={1} />

      {/* ── Tasset (hip guards) ── */}
      <rect x={22} y={96} width={22} height={14} fill="#999" stroke="#111" strokeWidth={1.5} />
      <rect x={52} y={96} width={22} height={14} fill="#999" stroke="#111" strokeWidth={1.5} />

      {/* ── Legs ── */}
      <rect x={24} y={110} width={18} height={24} fill="#777" stroke="#111" strokeWidth={1.5} />
      <rect x={54} y={110} width={18} height={24} fill="#777" stroke="#111" strokeWidth={1.5} />

      {/* ── Boots ── */}
      <rect x={20} y={130} width={22} height={12} fill="#222" stroke="#111" strokeWidth={1.5} />
      <rect x={18} y={138} width={24} height={6}  fill="#111" />
      <rect x={54} y={130} width={22} height={12} fill="#222" stroke="#111" strokeWidth={1.5} />
      <rect x={54} y={138} width={24} height={6}  fill="#111" />
    </svg>
  );
}

/* ─── Bug enemy sprite ───────────────────────────────────────── */
function BugSprite({ label, color, dying }: { label: string; color: string; dying: boolean }) {
  return (
    <motion.div
      animate={dying ? { scale: [1, 1.4, 0], opacity: [1, 1, 0], rotate: [0, 15, -15, 0] } : { y: [0, -4, 0] }}
      transition={dying
        ? { duration: 0.4, ease: "easeOut" }
        : { duration: 1.2, repeat: Infinity, ease: "easeInOut" }
      }
      className="flex flex-col items-center gap-1"
    >
      <svg width={48} height={48} viewBox="0 0 48 48" style={{ imageRendering: "pixelated" }}>
        {/* bug body */}
        <rect x={4}  y={8}  width={40} height={32} fill={color}  stroke="#111" strokeWidth={2} />
        {/* error face */}
        <rect x={10} y={14} width={8}  height={8}  fill="#111" />
        <rect x={30} y={14} width={8}  height={8}  fill="#111" />
        <rect x={12} y={30} width={24} height={4}  fill="#111" />
        {/* antennae */}
        <rect x={12} y={2}  width={4}  height={8}  fill={color} stroke="#111" strokeWidth={1.5} />
        <rect x={32} y={2}  width={4}  height={8}  fill={color} stroke="#111" strokeWidth={1.5} />
        <rect x={10} y={0}  width={8}  height={4}  fill={color} />
        <rect x={30} y={0}  width={8}  height={4}  fill={color} />
        {/* legs */}
        <rect x={0}  y={16} width={6}  height={4}  fill={color} stroke="#111" strokeWidth={1} />
        <rect x={42} y={16} width={6}  height={4}  fill={color} stroke="#111" strokeWidth={1} />
        <rect x={0}  y={26} width={6}  height={4}  fill={color} stroke="#111" strokeWidth={1} />
        <rect x={42} y={26} width={6}  height={4}  fill={color} stroke="#111" strokeWidth={1} />
      </svg>
      <span
        className="font-pixel text-[8px] px-1 leading-tight text-center"
        style={{ color: "#111", background: color, border: "1.5px solid #111" }}
      >
        {label}
      </span>
    </motion.div>
  );
}

/* ─── Main component ─────────────────────────────────────────── */
export function PaladinBattle() {
  const [pose, setPose]       = useState<PoseId>("walk");
  const [facing, setFacing]   = useState<1 | -1>(1);
  const [bubble, setBubble]   = useState<typeof ACTIONS[number] | null>(null);
  const [bugs, setBugs]       = useState<Bug[]>([]);
  const [bugId, setBugId]     = useState(0);
  const [clicks, setClicks]   = useState(0);
  const controls              = useAnimation();
  const containerRef          = useRef<HTMLDivElement>(null);
  const walkingRef            = useRef(true);
  const bubbleTimer           = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const poseTimer             = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  /* spawn bugs periodically */
  useEffect(() => {
    const interval = setInterval(() => {
      if (!containerRef.current) return;
      // clamp to actual visible width so bugs never overflow
      const w = Math.min(containerRef.current.offsetWidth, window.innerWidth);
      const fromRight = Math.random() > 0.5;
      setBugs((prev) => {
        if (prev.filter((b) => !b.dying).length >= 5) return prev;
        return [
          ...prev,
          {
            id: bugId,
            x: fromRight ? w - 80 : 20,
            label: BUG_LABELS[Math.floor(Math.random() * BUG_LABELS.length)],
            color: BUG_COLORS[Math.floor(Math.random() * BUG_COLORS.length)],
            dying: false,
          },
        ];
      });
      setBugId((n) => n + 1);
    }, 3200);

    return () => clearInterval(interval);
  }, [bugId]);

  /* auto-remove dead bugs */
  useEffect(() => {
    const dying = bugs.filter((b) => b.dying);
    if (dying.length === 0) return;
    const t = setTimeout(() => {
      setBugs((prev) => prev.filter((b) => !b.dying));
    }, 600);
    return () => clearTimeout(t);
  }, [bugs]);

  /* patrol loop */
  useEffect(() => {
    let cancelled = false;

    async function patrol() {
      if (!containerRef.current) return;

      while (!cancelled) {
        // Re-read bounds each cycle so resize changes are picked up
        const w = Math.min(containerRef.current.offsetWidth, window.innerWidth);
        // 96px = character width, 16px = right padding
        const end = Math.max(16, w - 96 - 16);

        setFacing(1);
        setPose("walk");
        await controls.start({
          x: end,
          transition: { duration: Math.max(3, end / 80), ease: "linear" },
        });
        if (cancelled) break;

        const wBack = Math.min(containerRef.current.offsetWidth, window.innerWidth);
        const endBack = Math.max(16, wBack - 96 - 16);

        setFacing(-1);
        setPose("walk");
        await controls.start({
          x: 16,
          transition: { duration: Math.max(3, endBack / 80), ease: "linear" },
        });
      }
    }

    const raf = requestAnimationFrame(() => { patrol(); });
    return () => {
      cancelled = true;
      cancelAnimationFrame(raf);
      controls.stop();
    };
  }, [controls]);

  /* handle interaction */
  const handleInteract = useCallback(() => {
    const action = ACTIONS[Math.floor(Math.random() * ACTIONS.length)];
    setClicks((c) => c + 1);

    clearTimeout(bubbleTimer.current);
    clearTimeout(poseTimer.current);

    setBubble(action);
    setPose(action.pose as PoseId);

    // kill closest bug
    setBugs((prev) => {
      const alive = prev.filter((b) => !b.dying);
      if (alive.length === 0) return prev;
      return prev.map((b, i) =>
        !b.dying && i === prev.findIndex((x) => !x.dying) ? { ...b, dying: true } : b
      );
    });

    bubbleTimer.current = setTimeout(() => setBubble(null), 2600);
    poseTimer.current   = setTimeout(() => setPose("walk"),   800);
  }, []);

  /* walk leg bob */
  const legBob = pose === "walk" ? { y: [0, -6, 0] } : { y: 0 };

  return (
    <section
      id="paladin-battle"
      className="relative overflow-hidden"
      style={{ background: "#0a0a0a", borderTop: "3px solid #FF3131", borderBottom: "3px solid #FF3131" }}
    >
      {/* ── Header ── */}
      <div
        className="relative z-10 flex flex-wrap items-center justify-between gap-4 px-5 sm:px-10 py-5"
        style={{ borderBottom: "2px solid #FF3131" }}
      >
        <div>
          <p className="font-pixel text-[10px] text-[#FF3131] tracking-widest mb-1">
            [ STAGE 01 ]
          </p>
          <h2 className="font-pixel text-base sm:text-xl text-white leading-tight">
            PALADIN VS <span className="text-[#FF3131]">PRODUCTION</span>
          </h2>
        </div>
        <div className="flex items-center gap-6 font-pixel text-[9px]">
          <span className="text-[#00FF88]">BUGS SLAIN: {clicks}</span>
          <span className="text-[#FFE600]">
            <span className="animate-blink">▋</span> CLICK PALADIN TO ATTACK
          </span>
        </div>
      </div>

      {/* ── Battlefield ── */}
      <div
        ref={containerRef}
        className="relative overflow-hidden"
        style={{ height: 280, background: "#0f0f0f" }}
      >
        {/* dot grid overlay */}
        <div className="absolute inset-0 dot-grid opacity-10 pointer-events-none" />

        {/* background: floating "error" text */}
        {["503", "OOM", "NaN", "ERR", "PANIC", "NULL"].map((t, i) => (
          <motion.span
            key={t}
            className="absolute font-pixel text-[9px] select-none pointer-events-none"
            style={{
              left: `${10 + i * 15}%`,
              top:  `${20 + (i % 3) * 20}%`,
              color: "#FF3131",
              opacity: 0.12,
            }}
            animate={{ y: [0, -8, 0], opacity: [0.12, 0.2, 0.12] }}
            transition={{ duration: 2 + i * 0.4, repeat: Infinity, ease: "easeInOut" }}
          >
            {t}
          </motion.span>
        ))}

        {/* ── Bug enemies ── */}
        {bugs.map((bug) => (
          <div
            key={bug.id}
            className="absolute bottom-[80px]"
            style={{ left: bug.x }}
          >
            <BugSprite label={bug.label} color={bug.color} dying={bug.dying} />
          </div>
        ))}

        {/* ── Paladin character ── */}
        <motion.div
          animate={controls}
          style={{ position: "absolute", bottom: 64, x: 16, cursor: "pointer" }}
          onClick={handleInteract}
          whileHover={{ filter: "brightness(1.15)" }}
          title="Click to attack!"
        >
          {/* Speech bubble sits OUTSIDE the scaleX wrapper so text is never mirrored */}
          <AnimatePresence>
            {bubble && (
              <motion.div
                key={bubble.id}
                initial={{ opacity: 0, y: 10, scale: 0.8 }}
                animate={{ opacity: 1, y: 0,  scale: 1   }}
                exit={{  opacity: 0, y: -8, scale: 0.8 }}
                transition={{ duration: 0.18 }}
                className="absolute font-pixel text-[10px] text-[#111] text-center leading-tight whitespace-pre-line px-2 py-1"
                style={{
                  background: bubble.color,
                  border: "2.5px solid #111",
                  boxShadow: "3px 3px 0 #111",
                  bottom: "calc(100% + 8px)",
                  left: "50%",
                  transform: "translateX(-50%)",
                  minWidth: 90,
                  pointerEvents: "none",
                  zIndex: 10,
                }}
              >
                {bubble.text}
                {/* bubble tail */}
                <span
                  className="absolute -bottom-[10px] left-1/2 -translate-x-1/2"
                  style={{
                    width: 0,
                    height: 0,
                    borderLeft:  "6px solid transparent",
                    borderRight: "6px solid transparent",
                    borderTop:   `10px solid ${bubble.color}`,
                  }}
                />
              </motion.div>
            )}
          </AnimatePresence>

          {/* scale for facing direction — only the sprite flips */}
          <motion.div
            animate={{ scaleX: facing }}
            transition={{ duration: 0.08 }}
            style={{ transformOrigin: "48px center" }}
          >
            {/* walking body bob */}
            <motion.div
              animate={legBob}
              transition={{ duration: 0.35, repeat: pose === "walk" ? Infinity : 0, ease: "easeInOut" }}
            >
              <PaladinSprite pose={pose} facing={facing} />
            </motion.div>
          </motion.div>
        </motion.div>

        {/* ── Ground platform ── */}
        <div
          className="absolute bottom-0 left-0 right-0 h-16"
          style={{
            background: "repeating-linear-gradient(90deg, #1a1a1a 0px, #1a1a1a 47px, #222 47px, #222 48px)",
            borderTop: "3px solid #FF3131",
          }}
        />
        {/* ground bricks */}
        <div
          className="absolute bottom-0 left-0 right-0 h-4"
          style={{
            background: "repeating-linear-gradient(90deg, #FF3131 0px, #FF3131 47px, #111 47px, #111 48px)",
            opacity: 0.3,
          }}
        />
      </div>

      {/* ── Action log / ticker ── */}
      <div
        className="relative flex items-center gap-3 px-5 py-3 overflow-hidden"
        style={{ borderTop: "2px solid #333", background: "#080808" }}
      >
        <span className="font-pixel text-[9px] text-[#FF3131] shrink-0">[ LOG ]</span>
        <div className="overflow-hidden flex-1">
          <motion.div
            className="flex gap-8 font-pixel text-[9px] text-white/40 whitespace-nowrap"
            animate={{ x: ["0%", "-50%"] }}
            transition={{ duration: 18, repeat: Infinity, ease: "linear" }}
          >
            {[
              "DB CONNECTION POOL EXHAUSTED",
              "P99 LATENCY: 2.3s",
              "ACTIVE ALERTS: 3",
              "MEMORY USAGE: 94%",
              "DISK I/O SPIKE DETECTED",
              "CIRCUIT BREAKER: OPEN",
              "QUEUE DEPTH: 18,432",
              "CPU THROTTLED",
              "DB CONNECTION POOL EXHAUSTED",
              "P99 LATENCY: 2.3s",
              "ACTIVE ALERTS: 3",
              "MEMORY USAGE: 94%",
              "DISK I/O SPIKE DETECTED",
              "CIRCUIT BREAKER: OPEN",
              "QUEUE DEPTH: 18,432",
              "CPU THROTTLED",
            ].map((t, i) => (
              <span key={i} className="shrink-0">
                <span className="text-[#FF3131]">■</span> {t}
              </span>
            ))}
          </motion.div>
        </div>
      </div>
    </section>
  );
}

export default PaladinBattle;
