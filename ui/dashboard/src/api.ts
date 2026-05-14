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

export type BackendMode = "live" | "mock"

export type RunbookRecord = {
  id: string
  title: string
  slug: string
  source: string
  status: string
  embedded: boolean
  updatedAt: string
}

export type IntegrationRecord = {
  name: string
  detail: string
  status: string
}

export type DashboardMetric = {
  apiHealthPct: number
  avgConfidence: number
  avgLatencyMS: number
  computeLoadPct: number
  tokenUsagePct: number
  generationShare: number
  embeddingShare: number
  semanticPoints: number
  checksGreen: number
}

export type DashboardData = {
  mode: BackendMode
  apiBaseURL: string
  wsURL: string
  incidents: Incident[]
  runbooks: RunbookRecord[]
  integrations: IntegrationRecord[]
  incidentTrend: typeof incidentTrend
  severitySplit: typeof severitySplit
  agentTimeline: typeof agentTimeline
  activity: typeof activity
  sloBudget: typeof sloBudget
  qualityTrend: typeof qualityTrend
  modelMix: typeof modelMix
  metrics: DashboardMetric
}

export type ReplayResult = {
  replayID: string
  sourceID: string
  startedAt: string
}

type APIResponse<T> = {
  data?: T
}

const defaultTenantID = "acme-prod"

export function mockDashboardData(mode: BackendMode = "mock"): DashboardData {
  return {
    mode,
    apiBaseURL: dashboardAPIBaseURL(),
    wsURL: dashboardWSURL(),
    incidents,
    runbooks: runbooks.map(([title, slug, status], index) => ({
      id: `runbook-${index + 1}`,
      title,
      slug,
      source: "local",
      status,
      embedded: status === "embedded",
      updatedAt: "2026-05-14T00:00:00Z",
    })),
    integrations: integrations.map(([name, detail, status]) => ({ name, detail, status })),
    incidentTrend,
    severitySplit,
    agentTimeline,
    activity,
    sloBudget,
    qualityTrend,
    modelMix,
    metrics: {
      apiHealthPct: 99.9,
      avgConfidence: 92,
      avgLatencyMS: 212,
      computeLoadPct: 42,
      tokenUsagePct: 72.4,
      generationShare: 70,
      embeddingShare: 30,
      semanticPoints: 5000,
      checksGreen: 4,
    },
  }
}

export async function loadDashboardData(signal?: AbortSignal): Promise<DashboardData> {
  const apiBaseURL = dashboardAPIBaseURL()
  const token = dashboardToken()
  const tenantID = dashboardTenantID()

  if (!token) {
    return mockDashboardData("mock")
  }

  try {
    const [incidentResult, runbookResult, integrationResult] = await Promise.allSettled([
      fetchIncidents(apiBaseURL, token, tenantID, signal),
      fetchRunbooks(apiBaseURL, token, tenantID, signal),
      fetchIntegrations(apiBaseURL, token, tenantID, signal),
    ])

    if (incidentResult.status === "rejected" && runbookResult.status === "rejected" && integrationResult.status === "rejected") {
      return mockDashboardData("mock")
    }

    return liveDashboardData({
      apiBaseURL,
      wsURL: dashboardWSURL(),
      incidents: incidentResult.status === "fulfilled" ? incidentResult.value : [],
      runbooks: runbookResult.status === "fulfilled" ? runbookResult.value : [],
      integrations: integrationResult.status === "fulfilled" ? integrationResult.value : [],
    })
  } catch {
    return mockDashboardData("mock")
  }
}

export function mergeLiveAlert(data: DashboardData, alert: LiveAlert): DashboardData {
  if (data.mode !== "live") return data
  const incident = toDashboardIncident({
    id: alert.fingerprint || `alert-${Date.now()}`,
    status: alert.status || "open",
    severity: alert.severity || "P3",
    title: alert.title || "Live alert",
    alert_count: 1,
    labels: { service: alert.service || "unknown-service" },
    created_at: alert.starts_at || new Date().toISOString(),
  })
  const incidents = [incident, ...data.incidents.filter((item) => item.id !== incident.id)]
  return liveDashboardData({
    apiBaseURL: data.apiBaseURL,
    wsURL: data.wsURL,
    incidents,
    runbooks: data.runbooks,
    integrations: data.integrations,
  })
}

export function dashboardAPIBaseURL(): string {
  return import.meta.env.VITE_PALADIN_API_URL || "http://127.0.0.1:9002/api/v1"
}

export function dashboardWSURL(): string {
  return import.meta.env.VITE_PALADIN_WS_URL || "ws://127.0.0.1:9007/v2/ws/alerts"
}

export function dashboardTenantID(): string {
  return window.localStorage.getItem("paladin-tenant-id") || import.meta.env.VITE_PALADIN_TENANT_ID || defaultTenantID
}

export function dashboardToken(): string {
  return window.localStorage.getItem("paladin-auth-token") || import.meta.env.VITE_PALADIN_AUTH_TOKEN || ""
}

export function dashboardWebSocketProtocols(token = dashboardToken()): string[] {
  if (!token) return ["paladinai.v2"]
  return ["paladinai.v2", `paladinai.jwt.${token}`]
}

export async function requestIncidentReplay(incidentID: string, signal?: AbortSignal): Promise<ReplayResult> {
  const apiBaseURL = dashboardAPIBaseURL()
  const token = dashboardToken()
  const tenantID = dashboardTenantID()
  if (!token) {
    throw new Error("paladin auth token is required")
  }
  const result = await postJSON<ReplayWire>(`${apiBaseURL}/incidents/${encodeURIComponent(incidentID)}/replay`, token, tenantID, signal)
  return {
    replayID: String(result.replay_id || ""),
    sourceID: String(result.source_id || incidentID),
    startedAt: String(result.started_at || ""),
  }
}

async function fetchIncidents(apiBaseURL: string, token: string, tenantID: string, signal?: AbortSignal): Promise<Incident[]> {
  const response = await getJSON<APIResponse<AgentIncident[]>>(`${apiBaseURL}/incidents`, token, tenantID, signal)
  return (response.data ?? []).map(toDashboardIncident)
}

async function fetchRunbooks(apiBaseURL: string, token: string, tenantID: string, signal?: AbortSignal): Promise<RunbookRecord[]> {
  const response = await getJSON<APIResponse<RunbookWire[]>>(`${apiBaseURL}/runbooks`, token, tenantID, signal)
  return (response.data ?? []).map((runbook, index) => ({
    id: String(runbook.id || `runbook-${index + 1}`),
    title: String(runbook.title || "Untitled runbook"),
    slug: String(runbook.slug || runbook.id || runbook.title || `runbook-${index + 1}`),
    source: String(runbook.source || "unknown"),
    status: runbook.embedded ? "embedded" : "pending review",
    embedded: Boolean(runbook.embedded),
    updatedAt: String(runbook.updated_at || ""),
  }))
}

async function fetchIntegrations(apiBaseURL: string, token: string, tenantID: string, signal?: AbortSignal): Promise<IntegrationRecord[]> {
  const response = await getJSON<APIResponse<MCPServerWire[]>>(`${apiBaseURL}/mcp/servers`, token, tenantID, signal)
  return (response.data ?? []).map((server) => ({
    name: String(server.name || server.id || "unknown"),
    detail: `${Array.isArray(server.capabilities) ? server.capabilities.length : 0} tools`,
    status: server.healthy === false ? "degraded" : "enabled",
  }))
}

async function getJSON<T>(url: string, token: string, tenantID: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, {
    headers: {
      Authorization: `Bearer ${token}`,
      "X-Tenant-ID": tenantID,
    },
    signal,
  })
  if (!response.ok) {
    throw new Error(`GET ${url} returned ${response.status}`)
  }
  return response.json() as Promise<T>
}

async function postJSON<T>(url: string, token: string, tenantID: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "X-Tenant-ID": tenantID,
    },
    signal,
  })
  if (!response.ok) {
    throw new Error(`POST ${url} returned ${response.status}`)
  }
  return response.json() as Promise<T>
}

type AgentIncident = {
  id: string
  status: string
  severity: string
  title: string
  alert_count?: number
  labels?: Record<string, string>
  triage_result?: unknown
  created_at?: string
}

type RunbookWire = {
  id?: string
  title?: string
  slug?: string
  source?: string
  embedded?: boolean
  updated_at?: string
}

type ReplayWire = {
  replay_id?: string
  source_id?: string
  started_at?: string
}

type MCPServerWire = {
  id?: string
  name?: string
  capabilities?: string[]
  healthy?: boolean
}

type LiveAlert = {
  fingerprint?: string
  severity?: string
  status?: string
  service?: string
  title?: string
  starts_at?: string
}

function toDashboardIncident(incident: AgentIncident): Incident {
  const labels = incident.labels ?? {}
  const status = normalizeStatus(incident.status)
  const triage = parseTriageResult(incident.triage_result)
  return {
    id: incident.id,
    severity: normalizeSeverity(incident.severity),
    title: incident.title || labels.alertname || "Untitled incident",
    service: labels.service || labels.job || "unknown-service",
    age: ageLabel(incident.created_at),
    status,
    agent: status === "Resolved" ? "Closed" : "RCA running",
    owner: labels.owner || "@unassigned",
    rootCause: triage.rootCause || "Waiting for RCA evidence from paladin-agent.",
    confidence: triage.confidence || 0,
    action: triage.action || "No remediation proposed yet",
    signals: triage.signals.length > 0 ? triage.signals : [`alerts ${incident.alert_count ?? 1}`, `status ${incident.status}`],
    createdAt: incident.created_at,
  }
}

function normalizeSeverity(severity: string): Incident["severity"] {
  const value = severity.toUpperCase()
  if (value === "P1" || value === "CRITICAL") return "P1"
  if (value === "P2" || value === "WARNING" || value === "HIGH") return "P2"
  if (value === "P3" || value === "MEDIUM") return "P3"
  return "P4"
}

function normalizeStatus(status: string): Incident["status"] {
  if (status === "resolved") return "Resolved"
  if (status === "replaying") return "Awaiting approval"
  return "Triaging"
}

function parseTriageResult(raw: unknown): { rootCause: string; confidence: number; action: string; signals: string[] } {
  if (!raw || typeof raw !== "object") {
    return { rootCause: "", confidence: 0, action: "", signals: [] }
  }
  const data = raw as Record<string, unknown>
  return {
    rootCause: stringField(data, "root_cause") || stringField(data, "rootCause"),
    confidence: numberField(data, "confidence"),
    action: stringField(data, "recommended_action") || stringField(data, "action"),
    signals: arrayField(data, "evidence").concat(arrayField(data, "signals")).slice(0, 4),
  }
}

function stringField(data: Record<string, unknown>, key: string): string {
  const value = data[key]
  return typeof value === "string" ? value : ""
}

function numberField(data: Record<string, unknown>, key: string): number {
  const value = data[key]
  return typeof value === "number" && Number.isFinite(value) ? Math.round(value) : 0
}

function arrayField(data: Record<string, unknown>, key: string): string[] {
  const value = data[key]
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : []
}

function liveDashboardData({
  apiBaseURL,
  wsURL,
  incidents,
  runbooks,
  integrations,
}: {
  apiBaseURL: string
  wsURL: string
  incidents: Incident[]
  runbooks: RunbookRecord[]
  integrations: IntegrationRecord[]
}): DashboardData {
  return {
    mode: "live",
    apiBaseURL,
    wsURL,
    incidents,
    runbooks,
    integrations,
    incidentTrend: buildIncidentTrend(incidents),
    severitySplit: buildSeveritySplit(incidents),
    agentTimeline: buildAgentTimeline(incidents, runbooks, integrations),
    activity: buildActivity(incidents),
    sloBudget: buildSLOBudget(incidents),
    qualityTrend: buildQualityTrend(incidents),
    modelMix: buildModelMix(incidents),
    metrics: buildMetrics(incidents, runbooks, integrations),
  }
}

function buildIncidentTrend(items: Incident[]): typeof incidentTrend {
  const now = new Date()
  return Array.from({ length: 14 }, (_, index) => {
    const offset = 13 - index
    const dayStart = new Date(now)
    dayStart.setHours(0, 0, 0, 0)
    dayStart.setDate(dayStart.getDate() - offset)
    const dayEnd = new Date(dayStart)
    dayEnd.setDate(dayEnd.getDate() + 1)
    const dayItems = items.filter((incident) => {
      const created = createdTime(incident)
      return created >= dayStart.getTime() && created < dayEnd.getTime()
    })
    const noise = dayItems.filter((incident) => incident.status === "Resolved" || incident.severity === "P4").length
    return {
      day: offset === 0 ? "Today" : `D-${offset}`,
      incidents: dayItems.length,
      noise,
      cost: Math.max(0, dayItems.reduce((sum, incident) => sum + tierCost(incident), 0)),
    }
  })
}

function buildSeveritySplit(items: Incident[]): typeof severitySplit {
  const colors: Record<Incident["severity"], string> = { P1: "#f97316", P2: "#f59e0b", P3: "#8b5cf6", P4: "#cbd5e1" }
  return (["P1", "P2", "P3", "P4"] as const).map((severity) => ({
    severity,
    value: items.filter((incident) => incident.severity === severity).length,
    color: colors[severity],
  }))
}

function buildAgentTimeline(items: Incident[], liveRunbooks: RunbookRecord[], liveIntegrations: IntegrationRecord[]): typeof agentTimeline {
  const open = items.filter((incident) => incident.status !== "Resolved").length
  const awaiting = items.filter((incident) => incident.status === "Awaiting approval").length
  const embedded = liveRunbooks.filter((runbook) => runbook.embedded).length
  const healthyTools = liveIntegrations.filter((integration) => integration.status !== "degraded").length
  return [
    { label: "Open incidents", ms: Math.max(1, open) * 100, status: open > 0 ? "running" : "done" },
    { label: "Await approvals", ms: Math.max(1, awaiting) * 160, status: awaiting > 0 ? "running" : "done" },
    { label: "Runbook index", ms: Math.max(1, embedded) * 120, status: "done" },
    { label: "Tool health", ms: Math.max(1, healthyTools) * 90, status: "done" },
  ]
}

function buildActivity(items: Incident[]): typeof activity {
  return [...items]
    .sort((a, b) => createdTime(b) - createdTime(a))
    .slice(0, 6)
    .map((incident) => ({
      time: timeLabel(incident.createdAt),
      label: `${incident.status}: ${incident.title}`,
      tone: incident.severity === "P1" ? "critical" : incident.status === "Resolved" ? "success" : "info",
    }))
}

function buildSLOBudget(items: Incident[]): typeof sloBudget {
  const services = [...new Set(items.map((incident) => incident.service))].slice(0, 5)
  return services.map((service) => {
    const serviceIncidents = items.filter((incident) => incident.service === service)
    const burn = serviceIncidents.reduce((sum, incident) => sum + severityBurn(incident.severity), 0)
    const budget = Math.max(0, 100 - burn)
    return { service, budget, tone: budget < 50 ? "warn" : "good" }
  })
}

function buildQualityTrend(items: Incident[]): typeof qualityTrend {
  const now = new Date()
  return Array.from({ length: 7 }, (_, index) => {
    const offset = 6 - index
    const bucketEnd = new Date(now.getTime() - offset * 10 * 60_000)
    const recent = items.filter((incident) => createdTime(incident) <= bucketEnd.getTime()).slice(0, 10)
    const accuracy = recent.length > 0 ? Math.round(recent.reduce((sum, incident) => sum + incident.confidence, 0) / recent.length) : 0
    return {
      time: bucketEnd.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false }),
      accuracy,
      latency: Math.max(0, Math.round(100 - accuracy)),
    }
  })
}

function buildModelMix(items: Incident[]): typeof modelMix {
  const counts = {
    "Tier A": items.filter((incident) => incident.severity === "P4").length,
    "Tier B": items.filter((incident) => incident.severity === "P2" || incident.severity === "P3").length,
    "Tier C": items.filter((incident) => incident.severity === "P1").length,
  }
  const total = Math.max(1, items.length)
  return [
    { tier: "Tier A", label: "qwen3-1.7b", share: Math.round((counts["Tier A"] / total) * 100), color: "from-violet-500 to-fuchsia-500" },
    { tier: "Tier B", label: "qwen3-8b", share: Math.round((counts["Tier B"] / total) * 100), color: "from-emerald-500 to-cyan-500" },
    { tier: "Tier C", label: "deepseek-v3", share: Math.round((counts["Tier C"] / total) * 100), color: "from-amber-500 to-orange-500" },
  ]
}

function buildMetrics(items: Incident[], liveRunbooks: RunbookRecord[], liveIntegrations: IntegrationRecord[]): DashboardMetric {
  const avgConfidence = items.length > 0 ? Math.round(items.reduce((sum, incident) => sum + incident.confidence, 0) / items.length) : 0
  const healthyIntegrations = liveIntegrations.filter((integration) => integration.status !== "degraded").length
  const apiHealthPct = liveIntegrations.length > 0 ? Math.round((healthyIntegrations / liveIntegrations.length) * 1000) / 10 : 100
  const tokenUsagePct = Math.min(100, Math.round(items.reduce((sum, incident) => sum + tierCost(incident), 0) * 10) / 10)
  const generationShare = Math.min(100, Math.round((items.length / Math.max(1, items.length + liveRunbooks.length)) * 100))
  return {
    apiHealthPct,
    avgConfidence,
    avgLatencyMS: Math.max(50, Math.round(1200 - avgConfidence * 10)),
    computeLoadPct: Math.min(100, items.filter((incident) => incident.status !== "Resolved").length * 14 + liveIntegrations.length * 4),
    tokenUsagePct,
    generationShare,
    embeddingShare: 100 - generationShare,
    semanticPoints: liveRunbooks.reduce((sum, runbook) => sum + (runbook.embedded ? 1 : 0), 0),
    checksGreen: healthyIntegrations + liveRunbooks.filter((runbook) => runbook.embedded).length,
  }
}

function createdTime(incident: Incident): number {
  const parsed = incident.createdAt ? new Date(incident.createdAt).getTime() : Number.NaN
  return Number.isFinite(parsed) ? parsed : Date.now()
}

function timeLabel(createdAt?: string): string {
  if (!createdAt) return "now"
  const parsed = new Date(createdAt)
  if (Number.isNaN(parsed.getTime())) return "now"
  return parsed.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false })
}

function severityBurn(severity: Incident["severity"]): number {
  if (severity === "P1") return 28
  if (severity === "P2") return 18
  if (severity === "P3") return 10
  return 4
}

function tierCost(incident: Incident): number {
  if (incident.severity === "P1") return 8
  if (incident.severity === "P2") return 4
  if (incident.severity === "P3") return 2
  return 1
}

function ageLabel(createdAt?: string): string {
  if (!createdAt) return "now"
  const created = new Date(createdAt).getTime()
  if (!Number.isFinite(created)) return "now"
  const minutes = Math.max(0, Math.round((Date.now() - created) / 60_000))
  if (minutes < 60) return `${minutes}m`
  return `${Math.round(minutes / 60)}h`
}
