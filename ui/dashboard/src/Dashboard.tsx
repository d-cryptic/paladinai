import {
  Activity,
  ArrowRight,
  AlertTriangle,
  Bell,
  BookOpen,
  CheckCircle2,
  CircleDot,
  Command,
  FlaskConical,
  GitBranch,
  LayoutDashboard,
  RadioTower,
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
  modelMix,
  qualityTrend,
  runbooks,
  severitySplit,
  sloBudget,
  type Incident,
} from "@/data"
import { cn } from "@/lib/utils"
import { WorkspacePage, type DashboardView } from "@/pages"
import { loadDashboardData, mockDashboardData, type DashboardData } from "@/api"

const navItems = [
  { label: "Dashboard", view: "dashboard", icon: LayoutDashboard, count: 3 },
  { label: "Incidents", view: "incidents", icon: AlertTriangle },
  { label: "Runbooks", view: "runbooks", icon: BookOpen },
  { label: "Integrations", view: "integrations", icon: GitBranch },
  { label: "Evals", view: "evals", icon: FlaskConical },
  { label: "Agents", view: "agents", icon: RadioTower },
  { label: "Settings", view: "settings", icon: Settings },
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
  const [dashboardData, setDashboardData] = useState<DashboardData>(() => mockDashboardData())
  const [view, setView] = useState<DashboardView>(() => readView())
  const [selectedID, setSelectedID] = useState(dashboardData.incidents[0].id)
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
    const handler = () => setView(readView())
    window.addEventListener("hashchange", handler)
    return () => window.removeEventListener("hashchange", handler)
  }, [])

  useEffect(() => {
    window.localStorage.setItem("paladin-theme", dark ? "dark" : "light")
  }, [dark])

  useEffect(() => {
    const controller = new AbortController()
    loadDashboardData(controller.signal).then((data) => {
      setDashboardData(data)
      setSelectedID((current) => (data.incidents.some((incident) => incident.id === current) ? current : data.incidents[0]?.id ?? ""))
    })
    return () => controller.abort()
  }, [])

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return dashboardData.incidents.filter((incident) => {
      if (severity !== "all" && incident.severity !== severity) return false
      if (!needle) return true
      return `${incident.id} ${incident.title} ${incident.service}`.toLowerCase().includes(needle)
    })
  }, [dashboardData.incidents, severity, query])

  const selected = filtered.find((incident) => incident.id === selectedID) ?? filtered[0] ?? dashboardData.incidents[0]
  const active = dashboardData.incidents.filter((incident) => incident.status !== "Resolved")
  const p1 = active.filter((incident) => incident.severity === "P1").length
  const p2 = active.filter((incident) => incident.severity === "P2").length
  const navigate = (nextView: DashboardView) => {
    setView(nextView)
    window.history.replaceState(null, "", nextView === "dashboard" ? "#" : `#${nextView}`)
  }

  return (
    <div className={cn("min-h-screen bg-background text-foreground", dark && "dark")}>
      <div className="linear-surface grid min-h-screen grid-cols-1 lg:grid-cols-[228px_minmax(0,1fr)]">
        <Sidebar currentView={view} onNavigate={navigate} />
        <main className="min-w-0 px-4 py-4 sm:px-6 lg:px-6">
          <Topbar dark={dark} onTheme={() => setDark((value) => !value)} onCommand={() => setCommandOpen(true)} />
          {view === "dashboard" ? (
            <>
              <OverviewTabs />
              <BackendStatus data={dashboardData} />
              <SystemHealthPanel />

              <motion.section
                aria-label="Incident metrics"
                className="mt-2.5 grid gap-2.5 md:grid-cols-2 xl:grid-cols-4"
                initial="initial"
                animate="animate"
                transition={{ staggerChildren: 0.04 }}
              >
                <Metric title="Active incidents" value={String(active.length)} detail={`${p1} P1, ${p2} P2`} icon={Bell} />
                <Metric title="MTTR this week" value="14m" detail="22% faster" icon={CheckCircle2} intent="good" />
                <Metric title="Alert noise ratio" value="0.31" detail="within target" icon={ShieldCheck} intent="good" />
                <Metric title="LLM cost today" value="$42" detail="tier A: 71%" icon={Sparkles} />
              </motion.section>

              <section className="mt-2.5 grid items-start gap-2.5 xl:grid-cols-[minmax(0,1.55fr)_360px]">
                <div className="grid gap-2.5">
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
                </div>
                <div className="grid gap-2.5">
                  <IncidentDetail incident={selected} />
                  <SeverityDonut />
                  <QualityTrendCard />
                  <ModelMixCard />
                  <SLOBudgetCard />
                  <ActivityStream />
                  <SideStacks />
                </div>
              </section>
            </>
          ) : (
            <WorkspacePage view={view} onNavigate={navigate} data={dashboardData} />
          )}
        </main>
      </div>
      <CommandPalette
        open={commandOpen}
        onOpenChange={setCommandOpen}
        incidents={dashboardData.incidents}
        onSelectIncident={(id) => {
          setSelectedID(id)
          setSeverity("all")
          navigate("dashboard")
        }}
        onSetSeverity={setSeverity}
        onNavigate={navigate}
      />
    </div>
  )
}

function readView(): DashboardView {
  const nextView = window.location.hash.replace("#", "")
  if (["incidents", "runbooks", "integrations", "evals", "agents", "settings"].includes(nextView)) {
    return nextView as DashboardView
  }
  return "dashboard"
}

function Sidebar({ currentView, onNavigate }: { currentView: DashboardView; onNavigate: (view: DashboardView) => void }) {
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
          <button
            key={item.label}
            className={cn(
              "group flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-muted-foreground transition-colors",
              currentView === item.view && "bg-accent text-accent-foreground",
            )}
            type="button"
            onClick={() => onNavigate(item.view as DashboardView)}
          >
            <item.icon className="size-4 transition-colors group-hover:text-foreground" />
            <span className="flex-1">{item.label}</span>
            {item.count ? <span className="text-xs tabular-nums">{item.count}</span> : null}
          </button>
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
    <header className="mb-3 flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
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

function BackendStatus({ data }: { data: DashboardData }) {
  return (
    <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
      <Badge
        variant="secondary"
        className={cn(
          data.mode === "live"
            ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300"
            : "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300",
        )}
      >
        {data.mode === "live" ? "Live backend" : "Mock fallback"}
      </Badge>
      <span>API {data.apiBaseURL}</span>
      <span>WS {data.wsURL}</span>
    </div>
  )
}

function OverviewTabs() {
  const tabs = ["Dallas prod", "Global edge", "AI runtime", "Integrations"]
  return (
    <div className="mb-3 flex min-w-0 items-center gap-5 overflow-x-auto border-b text-sm">
      {tabs.map((tab, index) => (
        <button
          key={tab}
          className={cn(
            "relative h-9 whitespace-nowrap font-medium text-muted-foreground transition-colors hover:text-foreground",
            index === 0 && "text-foreground",
          )}
          type="button"
        >
          {tab}
          {index === 0 ? <span className="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-primary" /> : null}
        </button>
      ))}
    </div>
  )
}

function SystemHealthPanel() {
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
                4 checks green
              </Badge>
            </div>
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_190px]">
              <div>
                <div className="mb-2 flex items-center justify-between gap-3">
                  <div>
                    <div className="text-sm font-semibold">Retrieval accuracy</div>
                    <div className="text-xs text-muted-foreground">Goal 95% · last 60 minutes</div>
                  </div>
                  <div className="rounded-full bg-lime-300 px-2 py-1 text-xs font-semibold text-slate-950">92%</div>
                </div>
                <div className="h-44">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={qualityTrend} margin={{ left: -28, right: 8, top: 10, bottom: 0 }}>
                      <defs>
                        <linearGradient id="hero-accuracy-fill" x1="0" x2="0" y1="0" y2="1">
                          <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.22} />
                          <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.01} />
                        </linearGradient>
                      </defs>
                      <XAxis dataKey="time" tickLine={false} axisLine={false} minTickGap={22} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
                      <YAxis hide domain={[70, 100]} />
                      <Tooltip
                        cursor={{ stroke: "hsl(var(--border))" }}
                        contentStyle={{
                          borderRadius: 8,
                          borderColor: "hsl(var(--border))",
                          background: "hsl(var(--card))",
                          color: "hsl(var(--foreground))",
                          boxShadow: "0 18px 42px rgba(15, 23, 42, 0.14)",
                        }}
                      />
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
              <TokenUsageRing />
            </div>
          </div>
          <div className="grid gap-2.5">
            <SemanticMapCard />
            <ApiHealthCard />
          </div>
        </CardContent>
      </Card>
      <ComputeLoadCard />
    </section>
  )
}

function TokenUsageRing() {
  return (
    <div className="grid place-items-center rounded-lg border bg-background/55 p-4 text-center">
      <div
        className="grid size-32 place-items-center rounded-full"
        style={{
          background:
            "conic-gradient(#c7f72c 0 72%, #8b5cf6 72% 100%), radial-gradient(circle, hsl(var(--card)) 0 58%, transparent 59%)",
        }}
      >
        <div className="grid size-[94px] place-items-center rounded-full bg-card text-center shadow-inner">
          <div>
            <div className="text-[11px] text-muted-foreground">Monthly token</div>
            <div className="text-[11px] text-muted-foreground">usage</div>
          </div>
        </div>
      </div>
      <div className="mt-3 text-[34px] font-semibold leading-none tracking-[-0.02em]">72.4%</div>
      <div className="mt-2 grid w-full gap-1 text-xs text-muted-foreground">
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-violet-500" />Generation</span>
          <span>70%</span>
        </div>
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-lime-300" />Embedding</span>
          <span>30%</span>
        </div>
      </div>
    </div>
  )
}

function SemanticMapCard() {
  return (
    <div className="rounded-lg border bg-background/55 p-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">Semantic map</div>
          <div className="text-xs text-muted-foreground">Cluster A · 5k pts</div>
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

function ApiHealthCard() {
  return (
    <div className="rounded-lg border bg-background/55 p-3">
      <div className="text-sm font-semibold">API health</div>
      <div className="mt-3 flex items-center gap-3">
        <div
          className="grid size-20 place-items-center rounded-full"
          style={{
            background: "conic-gradient(#f97316 0 8%, #e5e7eb 8% 100%)",
          }}
        >
          <div className="grid size-14 place-items-center rounded-full bg-card text-center">
            <span className="text-lg font-semibold">99.9%</span>
          </div>
        </div>
        <div className="text-sm">
          <div className="font-medium">Uptime</div>
          <div className="text-muted-foreground">p99 212ms · no brownouts</div>
        </div>
      </div>
    </div>
  )
}

function ComputeLoadCard() {
  return (
    <Card className="overflow-hidden border-0 bg-gradient-to-br from-cyan-500 via-sky-600 to-slate-950 text-white shadow-[0_24px_80px_rgba(2,132,199,0.32)]">
      <CardContent className="p-5">
        <div className="mb-8 flex items-start justify-between">
          <div>
            <CardDescription className="text-white/70">Compute load</CardDescription>
            <CardTitle className="mt-1 text-[28px]">42% GPU</CardTitle>
          </div>
          <Sparkles className="size-5 text-lime-200" />
        </div>
        <div className="h-52">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={qualityTrend} margin={{ left: -28, right: 8, top: 10, bottom: 0 }}>
              <defs>
                <linearGradient id="compute-fill" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="5%" stopColor="#ffffff" stopOpacity={0.24} />
                  <stop offset="95%" stopColor="#ffffff" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <XAxis dataKey="time" tickLine={false} axisLine={false} minTickGap={20} tick={{ fill: "rgba(255,255,255,0.65)", fontSize: 11 }} />
              <YAxis hide />
              <Tooltip
                cursor={{ stroke: "rgba(255,255,255,0.3)" }}
                contentStyle={{
                  borderRadius: 8,
                  borderColor: "rgba(255,255,255,0.14)",
                  background: "rgba(15,23,42,0.92)",
                  color: "white",
                }}
              />
              <Area type="monotone" dataKey="latency" stroke="#c7f72c" strokeWidth={3} fill="url(#compute-fill)" activeDot={{ r: 5 }} isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}

function CommandPalette({
  open,
  onOpenChange,
  incidents,
  onSelectIncident,
  onSetSeverity,
  onNavigate,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  incidents: Incident[]
  onSelectIncident: (id: string) => void
  onSetSeverity: (severity: string) => void
  onNavigate: (view: DashboardView) => void
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
    { key: "runbooks", title: "Open runbooks", detail: "Response library", action: () => onNavigate("runbooks") },
    { key: "integrations", title: "Open integrations", detail: "Tool health", action: () => onNavigate("integrations") },
    { key: "evals", title: "Open evals", detail: "Golden tests and model quality", action: () => onNavigate("evals") },
    { key: "agents", title: "Open agents", detail: "Workers, traces, and queues", action: () => onNavigate("agents") },
    { key: "settings", title: "Open settings", detail: "Tenant, routing, and audit policy", action: () => onNavigate("settings") },
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

function QualityTrendCard() {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">LLM evals</CardDescription>
            <CardTitle className="text-[15px]">Eval quality</CardTitle>
          </div>
          <div className="rounded-full border bg-background/70 px-2 py-1 text-xs text-muted-foreground">93% acc</div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="h-36" aria-label="Eval quality chart">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={qualityTrend} margin={{ left: -28, right: 4, top: 8, bottom: 0 }}>
              <defs>
                <linearGradient id="accuracy-fill" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.28} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0.02} />
                </linearGradient>
                <linearGradient id="latency-fill" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.22} />
                  <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <XAxis dataKey="time" tickLine={false} axisLine={false} minTickGap={16} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 10 }} />
              <YAxis hide domain={[0, 100]} />
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
                dataKey="accuracy"
                stroke="#10b981"
                strokeWidth={2}
                fill="url(#accuracy-fill)"
                isAnimationActive={false}
              />
              <Area
                type="monotone"
                dataKey="latency"
                stroke="#8b5cf6"
                strokeWidth={2}
                fill="url(#latency-fill)"
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
        <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-emerald-500" />
            accuracy
          </span>
          <span className="flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-violet-500" />
            latency
          </span>
        </div>
      </CardContent>
    </Card>
  )
}

function ModelMixCard() {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2">
        <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Router policy</CardDescription>
        <CardTitle className="text-[15px]">Model mix</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        {modelMix.map((item) => (
          <div key={item.tier} className="grid gap-1.5">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="font-medium">{item.tier}</span>
              <span className="min-w-0 truncate text-muted-foreground">{item.label}</span>
              <span className="tabular-nums">{item.share}%</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <motion.div
                className={cn("h-full rounded-full bg-gradient-to-r", item.color)}
                initial={{ width: 0 }}
                animate={{ width: `${item.share}%` }}
                transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
              />
            </div>
          </div>
        ))}
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
