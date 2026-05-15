import { Sparkles } from "lucide-react"
import { motion } from "framer-motion"
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { type DashboardData } from "@/api"
import { cn } from "@/lib/utils"

const activityToneClass: Record<string, string> = {
  critical: "bg-orange-500 shadow-[0_0_0_4px_rgba(249,115,22,0.12)]",
  info: "bg-violet-500 shadow-[0_0_0_4px_rgba(139,92,246,0.12)]",
  success: "bg-emerald-500 shadow-[0_0_0_4px_rgba(16,185,129,0.12)]",
}

const sloToneClass: Record<string, string> = {
  good: "from-emerald-500 to-cyan-500",
  warn: "from-amber-500 to-orange-500",
}

export function SystemHealthPanel({ data }: { data: DashboardData }) {
  return (
    <section className="grid gap-2.5 xl:grid-cols-[minmax(0,1.3fr)_360px]">
      <Card className="overflow-hidden border-0 bg-white/88 shadow-[0_20px_70px_rgba(15,23,42,0.08)] ring-1 ring-slate-200/70 dark:bg-card/90 dark:ring-white/10">
        <CardContent className="grid gap-6 p-5 lg:grid-cols-[minmax(0,1fr)_220px]">
          <div>
            <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <CardDescription className="text-[12px] font-semibold uppercase tracking-[0.1em]">Pipeline overview</CardDescription>
                <h2 className="mt-1 text-[34px] font-semibold leading-none tracking-[-0.02em] sm:text-[40px]">
                  System Health, Normal.
                </h2>
              </div>
              <Badge variant="secondary" className="w-fit border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300">
                {data.metrics.checksGreen} checks green
              </Badge>
            </div>
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_190px]">
              <div>
                <div className="mb-2 flex items-center justify-between gap-3">
                  <div>
                    <div className="text-sm font-semibold">Retrieval accuracy</div>
                    <div className="text-xs text-muted-foreground">Goal 95% · last 60 minutes</div>
                  </div>
                  <div className="rounded-full bg-lime-300 px-2 py-1 text-xs font-semibold text-slate-950">{data.metrics.avgConfidence}%</div>
                </div>
                <div className="h-44">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={data.qualityTrend} margin={{ left: -28, right: 8, top: 10, bottom: 0 }}>
                      <defs>
                        <linearGradient id="hero-accuracy-fill" x1="0" x2="0" y1="0" y2="1">
                          <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.22} />
                          <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.01} />
                        </linearGradient>
                      </defs>
                      <XAxis dataKey="time" tickLine={false} axisLine={false} minTickGap={22} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
                      <YAxis hide domain={[70, 100]} />
                      <Tooltip cursor={{ stroke: "hsl(var(--border))" }} contentStyle={tooltipStyle} />
                      <Area
                        type="monotone"
                        dataKey="accuracy"
                        stroke="#8b5cf6"
                        strokeWidth={3}
                        fill="url(#hero-accuracy-fill)"
                        activeDot={{ r: 5, fill: "#c7f72c", stroke: "#111827", strokeWidth: 2 }}
                        isAnimationActive={false}
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </div>
              <TokenUsageRing metrics={data.metrics} />
            </div>
          </div>
          <div className="grid gap-2.5">
            <SemanticMapCard metrics={data.metrics} />
            <ApiHealthCard metrics={data.metrics} />
          </div>
        </CardContent>
      </Card>
      <ComputeLoadCard data={data} />
    </section>
  )
}

const tooltipStyle = {
  borderRadius: 8,
  borderColor: "hsl(var(--border))",
  background: "hsl(var(--card))",
  color: "hsl(var(--foreground))",
  boxShadow: "0 18px 42px rgba(15, 23, 42, 0.14)",
}

function TokenUsageRing({ metrics }: { metrics: DashboardData["metrics"] }) {
  return (
    <div className="grid place-items-center rounded-lg border bg-background/55 p-4 text-center">
      <div
        className="grid size-32 place-items-center rounded-full"
        style={{
          background:
            `conic-gradient(#c7f72c 0 ${metrics.tokenUsagePct}%, #8b5cf6 ${metrics.tokenUsagePct}% 100%), radial-gradient(circle, hsl(var(--card)) 0 58%, transparent 59%)`,
        }}
      >
        <div className="grid size-[94px] place-items-center rounded-full bg-card text-center shadow-inner">
          <div>
            <div className="text-[11px] text-muted-foreground">Monthly token</div>
            <div className="text-[11px] text-muted-foreground">usage</div>
          </div>
        </div>
      </div>
      <div className="mt-3 text-[34px] font-semibold leading-none tracking-[-0.02em]">{metrics.tokenUsagePct}%</div>
      <div className="mt-2 grid w-full gap-1 text-xs text-muted-foreground">
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-violet-500" />Generation</span>
          <span>{metrics.generationShare}%</span>
        </div>
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-lime-300" />Embedding</span>
          <span>{metrics.embeddingShare}%</span>
        </div>
      </div>
    </div>
  )
}

function SemanticMapCard({ metrics }: { metrics: DashboardData["metrics"] }) {
  return (
    <div className="rounded-lg border bg-background/55 p-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">Semantic map</div>
          <div className="text-xs text-muted-foreground">Runbook index · {metrics.semanticPoints} docs</div>
        </div>
        <Badge variant="secondary">prod</Badge>
      </div>
      <div className="relative mt-3 h-28 overflow-hidden rounded-lg bg-muted/40">
        <div className="absolute left-8 top-14 h-px w-28 rotate-[-6deg] bg-border" />
        <div className="absolute right-12 top-12 h-px w-24 rotate-[-30deg] bg-border" />
        <div className="absolute left-14 top-12 size-4 rounded-full bg-violet-500 shadow-[0_0_0_10px_rgba(139,92,246,0.10)]" />
        <div className="absolute left-28 top-16 size-2 rounded-full bg-violet-300" />
        <div className="absolute right-24 top-14 size-4 rounded-full bg-lime-300 shadow-[0_0_0_22px_rgba(199,247,44,0.10)]" />
        <div className="absolute right-10 top-8 size-3 rounded-full bg-sky-500" />
        <div className="absolute left-24 top-8 h-14 w-40 rounded-[50%] border border-dashed border-border" />
      </div>
    </div>
  )
}

function ApiHealthCard({ metrics }: { metrics: DashboardData["metrics"] }) {
  return (
    <div className="rounded-lg border bg-background/55 p-3">
      <div className="text-sm font-semibold">API health</div>
      <div className="mt-3 flex items-center gap-3">
        <div className="grid size-20 place-items-center rounded-full" style={{ background: `conic-gradient(#f97316 0 ${100 - metrics.apiHealthPct}%, #e5e7eb ${100 - metrics.apiHealthPct}% 100%)` }}>
          <div className="grid size-14 place-items-center rounded-full bg-card text-center">
            <span className="text-lg font-semibold">{metrics.apiHealthPct}%</span>
          </div>
        </div>
        <div className="text-sm">
          <div className="font-medium">Uptime</div>
          <div className="text-muted-foreground">p95 {metrics.avgLatencyMS}ms · {metrics.checksGreen} checks green</div>
        </div>
      </div>
    </div>
  )
}

export function ComputeLoadCard({ data }: { data: DashboardData }) {
  return (
    <Card className="overflow-hidden border-0 bg-gradient-to-br from-cyan-500 via-sky-600 to-slate-950 text-white shadow-[0_24px_80px_rgba(2,132,199,0.32)]">
      <CardContent className="p-5">
        <div className="mb-8 flex items-start justify-between">
          <div>
            <CardDescription className="text-white/70">Compute load</CardDescription>
            <CardTitle className="mt-1 text-[28px]">{data.metrics.computeLoadPct}% load</CardTitle>
          </div>
          <Sparkles className="size-5 text-lime-200" />
        </div>
        <div className="h-52">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data.qualityTrend} margin={{ left: -28, right: 8, top: 10, bottom: 0 }}>
              <defs>
                <linearGradient id="compute-fill" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="5%" stopColor="#ffffff" stopOpacity={0.24} />
                  <stop offset="95%" stopColor="#ffffff" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <XAxis dataKey="time" tickLine={false} axisLine={false} minTickGap={20} tick={{ fill: "rgba(255,255,255,0.65)", fontSize: 11 }} />
              <YAxis hide />
              <Tooltip cursor={{ stroke: "rgba(255,255,255,0.3)" }} contentStyle={{ borderRadius: 8, borderColor: "rgba(255,255,255,0.14)", background: "rgba(15,23,42,0.92)", color: "white" }} />
              <Area type="monotone" dataKey="latency" stroke="#c7f72c" strokeWidth={3} fill="url(#compute-fill)" activeDot={{ r: 5 }} isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}

export function IncidentTrendChart({ data }: { data: DashboardData["incidentTrend"] }) {
  return (
    <div className="h-56" aria-label="Incident volume bar chart">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ left: -28, right: 4, top: 8, bottom: 0 }}>
          <defs>
            <linearGradient id="incident-fill" x1="0" x2="0" y1="0" y2="1">
              <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.28} />
              <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <XAxis dataKey="day" tickLine={false} axisLine={false} minTickGap={18} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
          <YAxis hide />
          <Tooltip cursor={{ stroke: "hsl(var(--border))" }} contentStyle={tooltipStyle} />
          <Area type="monotone" dataKey="incidents" stroke="#8b5cf6" strokeWidth={2} fill="url(#incident-fill)" activeDot={{ r: 4 }} isAnimationActive={false} />
          <Area type="monotone" dataKey="noise" stroke="#10b981" strokeWidth={2} fillOpacity={0} strokeDasharray="4 4" isAnimationActive={false} />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

export function SeverityDonut({ data }: { data: DashboardData["severitySplit"] }) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-1">
        <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Distribution</CardDescription>
        <CardTitle className="text-[15px]">Severity split</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-[150px_minmax(0,1fr)] items-center gap-2 max-sm:grid-cols-1">
        <div className="h-32">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie data={data} dataKey="value" nameKey="severity" innerRadius={42} outerRadius={58} paddingAngle={5} stroke="none" isAnimationActive={false}>
                {data.map((item) => <Cell key={item.severity} fill={item.color} />)}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        </div>
        <div className="grid gap-2">
          {data.map((item) => (
            <div key={item.severity} className="grid grid-cols-[32px_minmax(0,1fr)_20px] items-center gap-2 text-sm">
              <span className="font-medium">{item.severity}</span>
              <span className="h-2 overflow-hidden rounded-full bg-muted">
                <span className="block h-full rounded-full" style={{ width: `${Math.min(100, item.value * 35)}%`, backgroundColor: item.color }} />
              </span>
              <span className="text-right text-muted-foreground">{item.value}</span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

export function QualityTrendCard({ data, accuracy }: { data: DashboardData["qualityTrend"]; accuracy: number }) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">LLM evals</CardDescription>
            <CardTitle className="text-[15px]">Eval quality</CardTitle>
          </div>
          <div className="rounded-full border bg-background/70 px-2 py-1 text-xs text-muted-foreground">{accuracy}% acc</div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="h-36" aria-label="Eval quality chart">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ left: -28, right: 4, top: 8, bottom: 0 }}>
              <defs>
                <linearGradient id="accuracy-fill" x1="0" x2="0" y1="0" y2="1"><stop offset="5%" stopColor="#10b981" stopOpacity={0.28} /><stop offset="95%" stopColor="#10b981" stopOpacity={0.02} /></linearGradient>
                <linearGradient id="latency-fill" x1="0" x2="0" y1="0" y2="1"><stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.22} /><stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.02} /></linearGradient>
              </defs>
              <XAxis dataKey="time" tickLine={false} axisLine={false} minTickGap={16} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 10 }} />
              <YAxis hide domain={[0, 100]} />
              <Tooltip cursor={{ stroke: "hsl(var(--border))" }} contentStyle={tooltipStyle} />
              <Area type="monotone" dataKey="accuracy" stroke="#10b981" strokeWidth={2} fill="url(#accuracy-fill)" isAnimationActive={false} />
              <Area type="monotone" dataKey="latency" stroke="#8b5cf6" strokeWidth={2} fill="url(#latency-fill)" isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
        <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-emerald-500" />accuracy</span>
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-violet-500" />latency</span>
        </div>
      </CardContent>
    </Card>
  )
}

export function ModelMixCard({ data }: { data: DashboardData["modelMix"] }) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Router policy</CardDescription>
        <CardTitle className="text-[15px]">Model mix</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        {data.map((item) => (
          <div key={item.tier} className="grid gap-1.5">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="font-medium">{item.tier}</span>
              <span className="min-w-0 truncate text-muted-foreground">{item.label}</span>
              <span className="tabular-nums">{item.share}%</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <motion.div className={cn("h-full rounded-full bg-gradient-to-r", item.color)} initial={{ width: 0 }} animate={{ width: `${item.share}%` }} transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }} />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export function SLOBudgetCard({ data }: { data: DashboardData["sloBudget"] }) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">SLO control</CardDescription>
            <CardTitle className="text-[15px]">Error budget</CardTitle>
          </div>
          <Badge variant="secondary">live</Badge>
        </div>
      </CardHeader>
      <CardContent className="grid gap-3">
        {data.map((item) => (
          <div key={item.service} className="grid gap-1.5">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="font-medium">{item.service}</span>
              <span className="tabular-nums text-muted-foreground">{item.budget}% left</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <motion.div className={cn("h-full rounded-full bg-gradient-to-r", sloToneClass[item.tone])} initial={{ width: 0 }} animate={{ width: `${item.budget}%` }} transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }} />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export function ActivityStream({ data }: { data: DashboardData["activity"] }) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Control loop</CardDescription>
        <CardTitle className="text-[15px]">Activity</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-1">
        {data.map((item) => (
          <motion.div key={`${item.time}-${item.label}`} className="grid grid-cols-[42px_12px_minmax(0,1fr)] items-start gap-2 rounded-md px-1.5 py-1.5 text-sm transition-colors hover:bg-accent/60" initial={{ opacity: 0.72, y: 3 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.18 }}>
            <span className="text-xs tabular-nums text-muted-foreground">{item.time}</span>
            <span className={cn("mt-1.5 size-2 rounded-full", activityToneClass[item.tone])} />
            <span className="leading-snug">{item.label}</span>
          </motion.div>
        ))}
      </CardContent>
    </Card>
  )
}

export function AgentTimeline({ data }: { data: DashboardData["agentTimeline"] }) {
  return (
    <div>
      <div className="mb-2 text-xs font-semibold uppercase text-muted-foreground">Agent path</div>
      <div className="h-24">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} layout="vertical" margin={{ left: -36, right: 4, top: 0, bottom: 0 }}>
            <XAxis type="number" hide />
            <YAxis dataKey="label" type="category" width={120} tickLine={false} axisLine={false} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
            <Tooltip cursor={{ fill: "hsl(var(--accent))" }} contentStyle={tooltipStyle} />
            <Bar dataKey="ms" radius={[0, 6, 6, 0]} fill="#8b5cf6" isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
