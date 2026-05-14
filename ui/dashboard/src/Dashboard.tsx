import {
  Activity,
  ArrowRight,
  AlertTriangle,
  Bell,
  BookOpen,
  CheckCircle2,
  CircleDot,
  Command,
  GitBranch,
  LayoutDashboard,
  Moon,
  Search,
  Settings,
  ShieldCheck,
  Sparkles,
  TerminalSquare,
  X,
} from "lucide-react"
import { AnimatePresence, motion } from "framer-motion"
import { useEffect, useMemo, useState } from "react"
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
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  activity,
  agentTimeline,
  incidentTrend,
  incidents,
  integrations,
  runbooks,
  severitySplit,
  sloBudget,
  type Incident,
} from "@/data"
import { cn } from "@/lib/utils"

const navItems = [
  { label: "Dashboard", icon: LayoutDashboard, count: 3 },
  { label: "Incidents", icon: AlertTriangle },
  { label: "Runbooks", icon: BookOpen },
  { label: "Integrations", icon: GitBranch },
  { label: "Settings", icon: Settings },
]

const severityClass: Record<Incident["severity"], string> = {
  P1: "border-orange-200 bg-orange-50 text-orange-700 shadow-[0_0_0_3px_rgba(249,115,22,0.08)]",
  P2: "border-amber-200 bg-amber-50 text-amber-700 shadow-[0_0_0_3px_rgba(245,158,11,0.08)]",
  P3: "border-violet-200 bg-violet-50 text-violet-700 shadow-[0_0_0_3px_rgba(139,92,246,0.08)]",
  P4: "border-slate-200 bg-white text-slate-500",
}

const cardMotion = {
  initial: { y: 4 },
  animate: { y: 0 },
  transition: { duration: 0.22, ease: [0.22, 1, 0.36, 1] },
}

const activityToneClass: Record<string, string> = {
  critical: "bg-orange-500 shadow-[0_0_0_4px_rgba(249,115,22,0.12)]",
  info: "bg-violet-500 shadow-[0_0_0_4px_rgba(139,92,246,0.12)]",
  success: "bg-emerald-500 shadow-[0_0_0_4px_rgba(16,185,129,0.12)]",
}

const sloToneClass: Record<string, string> = {
  good: "from-emerald-500 to-cyan-500",
  warn: "from-amber-500 to-orange-500",
}

export function Dashboard() {
  const [selectedID, setSelectedID] = useState(incidents[0].id)
  const [severity, setSeverity] = useState("all")
  const [query, setQuery] = useState("")
  const [commandOpen, setCommandOpen] = useState(false)
  const [dark, setDark] = useState(() => window.localStorage.getItem("paladin-theme") === "dark")

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault()
        setCommandOpen(true)
      }
      if (event.key === "Escape") {
        setCommandOpen(false)
      }
    }
    window.addEventListener("keydown", handler)
    return () => window.removeEventListener("keydown", handler)
  }, [])

  useEffect(() => {
    window.localStorage.setItem("paladin-theme", dark ? "dark" : "light")
  }, [dark])

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return incidents.filter((incident) => {
      if (severity !== "all" && incident.severity !== severity) return false
      if (!needle) return true
      return `${incident.id} ${incident.title} ${incident.service}`.toLowerCase().includes(needle)
    })
  }, [severity, query])

  const selected = filtered.find((incident) => incident.id === selectedID) ?? filtered[0] ?? incidents[0]
  const active = incidents.filter((incident) => incident.status !== "Resolved")
  const p1 = active.filter((incident) => incident.severity === "P1").length
  const p2 = active.filter((incident) => incident.severity === "P2").length

  return (
    <div className={cn("min-h-screen bg-background", dark && "dark")}>
      <div className="linear-surface grid min-h-screen grid-cols-1 lg:grid-cols-[228px_minmax(0,1fr)]">
        <Sidebar />
        <main className="min-w-0 px-4 py-4 sm:px-6 lg:px-6">
          <Topbar dark={dark} onTheme={() => setDark((value) => !value)} onCommand={() => setCommandOpen(true)} />

          <motion.section
            aria-label="Incident metrics"
            className="grid gap-2.5 md:grid-cols-2 xl:grid-cols-4"
            initial="initial"
            animate="animate"
            transition={{ staggerChildren: 0.04 }}
          >
            <Metric title="Active incidents" value={String(active.length)} detail={`${p1} P1, ${p2} P2`} icon={Bell} />
            <Metric title="MTTR this week" value="14m" detail="22% faster" icon={CheckCircle2} intent="good" />
            <Metric title="Alert noise ratio" value="0.31" detail="within target" icon={ShieldCheck} intent="good" />
            <Metric title="LLM cost today" value="$42" detail="tier A: 71%" icon={Sparkles} />
          </motion.section>

          <section className="mt-2.5 grid gap-2.5 xl:grid-cols-[minmax(0,1.55fr)_360px]">
            <Card className="overflow-hidden">
              <CardHeader className="flex-row items-center justify-between gap-4 space-y-0 border-b bg-card/70 px-4 py-3">
                <div>
                  <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Live triage</CardDescription>
                  <CardTitle className="text-[15px]">Incidents</CardTitle>
                </div>
                <div className="flex min-w-0 flex-1 justify-end gap-2">
                  <label className="sr-only" htmlFor="severity-filter">
                    Severity filter
                  </label>
                  <select
                    id="severity-filter"
                    className="h-8 rounded-md border bg-background/80 px-2 text-sm shadow-[0_1px_0_rgba(15,23,42,0.03)]"
                    value={severity}
                    onChange={(event) => setSeverity(event.target.value)}
                  >
                    <option value="all">All severities</option>
                    <option value="P1">P1</option>
                    <option value="P2">P2</option>
                    <option value="P3">P3</option>
                    <option value="P4">P4</option>
                  </select>
                  <div className="relative w-full max-w-[260px]">
                    <Search className="absolute left-2 top-2 size-4 text-muted-foreground" />
                    <Input
                      className="bg-background/80 pl-8"
                      placeholder="Search service or title"
                      value={query}
                      onChange={(event) => setQuery(event.target.value)}
                    />
                  </div>
                </div>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/40">
                      <TableHead>Severity</TableHead>
                      <TableHead>Incident</TableHead>
                      <TableHead>Service</TableHead>
                      <TableHead>Age</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Agent</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filtered.map((incident) => (
                      <motion.tr
                        key={incident.id}
                        className={cn(
                          "cursor-pointer border-b border-l-2 border-l-transparent transition-colors hover:bg-accent/40",
                          selected.id === incident.id && "border-l-primary bg-accent/70",
                        )}
                        onClick={() => setSelectedID(incident.id)}
                        layout
                        initial={{ y: 2 }}
                        animate={{ y: 0 }}
                        transition={{ duration: 0.18 }}
                      >
                        <TableCell>
                          <Badge variant="outline" className={severityClass[incident.severity]}>
                            {incident.severity}
                          </Badge>
                        </TableCell>
                        <TableCell className="min-w-[280px] font-medium leading-snug">
                          <span className="text-muted-foreground">{incident.id}</span> · {incident.title}
                        </TableCell>
                        <TableCell>{incident.service}</TableCell>
                        <TableCell>{incident.age}</TableCell>
                        <TableCell>{incident.status}</TableCell>
                        <TableCell>{incident.agent}</TableCell>
                      </motion.tr>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            <IncidentDetail incident={selected} />

            <Card className="overflow-hidden">
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">30-day trend</CardDescription>
                    <CardTitle className="text-[15px]">Incident volume</CardTitle>
                  </div>
                  <div className="rounded-full border bg-background/70 px-2 py-1 text-xs text-muted-foreground">noise down 72%</div>
                </div>
              </CardHeader>
              <CardContent className="pt-2">
                <IncidentTrendChart />
              </CardContent>
            </Card>

            <div className="grid gap-2.5">
              <SeverityDonut />
              <SLOBudgetCard />
              <ActivityStream />
              <SideStacks />
            </div>
          </section>
        </main>
      </div>
      <CommandPalette
        open={commandOpen}
        onOpenChange={setCommandOpen}
        onSelectIncident={(id) => {
          setSelectedID(id)
          setSeverity("all")
        }}
        onSetSeverity={setSeverity}
      />
    </div>
  )
}

function Sidebar() {
  return (
    <aside className="border-r bg-card/80 px-3 py-4 backdrop-blur">
      <div className="mb-5 flex items-center gap-2 px-2">
        <div className="grid size-8 place-items-center rounded-md bg-gradient-to-br from-violet-500 to-emerald-500 text-white shadow-sm">
          <Command className="size-4" />
        </div>
        <div className="leading-tight">
          <div className="font-semibold">PaladinAI</div>
          <div className="text-xs text-muted-foreground">Incident Command</div>
        </div>
      </div>
      <nav className="grid gap-1">
        {navItems.map((item, index) => (
          <a
            key={item.label}
            href={`#${item.label.toLowerCase()}`}
            className={cn(
              "group flex items-center gap-2 rounded-md px-2 py-1.5 text-sm text-muted-foreground transition-colors",
              index === 0 && "bg-accent text-accent-foreground",
            )}
          >
            <item.icon className="size-4 transition-colors group-hover:text-foreground" />
            <span className="flex-1">{item.label}</span>
            {item.count ? <span className="text-xs tabular-nums">{item.count}</span> : null}
          </a>
        ))}
      </nav>
      <div className="mt-6 rounded-lg border bg-background/70 p-3 text-xs text-muted-foreground shadow-[0_1px_0_rgba(15,23,42,0.03)]">
        <div className="mb-2 flex items-center gap-2 text-foreground">
          <CircleDot className="size-3 fill-emerald-500 text-emerald-500" />
          Live stream connected
        </div>
        <div>RCA queue 2 active · cost guardrail normal</div>
      </div>
    </aside>
  )
}

function Topbar({ dark, onTheme, onCommand }: { dark: boolean; onTheme: () => void; onCommand: () => void }) {
  return (
    <header className="mb-4 flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">Tenant: acme-prod</p>
        <h1 className="text-[26px] font-semibold leading-tight tracking-[-0.01em]">Operations dashboard</h1>
      </div>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <button
          className="flex h-8 min-w-[320px] items-center gap-2 rounded-md border bg-card/80 px-3 text-left text-sm text-muted-foreground shadow-[0_1px_0_rgba(15,23,42,0.03)] transition-all hover:-translate-y-px hover:bg-accent hover:text-accent-foreground hover:shadow-md max-sm:min-w-0"
          type="button"
          aria-label="Open command menu"
          onClick={onCommand}
        >
          <Search className="size-4" />
          <span className="flex-1">Search incidents, runbooks, services</span>
          <kbd className="rounded border bg-background px-1.5 py-0.5 text-[10px] text-muted-foreground">Cmd K</kbd>
        </button>
        <Button variant="outline" onClick={onTheme} aria-pressed={dark}>
          <Moon className="size-4" />
          {dark ? "Light" : "Dark"}
        </Button>
        <Button variant="outline">
          <TerminalSquare className="size-4" />
          Export
        </Button>
      </div>
    </header>
  )
}

function CommandPalette({
  open,
  onOpenChange,
  onSelectIncident,
  onSetSeverity,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelectIncident: (id: string) => void
  onSetSeverity: (severity: string) => void
}) {
  const [value, setValue] = useState("")
  const commands = [
    ...incidents.map((incident) => ({
      key: incident.id,
      title: `${incident.id} · ${incident.title}`,
      detail: `${incident.service} · ${incident.status}`,
      action: () => onSelectIncident(incident.id),
    })),
    { key: "severity-p1", title: "Show P1 incidents", detail: "Filter incident table", action: () => onSetSeverity("P1") },
    { key: "severity-all", title: "Show all incidents", detail: "Clear severity filter", action: () => onSetSeverity("all") },
    { key: "runbooks", title: "Open runbooks", detail: "Response library", action: () => undefined },
    { key: "integrations", title: "Open integrations", detail: "Tool health", action: () => undefined },
  ]
  const filtered = commands.filter((item) => `${item.title} ${item.detail}`.toLowerCase().includes(value.toLowerCase()))

  function run(action: () => void) {
    action()
    onOpenChange(false)
    setValue("")
  }

  return (
    <AnimatePresence>
      {open ? (
        <motion.div
          className="fixed inset-0 z-50 grid place-items-start bg-slate-950/20 px-4 pt-24 backdrop-blur-sm"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          <motion.div
            role="dialog"
            aria-label="Command menu"
            className="mx-auto w-full max-w-2xl overflow-hidden rounded-xl border bg-card shadow-2xl"
            initial={{ y: -10, scale: 0.985 }}
            animate={{ y: 0, scale: 1 }}
            exit={{ y: -8, scale: 0.985 }}
            transition={{ duration: 0.16 }}
          >
            <div className="flex items-center gap-2 border-b px-3 py-2">
              <Search className="size-4 text-muted-foreground" />
              <input
                autoFocus
                className="h-9 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                placeholder="Jump to incident, runbook, integration, filter..."
                value={value}
                onChange={(event) => setValue(event.target.value)}
              />
              <Button variant="ghost" size="icon" onClick={() => onOpenChange(false)} aria-label="Close command menu">
                <X className="size-4" />
              </Button>
            </div>
            <div className="max-h-[420px] overflow-y-auto p-2">
              {filtered.map((item) => (
                <button
                  key={item.key}
                  className="group grid w-full grid-cols-[1fr_auto] items-center gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors hover:bg-accent"
                  type="button"
                  onClick={() => run(item.action)}
                >
                  <span>
                    <span className="block font-medium">{item.title}</span>
                    <span className="text-xs text-muted-foreground">{item.detail}</span>
                  </span>
                  <ArrowRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
                </button>
              ))}
            </div>
          </motion.div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  )
}

function Metric({
  title,
  value,
  detail,
  icon: Icon,
  intent,
}: {
  title: string
  value: string
  detail: string
  icon: typeof Activity
  intent?: "good"
}) {
  return (
    <motion.div variants={cardMotion}>
      <Card className="relative overflow-hidden transition-all hover:-translate-y-px hover:shadow-lg">
        <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/50 to-transparent" />
        <CardContent className="p-4">
          <div className="mb-3 flex items-center justify-between">
            <span className="text-[13px] text-muted-foreground">{title}</span>
            <Icon className="size-4 text-muted-foreground/80" />
          </div>
          <div className="text-[32px] font-semibold leading-none tracking-[-0.02em] tabular-nums">{value}</div>
          <div
            className={cn(
              "mt-2 text-[13px] text-muted-foreground",
              intent === "good" && "text-emerald-600 dark:text-emerald-400",
            )}
          >
            {detail}
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

function IncidentDetail({ incident }: { incident: Incident }) {
  return (
    <Card className="xl:row-span-2">
      <CardHeader className="border-b bg-card/70 pb-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Selected incident</CardDescription>
            <CardTitle className="text-[15px] leading-snug">
              {incident.id} · {incident.service}
            </CardTitle>
          </div>
          <Badge variant="outline" className={severityClass[incident.severity]}>
            {incident.severity}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4 pt-4">
        <Detail label="Root cause" value={incident.rootCause} />
        <Detail label="Confidence" value={`${incident.confidence}%`} />
        <Detail label="Owner" value={incident.owner} />
        <Detail label="Next action" value={incident.action} testID="detail-action" />
        <Separator />
        <AgentTimeline />
        <div>
          <div className="mb-2 text-xs font-semibold uppercase text-muted-foreground">Signals</div>
          <div className="grid gap-2">
            {incident.signals.map((signal) => (
              <div key={signal} className="rounded-md border bg-muted/50 px-2 py-1 text-sm">
                {signal}
              </div>
            ))}
          </div>
        </div>
        <div className="flex items-center justify-between gap-3 rounded-lg border bg-accent/50 p-3">
          <div className="flex items-center gap-2 text-sm">
            <CircleDot className="size-3 fill-amber-500 text-amber-500" />
            {incident.status === "Awaiting approval" ? incident.action : "No action pending"}
          </div>
          <Button variant="outline" disabled={incident.status !== "Awaiting approval"}>
            Approve
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function IncidentTrendChart() {
  return (
    <div className="h-56" aria-label="Incident volume bar chart">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={incidentTrend} margin={{ left: -28, right: 4, top: 8, bottom: 0 }}>
          <defs>
            <linearGradient id="incident-fill" x1="0" x2="0" y1="0" y2="1">
              <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.28} />
              <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <XAxis dataKey="day" tickLine={false} axisLine={false} minTickGap={18} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
          <YAxis hide />
          <Tooltip
            cursor={{ stroke: "hsl(var(--border))" }}
            contentStyle={{
              borderRadius: 8,
              borderColor: "hsl(var(--border))",
              background: "hsl(var(--card))",
              color: "hsl(var(--foreground))",
              boxShadow: "0 12px 32px rgba(15, 23, 42, 0.12)",
            }}
          />
          <Area
            type="monotone"
            dataKey="incidents"
            stroke="#8b5cf6"
            strokeWidth={2}
            fill="url(#incident-fill)"
            activeDot={{ r: 4 }}
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="noise"
            stroke="#10b981"
            strokeWidth={2}
            fillOpacity={0}
            strokeDasharray="4 4"
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

function SeverityDonut() {
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
              <Pie
                data={severitySplit}
                dataKey="value"
                nameKey="severity"
                innerRadius={42}
                outerRadius={58}
                paddingAngle={5}
                stroke="none"
                isAnimationActive={false}
              >
                {severitySplit.map((item) => (
                  <Cell key={item.severity} fill={item.color} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        </div>
        <div className="grid gap-2">
          {severitySplit.map((item) => (
            <div key={item.severity} className="grid grid-cols-[32px_minmax(0,1fr)_20px] items-center gap-2 text-sm">
              <span className="font-medium">{item.severity}</span>
              <span className="h-2 overflow-hidden rounded-full bg-muted">
                <span className="block h-full rounded-full" style={{ width: `${item.value * 35}%`, backgroundColor: item.color }} />
              </span>
              <span className="text-right text-muted-foreground">{item.value}</span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function SLOBudgetCard() {
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
        {sloBudget.map((item) => (
          <div key={item.service} className="grid gap-1.5">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="font-medium">{item.service}</span>
              <span className="tabular-nums text-muted-foreground">{item.budget}% left</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <motion.div
                className={cn("h-full rounded-full bg-gradient-to-r", sloToneClass[item.tone])}
                initial={{ width: 0 }}
                animate={{ width: `${item.budget}%` }}
                transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
              />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function ActivityStream() {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Control loop</CardDescription>
        <CardTitle className="text-[15px]">Activity</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-1">
        {activity.map((item) => (
          <motion.div
            key={`${item.time}-${item.label}`}
            className="grid grid-cols-[42px_12px_minmax(0,1fr)] items-start gap-2 rounded-md px-1.5 py-1.5 text-sm transition-colors hover:bg-accent/60"
            initial={{ opacity: 0.72, y: 3 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.18 }}
          >
            <span className="text-xs tabular-nums text-muted-foreground">{item.time}</span>
            <span className={cn("mt-1.5 size-2 rounded-full", activityToneClass[item.tone])} />
            <span className="leading-snug">{item.label}</span>
          </motion.div>
        ))}
      </CardContent>
    </Card>
  )
}

function AgentTimeline() {
  return (
    <div>
      <div className="mb-2 text-xs font-semibold uppercase text-muted-foreground">Agent path</div>
      <div className="h-24">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={agentTimeline} layout="vertical" margin={{ left: -36, right: 4, top: 0, bottom: 0 }}>
            <XAxis type="number" hide />
            <YAxis dataKey="label" type="category" width={120} tickLine={false} axisLine={false} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
            <Tooltip
              cursor={{ fill: "hsl(var(--accent))" }}
              contentStyle={{
                borderRadius: 8,
                borderColor: "hsl(var(--border))",
                background: "hsl(var(--card))",
                color: "hsl(var(--foreground))",
              }}
            />
            <Bar dataKey="ms" radius={[0, 6, 6, 0]} fill="#8b5cf6" isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

function Detail({ label, value, testID }: { label: string; value: string; testID?: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 text-sm leading-snug" data-testid={testID}>
        {value}
      </div>
    </div>
  )
}

function SideStacks() {
  return (
    <div className="grid gap-3">
      <Card>
        <CardHeader className="pb-2">
          <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Response library</CardDescription>
          <CardTitle className="text-[15px]">Runbooks</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2">
          {runbooks.map(([title, slug, status]) => (
            <div key={slug} className="grid grid-cols-[1fr_auto] gap-2 rounded-md border bg-background/55 p-2 text-sm">
              <div>
                <div className="font-medium">{title}</div>
                <div className="text-muted-foreground">{slug}</div>
              </div>
              <div className="text-xs text-muted-foreground">{status}</div>
            </div>
          ))}
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Health</CardDescription>
          <CardTitle className="text-[15px]">Integrations</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2">
          {integrations.map(([name, detail, status]) => (
            <div key={name} className="grid grid-cols-[112px_1fr_auto] items-center gap-2 text-sm">
              <div className="font-medium">{name}</div>
              <div className="text-muted-foreground">{detail}</div>
              <Badge variant={status === "degraded" ? "warning" : "secondary"}>{status}</Badge>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}
