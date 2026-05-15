import {
  AlertTriangle,
  Bell,
  BookOpen,
  CheckCircle2,
  CircleDot,
  Command,
  FlaskConical,
  GitBranch,
  LayoutDashboard,
  Moon,
  RadioTower,
  Search,
  Settings,
  ShieldCheck,
  Sparkles,
  TerminalSquare,
} from "lucide-react"
import { motion } from "framer-motion"
import { useEffect, useMemo, useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  ActivityStream,
  IncidentTrendChart,
  ModelMixCard,
  QualityTrendCard,
  SLOBudgetCard,
  SeverityDonut,
  SystemHealthPanel,
} from "@/dashboard_charts"
import { CommandPalette, IncidentDetail, Metric, SideStacks, severityClass } from "@/dashboard_controls"
import {
  dashboardToken,
  dashboardWSURL,
  dashboardWebSocketProtocols,
  loadDashboardData,
  mergeLiveAlert,
  mockDashboardData,
  requestIncidentReplay,
  type DashboardData,
} from "@/api"
import { cn } from "@/lib/utils"
import { WorkspacePage, type DashboardView } from "@/pages"

const navItems = [
  { label: "Dashboard", view: "dashboard", icon: LayoutDashboard, count: 3 },
  { label: "Incidents", view: "incidents", icon: AlertTriangle },
  { label: "Runbooks", view: "runbooks", icon: BookOpen },
  { label: "Integrations", view: "integrations", icon: GitBranch },
  { label: "Evals", view: "evals", icon: FlaskConical },
  { label: "Agents", view: "agents", icon: RadioTower },
  { label: "Settings", view: "settings", icon: Settings },
]

type StreamState = "mock" | "connecting" | "connected" | "disconnected" | "error"

export function Dashboard() {
  const [dashboardData, setDashboardData] = useState<DashboardData>(() => mockDashboardData())
  const [view, setView] = useState<DashboardView>(() => readView())
  const [selectedID, setSelectedID] = useState(dashboardData.incidents[0].id)
  const [severity, setSeverity] = useState("all")
  const [query, setQuery] = useState("")
  const [commandOpen, setCommandOpen] = useState(false)
  const [dark, setDark] = useState(() => window.localStorage.getItem("paladin-theme") === "dark")
  const [streamState, setStreamState] = useState<StreamState>(() => (dashboardToken() ? "connecting" : "mock"))
  const [liveAlertCount, setLiveAlertCount] = useState(0)
  const [replayStatus, setReplayStatus] = useState("")

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

  useEffect(() => {
    const token = dashboardToken()
    if (!token) {
      setStreamState("mock")
      return
    }

    setStreamState("connecting")
    const socket = new WebSocket(dashboardWSURL(), dashboardWebSocketProtocols(token))
    socket.onopen = () => setStreamState("connected")
    socket.onmessage = (event) => {
      setLiveAlertCount((count) => count + 1)
      try {
        const payload = JSON.parse(event.data as string)
        setDashboardData((current) => mergeLiveAlert(current, payload))
      } catch {
        // Non-alert frames still prove the stream is alive.
      }
    }
    socket.onerror = () => setStreamState("error")
    socket.onclose = () => setStreamState((state) => (state === "error" ? "error" : "disconnected"))

    return () => {
      socket.close(1000, "dashboard unmount")
    }
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
  const replayIncident = async (incidentID: string) => {
    const result = await requestIncidentReplay(incidentID)
    setReplayStatus(`Replay ${result.replayID || "started"} queued for ${result.sourceID}`)
  }

  return (
    <div className={cn("min-h-screen bg-background text-foreground", dark && "dark")}>
      <div className="linear-surface grid min-h-screen grid-cols-1 lg:grid-cols-[228px_minmax(0,1fr)]">
        <Sidebar currentView={view} onNavigate={navigate} streamState={streamState} liveAlertCount={liveAlertCount} />
        <main className="min-w-0 px-4 py-4 sm:px-6 lg:px-6">
          <Topbar dark={dark} onTheme={() => setDark((value) => !value)} onCommand={() => setCommandOpen(true)} />
          {view === "dashboard" ? (
            <>
              <OverviewTabs />
              <BackendStatus data={dashboardData} />
              <SystemHealthPanel data={dashboardData} />
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
                  <IncidentTable
                    filtered={filtered}
                    selectedID={selected.id}
                    severity={severity}
                    query={query}
                    onSeverity={setSeverity}
                    onQuery={setQuery}
                    onSelect={setSelectedID}
                  />
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
                      <IncidentTrendChart data={dashboardData.incidentTrend} />
                    </CardContent>
                  </Card>
                </div>
                <div className="grid gap-2.5">
                  <IncidentDetail incident={selected} data={dashboardData} replayStatus={replayStatus} onReplayIncident={replayIncident} />
                  <SeverityDonut data={dashboardData.severitySplit} />
                  <QualityTrendCard data={dashboardData.qualityTrend} accuracy={dashboardData.metrics.avgConfidence} />
                  <ModelMixCard data={dashboardData.modelMix} />
                  <SLOBudgetCard data={dashboardData.sloBudget} />
                  <ActivityStream data={dashboardData.activity} />
                  <SideStacks data={dashboardData} />
                </div>
              </section>
            </>
          ) : (
            <WorkspacePage view={view} onNavigate={navigate} data={dashboardData} onReplayIncident={replayIncident} />
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

function Sidebar({
  currentView,
  onNavigate,
  streamState,
  liveAlertCount,
}: {
  currentView: DashboardView
  onNavigate: (view: DashboardView) => void
  streamState: StreamState
  liveAlertCount: number
}) {
  const connected = streamState === "connected"
  const streamLabel = connected ? "Live stream connected" : streamState === "mock" ? "Mock stream" : `Live stream ${streamState}`
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
        {navItems.map((item) => (
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
          <CircleDot className={cn("size-3", connected ? "fill-emerald-500 text-emerald-500" : "fill-amber-500 text-amber-500")} />
          {streamLabel}
        </div>
        <div>{liveAlertCount} live alerts · RCA queue 2 active</div>
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

function IncidentTable({
  filtered,
  selectedID,
  severity,
  query,
  onSeverity,
  onQuery,
  onSelect,
}: {
  filtered: DashboardData["incidents"]
  selectedID: string
  severity: string
  query: string
  onSeverity: (severity: string) => void
  onQuery: (query: string) => void
  onSelect: (id: string) => void
}) {
  return (
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
            onChange={(event) => onSeverity(event.target.value)}
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
              onChange={(event) => onQuery(event.target.value)}
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
                  selectedID === incident.id && "border-l-primary bg-accent/70",
                )}
                onClick={() => onSelect(incident.id)}
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
  )
}
