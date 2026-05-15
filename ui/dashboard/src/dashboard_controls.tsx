import { Activity, ArrowRight, CircleDot, Search, X } from "lucide-react"
import { AnimatePresence, motion } from "framer-motion"
import { useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { type DashboardData } from "@/api"
import { type Incident } from "@/data"
import { cn } from "@/lib/utils"
import { type DashboardView } from "@/pages"
import { AgentTimeline } from "@/dashboard_charts"

export const severityClass: Record<Incident["severity"], string> = {
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

export function CommandPalette({
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

export function Metric({
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
          <div className={cn("mt-2 text-[13px] text-muted-foreground", intent === "good" && "text-emerald-600 dark:text-emerald-400")}>
            {detail}
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

export function IncidentDetail({
  incident,
  data,
  replayStatus,
  onReplayIncident,
}: {
  incident: Incident
  data: DashboardData
  replayStatus: string
  onReplayIncident: (incidentID: string) => Promise<void>
}) {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState("")
  const canReplay = incident.status === "Awaiting approval"
  const replay = async () => {
    if (!canReplay || pending) return
    setPending(true)
    setError("")
    try {
      await onReplayIncident(incident.id)
    } catch {
      setError("Replay request failed")
    } finally {
      setPending(false)
    }
  }

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
        <AgentTimeline data={data.agentTimeline} />
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
        <div className="grid gap-3 rounded-lg border bg-accent/50 p-3">
          <div className="flex items-center gap-2 text-sm">
            <CircleDot className="size-3 fill-amber-500 text-amber-500" />
            {incident.status === "Awaiting approval" ? incident.action : "No action pending"}
          </div>
          <Button className="w-fit" variant="outline" disabled={!canReplay || pending} onClick={replay}>
            {pending ? "Queuing" : "Approve replay"}
          </Button>
        </div>
        {replayStatus ? <div className="rounded-md bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">{replayStatus}</div> : null}
        {error ? <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div> : null}
      </CardContent>
    </Card>
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

export function SideStacks({ data }: { data: DashboardData }) {
  return (
    <div className="grid gap-3">
      <Card>
        <CardHeader className="pb-2">
          <CardDescription className="text-[11px] font-semibold uppercase tracking-[0.08em]">Response library</CardDescription>
          <CardTitle className="text-[15px]">Runbooks</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2">
          {data.runbooks.length === 0 ? <div className="text-sm text-muted-foreground">No runbooks indexed for this tenant.</div> : null}
          {data.runbooks.map((runbook) => (
            <div key={runbook.id} className="grid grid-cols-[1fr_auto] gap-2 rounded-md border bg-background/55 p-2 text-sm">
              <div>
                <div className="font-medium">{runbook.title}</div>
                <div className="text-muted-foreground">{runbook.slug}</div>
              </div>
              <div className="text-xs text-muted-foreground">{runbook.status}</div>
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
          {data.integrations.length === 0 ? <div className="text-sm text-muted-foreground">No MCP servers registered for this tenant.</div> : null}
          {data.integrations.map((integration) => (
            <div key={integration.name} className="grid grid-cols-[112px_1fr_auto] items-center gap-2 text-sm">
              <div className="font-medium">{integration.name}</div>
              <div className="text-muted-foreground">{integration.detail}</div>
              <Badge variant={integration.status === "degraded" ? "warning" : "secondary"}>{integration.status}</Badge>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}
