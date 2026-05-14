import {
  AlertTriangle,
  BellRing,
  BookOpen,
  CheckCircle2,
  CircleDot,
  Clock3,
  FlaskConical,
  GitBranch,
  KeyRound,
  Layers3,
  Plus,
  RadioTower,
  Save,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  TerminalSquare,
  Trash2,
  Users,
  Webhook,
  X,
} from "lucide-react"
import { AnimatePresence, motion } from "framer-motion"
import { useEffect, useMemo, useState } from "react"
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
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
import type { DashboardData, IntegrationRecord, RunbookRecord } from "@/api"
import type { Incident } from "@/data"
import { cn } from "@/lib/utils"

export type DashboardView = "dashboard" | "incidents" | "runbooks" | "integrations" | "evals" | "agents" | "settings"

type WorkspacePageProps = {
  view: DashboardView
  onNavigate: (view: DashboardView) => void
  data: DashboardData
}

const pageCopy: Record<Exclude<DashboardView, "dashboard">, { eyebrow: string; title: string; description: string }> = {
  incidents: {
    eyebrow: "Incident command",
    title: "Incidents",
    description: "Triage active incidents, inspect evidence, and approve response actions.",
  },
  runbooks: {
    eyebrow: "Response library",
    title: "Runbooks",
    description: "Version controlled automation recipes with approval gates and tool permissions.",
  },
  integrations: {
    eyebrow: "Tool mesh",
    title: "Integrations",
    description: "Connect telemetry, on-call, source control, and remediation systems.",
  },
  evals: {
    eyebrow: "Quality loop",
    title: "Evals",
    description: "Golden tests, real-call samples, latency, accuracy, and failure-mode tracking.",
  },
  agents: {
    eyebrow: "Runtime",
    title: "Agents",
    description: "Worker health, queue pressure, trace spans, and model-router execution.",
  },
  settings: {
    eyebrow: "Tenant control",
    title: "Settings",
    description: "Tenant policy, routing, data retention, guardrails, and audit configuration.",
  },
}

const goldenCases = [
  { suite: "Prometheus alerts", cases: 260, accuracy: 94, falsePositive: 2.1, latency: "1.8s" },
  { suite: "Cloud logs", cases: 220, accuracy: 91, falsePositive: 3.4, latency: "2.4s" },
  { suite: "Kubernetes events", cases: 180, accuracy: 93, falsePositive: 2.8, latency: "1.9s" },
  { suite: "Synthetic chaos", cases: 340, accuracy: 89, falsePositive: 4.9, latency: "3.1s" },
]

const agents = [
  { name: "triage-reactor-0", role: "ReAct triage", queue: 4, status: "healthy", latency: "740ms" },
  { name: "memory-linker-1", role: "Memory retrieval", queue: 1, status: "healthy", latency: "410ms" },
  { name: "runbook-verifier-0", role: "Action verifier", queue: 2, status: "degraded", latency: "1.2s" },
  { name: "routing-guard-0", role: "Policy guard", queue: 0, status: "healthy", latency: "180ms" },
]

const auditRows = [
  ["19:47", "@jane", "approved runbook dry-run", "INC-2041"],
  ["19:42", "paladin-agent", "created RCA hypothesis", "INC-2041"],
  ["19:38", "@alex", "rotated PagerDuty token", "integrations"],
  ["19:29", "paladin-eval", "ran 1000-case golden suite", "evals"],
]

export function WorkspacePage({ view, onNavigate, data }: WorkspacePageProps) {
  if (view === "dashboard") return null
  const copy = pageCopy[view]

  return (
    <div className="grid gap-3">
      <PageHeader copy={copy} view={view} onNavigate={onNavigate} />
      {view === "incidents" ? <IncidentsPage incidents={data.incidents} activity={data.activity} /> : null}
      {view === "runbooks" ? <RunbooksPage runbooks={data.runbooks} /> : null}
      {view === "integrations" ? <IntegrationsPage integrations={data.integrations} /> : null}
      {view === "evals" ? <EvalsPage data={data} /> : null}
      {view === "agents" ? <AgentsPage data={data} /> : null}
      {view === "settings" ? <SettingsPage /> : null}
    </div>
  )
}

function PageHeader({
  copy,
  view,
  onNavigate,
}: {
  copy: { eyebrow: string; title: string; description: string }
  view: Exclude<DashboardView, "dashboard">
  onNavigate: (view: DashboardView) => void
}) {
  const [actionOpen, setActionOpen] = useState(false)
  const actionLabel = view === "integrations" ? "Connect" : "Create"
  return (
    <>
      <div className="flex flex-col gap-3 border-b pb-3 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{copy.eyebrow}</p>
          <h2 className="mt-1 text-[34px] font-semibold leading-none tracking-[-0.02em]">{copy.title}</h2>
          <p className="mt-2 max-w-2xl text-sm text-muted-foreground">{copy.description}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => onNavigate("dashboard")}>
            <Layers3 className="size-4" />
            Overview
          </Button>
          <Button onClick={() => setActionOpen(true)}>
            {view === "integrations" ? <Webhook className="size-4" /> : <Plus className="size-4" />}
            {actionLabel}
          </Button>
        </div>
      </div>
      {view === "integrations" ? (
        <ConfirmDialog
          open={actionOpen}
          title="Connect integration"
          body="Choose a provider, validate credentials, and stage the connector before it receives production traffic."
          onClose={() => setActionOpen(false)}
        />
      ) : (
        <SideDrawer open={actionOpen} title={`${actionLabel} ${copy.title.toLowerCase()}`} onClose={() => setActionOpen(false)}>
          <FormRows rows={["Name", "Owner", "Scope", "Description"]} />
        </SideDrawer>
      )}
    </>
  )
}

function IncidentsPage({ incidents, activity }: { incidents: Incident[]; activity: DashboardData["activity"] }) {
  const [selected, setSelected] = useState<Incident>(incidents[0])
  const [approvalOpen, setApprovalOpen] = useState(false)
  const [query, setQuery] = useState("")

  useEffect(() => {
    if (!incidents.some((incident) => incident.id === selected.id)) {
      setSelected(incidents[0])
    }
  }, [incidents, selected.id])

  const filtered = useMemo(() => {
    const needle = query.toLowerCase()
    return incidents.filter((incident) => `${incident.id} ${incident.title} ${incident.service}`.toLowerCase().includes(needle))
  }, [incidents, query])

  return (
    <section className="grid items-start gap-3 xl:grid-cols-[minmax(0,1fr)_420px]">
      <Card className="overflow-hidden">
        <CardHeader className="flex-row items-center justify-between gap-4 space-y-0 border-b">
          <div>
            <CardDescription>Active response queue</CardDescription>
            <CardTitle>Incident workbench</CardTitle>
          </div>
          <div className="relative w-full max-w-[320px]">
            <Search className="absolute left-2 top-2 size-4 text-muted-foreground" />
            <Input className="pl-8" placeholder="Search incidents" value={query} onChange={(event) => setQuery(event.target.value)} />
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Service</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Owner</TableHead>
                <TableHead>Confidence</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((incident) => (
                <TableRow
                  key={incident.id}
                  className={cn("cursor-pointer", selected.id === incident.id && "bg-accent/70")}
                  onClick={() => setSelected(incident)}
                >
                  <TableCell className="font-medium">{incident.id}</TableCell>
                  <TableCell>{incident.service}</TableCell>
                  <TableCell>{incident.status}</TableCell>
                  <TableCell>{incident.owner}</TableCell>
                  <TableCell>{incident.confidence}%</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <Card className="overflow-hidden">
        <CardHeader className="border-b">
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardDescription>RCA package</CardDescription>
              <CardTitle>{selected.id} · {selected.service}</CardTitle>
            </div>
            <Badge variant="outline">{selected.severity}</Badge>
          </div>
        </CardHeader>
        <CardContent className="grid gap-4 pt-4">
          <InfoBlock title="Root cause" value={selected.rootCause} />
          <InfoBlock title="Recommended action" value={selected.action} />
          <div>
            <div className="mb-2 text-xs font-semibold uppercase text-muted-foreground">Evidence timeline</div>
            <Timeline rows={activity.map((row) => [row.time, row.label])} />
          </div>
          <div className="rounded-lg border bg-muted/40 p-3">
            <div className="mb-2 flex items-center gap-2 text-sm font-medium">
              <ShieldCheck className="size-4 text-emerald-500" />
              Approval gate
            </div>
            <p className="text-sm text-muted-foreground">Requires service owner approval and runbook dry-run verification before remediation.</p>
            <Button className="mt-3" onClick={() => setApprovalOpen(true)} disabled={selected.status !== "Awaiting approval"}>
              Approve remediation
            </Button>
          </div>
        </CardContent>
      </Card>
      <ConfirmDialog
        open={approvalOpen}
        title="Approve remediation"
        body={`Approve "${selected.action}" for ${selected.id}? This records an audit entry and releases the runbook executor.`}
        onClose={() => setApprovalOpen(false)}
      />
    </section>
  )
}

function RunbooksPage({ runbooks }: { runbooks: RunbookRecord[] }) {
  const [drawerOpen, setDrawerOpen] = useState(false)
  return (
    <section className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_380px]">
      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <div>
            <CardDescription>Versioned automation</CardDescription>
            <CardTitle>Runbook catalog</CardTitle>
          </div>
          <Button onClick={() => setDrawerOpen(true)}>
            <Plus className="size-4" />
            New runbook
          </Button>
        </CardHeader>
        <CardContent className="grid gap-2">
          {runbooks.map((runbook) => (
            <div key={runbook.id} className="grid gap-2 rounded-lg border bg-background/55 p-3 md:grid-cols-[1fr_auto_auto] md:items-center">
              <div>
                <div className="font-medium">{runbook.title}</div>
                <div className="text-sm text-muted-foreground">{runbook.slug}</div>
              </div>
              <Badge variant={runbook.status === "pending review" ? "warning" : "secondary"}>{runbook.status}</Badge>
              <Button variant="outline" size="sm">Edit</Button>
            </div>
          ))}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardDescription>Readiness</CardDescription>
          <CardTitle>Publishing checklist</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          {["Dry-run coverage", "Rollback command", "Owner approval", "Blast-radius guard"].map((item) => (
            <div key={item} className="flex items-center justify-between rounded-md border bg-muted/40 p-2 text-sm">
              <span>{item}</span>
              <CheckCircle2 className="size-4 text-emerald-500" />
            </div>
          ))}
          <EmptyState title="No blocked runbooks" body="Every production runbook has an owner, dry-run, and rollback command." />
        </CardContent>
      </Card>
      <SideDrawer open={drawerOpen} title="Create runbook" onClose={() => setDrawerOpen(false)}>
        <FormRows rows={["Name", "Service", "Trigger labels", "Rollback command"]} />
      </SideDrawer>
    </section>
  )
}

function IntegrationsPage({ integrations }: { integrations: IntegrationRecord[] }) {
  const [dialogOpen, setDialogOpen] = useState(false)
  return (
    <section className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_380px]">
      <div className="grid gap-3 md:grid-cols-2">
        {integrations.map((integration) => (
          <Card key={integration.name}>
            <CardHeader>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <CardDescription>Connector</CardDescription>
                  <CardTitle>{integration.name}</CardTitle>
                </div>
                <Badge variant={integration.status === "degraded" ? "warning" : "secondary"}>{integration.status}</Badge>
              </div>
            </CardHeader>
            <CardContent className="grid gap-3">
              <div className="text-sm text-muted-foreground">{integration.detail}</div>
              <Button variant="outline" onClick={() => setDialogOpen(true)}>Configure</Button>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardDescription>Security</CardDescription>
          <CardTitle>Connector policy</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <PolicyRow icon={KeyRound} title="Secret rotation" value="every 30 days" />
          <PolicyRow icon={ShieldCheck} title="Scoped tokens" value="required" />
          <PolicyRow icon={BellRing} title="Degraded alerts" value="PagerDuty + Slack" />
        </CardContent>
      </Card>
      <ConfirmDialog
        open={dialogOpen}
        title="Configure integration"
        body="Connection changes are staged locally first, then validated against health probes before becoming active."
        onClose={() => setDialogOpen(false)}
      />
    </section>
  )
}

function EvalsPage({ data }: { data: DashboardData }) {
  return (
    <section className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <Card>
        <CardHeader>
          <CardDescription>1000 golden cases</CardDescription>
          <CardTitle>Accuracy and latency</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={data.qualityTrend} margin={{ left: -28, right: 8, top: 8, bottom: 0 }}>
                <XAxis dataKey="time" tickLine={false} axisLine={false} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
                <YAxis hide />
                <Tooltip />
                <Area dataKey="accuracy" type="monotone" stroke="#10b981" fill="#10b981" fillOpacity={0.18} strokeWidth={2} isAnimationActive={false} />
                <Area dataKey="latency" type="monotone" stroke="#8b5cf6" fill="#8b5cf6" fillOpacity={0.12} strokeWidth={2} isAnimationActive={false} />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardDescription>Model router</CardDescription>
          <CardTitle>Tier mix</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          {data.modelMix.map((model) => (
            <div key={model.tier}>
              <div className="mb-1 flex justify-between text-sm"><span>{model.tier}</span><span>{model.share}%</span></div>
              <div className="h-2 rounded-full bg-muted"><div className="h-full rounded-full bg-primary" style={{ width: `${model.share}%` }} /></div>
            </div>
          ))}
        </CardContent>
      </Card>
      <Card className="xl:col-span-2">
        <CardHeader>
          <CardDescription>Suites</CardDescription>
          <CardTitle>Golden test coverage</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow><TableHead>Suite</TableHead><TableHead>Cases</TableHead><TableHead>Accuracy</TableHead><TableHead>False positive</TableHead><TableHead>Latency</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {goldenCases.map((suite) => (
                <TableRow key={suite.suite}>
                  <TableCell>{suite.suite}</TableCell><TableCell>{suite.cases}</TableCell><TableCell>{suite.accuracy}%</TableCell><TableCell>{suite.falsePositive}%</TableCell><TableCell>{suite.latency}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </section>
  )
}

function AgentsPage({ data }: { data: DashboardData }) {
  return (
    <section className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <Card>
        <CardHeader>
          <CardDescription>NATS JetStream</CardDescription>
          <CardTitle>Worker queues</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2">
          {agents.map((agent) => (
            <div key={agent.name} className="grid gap-2 rounded-lg border bg-background/55 p-3 md:grid-cols-[1fr_auto_auto] md:items-center">
              <div><div className="font-medium">{agent.name}</div><div className="text-sm text-muted-foreground">{agent.role}</div></div>
              <Badge variant={agent.status === "degraded" ? "warning" : "secondary"}>{agent.status}</Badge>
              <div className="text-sm text-muted-foreground">queue {agent.queue} · {agent.latency}</div>
            </div>
          ))}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardDescription>Trace spans</CardDescription>
          <CardTitle>Current RCA path</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data.agentTimeline} layout="vertical" margin={{ left: -28, right: 8, top: 0, bottom: 0 }}>
                <XAxis type="number" hide />
                <YAxis dataKey="label" type="category" width={120} tickLine={false} axisLine={false} tick={{ fill: "hsl(var(--muted-foreground))", fontSize: 11 }} />
                <Tooltip />
                <Bar dataKey="ms" fill="#8b5cf6" radius={[0, 6, 6, 0]} isAnimationActive={false} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
    </section>
  )
}

function SettingsPage() {
  return (
    <section className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_380px]">
      <Card>
        <CardHeader>
          <CardDescription>Tenant</CardDescription>
          <CardTitle>acme-prod policy</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <FormRows rows={["Tenant slug", "Default region", "Retention days", "Escalation channel"]} />
          <Button className="w-fit"><Save className="size-4" />Save policy</Button>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardDescription>Guardrails</CardDescription>
          <CardTitle>Runtime controls</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <PolicyRow icon={SlidersHorizontal} title="Auto remediation" value="approval required" />
          <PolicyRow icon={Users} title="Tenant isolation" value="strict" />
          <PolicyRow icon={Trash2} title="Data deletion" value="queued for admin review" />
        </CardContent>
      </Card>
      <Card className="xl:col-span-2">
        <CardHeader>
          <CardDescription>Audit log</CardDescription>
          <CardTitle>Recent changes</CardTitle>
        </CardHeader>
        <CardContent>
          <Timeline rows={auditRows.map(([time, actor, action, target]) => [time, `${actor} · ${action} · ${target}`])} />
        </CardContent>
      </Card>
    </section>
  )
}

function InfoBlock({ title, value }: { title: string; value: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase text-muted-foreground">{title}</div>
      <div className="mt-1 text-sm">{value}</div>
    </div>
  )
}

function Timeline({ rows }: { rows: string[][] }) {
  return (
    <div className="grid gap-2">
      {rows.map(([time, label]) => (
        <div key={`${time}-${label}`} className="grid grid-cols-[48px_12px_minmax(0,1fr)] items-start gap-2 text-sm">
          <span className="tabular-nums text-muted-foreground">{time}</span>
          <CircleDot className="mt-1 size-3 fill-primary text-primary" />
          <span>{label}</span>
        </div>
      ))}
    </div>
  )
}

function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-lg border border-dashed bg-background/55 p-4">
      <div className="font-medium">{title}</div>
      <div className="mt-1 text-sm text-muted-foreground">{body}</div>
    </div>
  )
}

function PolicyRow({ icon: Icon, title, value }: { icon: typeof AlertTriangle; title: string; value: string }) {
  return (
    <div className="grid grid-cols-[32px_1fr] gap-3 rounded-lg border bg-background/55 p-3">
      <div className="grid size-8 place-items-center rounded-md bg-accent text-accent-foreground"><Icon className="size-4" /></div>
      <div><div className="font-medium">{title}</div><div className="text-sm text-muted-foreground">{value}</div></div>
    </div>
  )
}

function FormRows({ rows }: { rows: string[] }) {
  return (
    <div className="grid gap-3">
      {rows.map((row) => (
        <label key={row} className="grid gap-1 text-sm">
          <span className="font-medium">{row}</span>
          <Input placeholder={row} />
        </label>
      ))}
    </div>
  )
}

function ConfirmDialog({ open, title, body, onClose }: { open: boolean; title: string; body: string; onClose: () => void }) {
  return (
    <AnimatePresence>
      {open ? (
        <motion.div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/30 px-4 backdrop-blur-sm" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
          <motion.div role="dialog" aria-label={title} className="w-full max-w-md rounded-lg border bg-card p-4 shadow-2xl" initial={{ y: 8, scale: 0.98 }} animate={{ y: 0, scale: 1 }} exit={{ y: 8, scale: 0.98 }}>
            <div className="flex items-start justify-between gap-3">
              <div><h3 className="font-semibold">{title}</h3><p className="mt-2 text-sm text-muted-foreground">{body}</p></div>
              <Button variant="ghost" size="icon" onClick={onClose} aria-label="Close dialog"><X className="size-4" /></Button>
            </div>
            <Separator className="my-4" />
            <div className="flex justify-end gap-2"><Button variant="outline" onClick={onClose}>Cancel</Button><Button onClick={onClose}>Confirm</Button></div>
          </motion.div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  )
}

function SideDrawer({ open, title, children, onClose }: { open: boolean; title: string; children: React.ReactNode; onClose: () => void }) {
  return (
    <AnimatePresence>
      {open ? (
        <motion.div className="fixed inset-0 z-50 bg-slate-950/30 backdrop-blur-sm" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
          <motion.aside className="ml-auto h-full w-full max-w-md border-l bg-card p-4 shadow-2xl" initial={{ x: 32 }} animate={{ x: 0 }} exit={{ x: 32 }}>
            <div className="mb-4 flex items-center justify-between"><h3 className="font-semibold">{title}</h3><Button variant="ghost" size="icon" onClick={onClose} aria-label="Close drawer"><X className="size-4" /></Button></div>
            {children}
            <div className="mt-4 flex justify-end gap-2"><Button variant="outline" onClick={onClose}>Cancel</Button><Button onClick={onClose}>Save</Button></div>
          </motion.aside>
        </motion.div>
      ) : null}
    </AnimatePresence>
  )
}
