export type Incident = {
  id: string
  severity: "P1" | "P2" | "P3" | "P4"
  title: string
  service: string
  age: string
  status: "Triaging" | "Awaiting approval" | "Resolved"
  agent: string
  owner: string
  rootCause: string
  confidence: number
  action: string
  signals: string[]
}

export const incidents: Incident[] = [
  {
    id: "INC-2041",
    severity: "P1",
    title: "Payment DB connection exhaustion",
    service: "payments-api",
    age: "4m",
    status: "Triaging",
    agent: "RCA running",
    owner: "@jane",
    rootCause: "Connection pool saturation after deploy v2.4.1.",
    confidence: 91,
    action: "Restart payments-api after pre-hook checks pass",
    signals: ["pg connections 497/500", "p99 8.2s", "5xx rate 12%"],
  },
  {
    id: "INC-2039",
    severity: "P2",
    title: "Auth latency spike",
    service: "auth-service",
    age: "18m",
    status: "Awaiting approval",
    agent: "Action proposed",
    owner: "@alex",
    rootCause: "Redis saturation on token introspection path.",
    confidence: 84,
    action: "Scale auth-cache read replicas",
    signals: ["cache CPU 91%", "token p95 1.9s", "queue depth 420"],
  },
  {
    id: "INC-2037",
    severity: "P3",
    title: "S3 error rate elevated",
    service: "uploads",
    age: "1h",
    status: "Resolved",
    agent: "Closed",
    owner: "@bot",
    rootCause: "Transient provider errors correlated with us-east-1.",
    confidence: 78,
    action: "No approval pending",
    signals: ["retry success 96%", "provider errors", "latency recovered"],
  },
  {
    id: "INC-2034",
    severity: "P3",
    title: "Worker memory pressure",
    service: "job-queue",
    age: "2h",
    status: "Triaging",
    agent: "Memory specialist",
    owner: "@sam",
    rootCause: "Long-running batch workers retaining payload buffers.",
    confidence: 73,
    action: "Drain worker-7 and roll batch pool",
    signals: ["rss 87%", "gc pause 220ms", "batch age 42m"],
  },
]

export const volume = [3, 4, 6, 5, 4, 7, 9, 8, 7, 5, 4, 6, 8, 10]

export const incidentTrend = [
  { day: "D-13", incidents: 3, noise: 18, cost: 9 },
  { day: "D-12", incidents: 4, noise: 16, cost: 12 },
  { day: "D-11", incidents: 6, noise: 14, cost: 18 },
  { day: "D-10", incidents: 5, noise: 15, cost: 16 },
  { day: "D-9", incidents: 4, noise: 13, cost: 13 },
  { day: "D-8", incidents: 7, noise: 12, cost: 21 },
  { day: "D-7", incidents: 9, noise: 11, cost: 29 },
  { day: "D-6", incidents: 8, noise: 10, cost: 24 },
  { day: "D-5", incidents: 7, noise: 9, cost: 22 },
  { day: "D-4", incidents: 5, noise: 8, cost: 16 },
  { day: "D-3", incidents: 4, noise: 7, cost: 12 },
  { day: "D-2", incidents: 6, noise: 8, cost: 19 },
  { day: "D-1", incidents: 8, noise: 6, cost: 25 },
  { day: "Today", incidents: 10, noise: 5, cost: 42 },
]

export const severitySplit = [
  { severity: "P1", value: 1, color: "#f97316" },
  { severity: "P2", value: 1, color: "#f59e0b" },
  { severity: "P3", value: 2, color: "#8b5cf6" },
  { severity: "P4", value: 0, color: "#cbd5e1" },
]

export const agentTimeline = [
  { label: "Fetch metrics", ms: 340, status: "done" },
  { label: "Search memory", ms: 410, status: "done" },
  { label: "Runbook match", ms: 620, status: "done" },
  { label: "RCA verify", ms: 1260, status: "running" },
]

export const runbooks = [
  ["Pool exhaustion", "payments-db-pool", "embedded"],
  ["Redis latency", "auth-cache-latency", "embedded"],
  ["Memory pressure", "worker-memory-pressure", "pending review"],
]

export const integrations = [
  ["Prometheus", "12 tools", "enabled"],
  ["PagerDuty", "8 tools", "enabled"],
  ["GitHub", "6 tools", "enabled"],
  ["Datadog", "token refresh needed", "degraded"],
]
