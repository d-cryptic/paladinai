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
    const [liveIncidents, liveRunbooks] = await Promise.all([
      fetchIncidents(apiBaseURL, token, tenantID, signal),
      fetchRunbooks(apiBaseURL, token, tenantID, signal),
    ])
    const fallback = mockDashboardData("live")
    return {
      ...fallback,
      apiBaseURL,
      wsURL: dashboardWSURL(),
      incidents: liveIncidents.length > 0 ? liveIncidents : fallback.incidents,
      runbooks: liveRunbooks.length > 0 ? liveRunbooks : fallback.runbooks,
    }
  } catch {
    return mockDashboardData("mock")
  }
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

function ageLabel(createdAt?: string): string {
  if (!createdAt) return "now"
  const created = new Date(createdAt).getTime()
  if (!Number.isFinite(created)) return "now"
  const minutes = Math.max(0, Math.round((Date.now() - created) / 60_000))
  if (minutes < 60) return `${minutes}m`
  return `${Math.round(minutes / 60)}h`
}
