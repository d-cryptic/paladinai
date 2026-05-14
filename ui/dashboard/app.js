const data = {
  incidents: [
    {
      id: "INC-2041",
      severity: "P1",
      title: "Payment DB connection exhaustion",
      service: "payments-api",
      age: "4m",
      status: "triaging",
      agent: "RCA running",
      owner: "@jane",
      rootCause: "Connection pool saturation after deploy v2.4.1.",
      confidence: 91,
      action: "Restart payments-api after pre-hook checks pass",
    },
    {
      id: "INC-2039",
      severity: "P2",
      title: "Auth latency spike",
      service: "auth-service",
      age: "18m",
      status: "awaiting approval",
      agent: "Action proposed",
      owner: "@alex",
      rootCause: "Redis saturation on token introspection path.",
      confidence: 84,
      action: "Scale auth-cache read replicas",
    },
    {
      id: "INC-2037",
      severity: "P3",
      title: "S3 error rate elevated",
      service: "uploads",
      age: "1h",
      status: "resolved",
      agent: "Closed",
      owner: "@bot",
      rootCause: "Transient provider errors correlated with us-east-1.",
      confidence: 78,
      action: "No approval pending",
    },
    {
      id: "INC-2034",
      severity: "P3",
      title: "Worker memory pressure",
      service: "job-queue",
      age: "2h",
      status: "triaging",
      agent: "Memory specialist",
      owner: "@sam",
      rootCause: "Long-running batch workers retaining payload buffers.",
      confidence: 73,
      action: "Drain worker-7 and roll batch pool",
    },
  ],
  volume: [3, 4, 6, 5, 4, 7, 9, 8, 7, 5, 4, 6, 8, 10],
  runbooks: [
    ["payments-db-pool", "Pool exhaustion", "embedded"],
    ["auth-cache-latency", "Redis latency", "embedded"],
    ["worker-memory-pressure", "Memory pressure", "pending review"],
  ],
  integrations: [
    ["Prometheus", "enabled", "12 tools"],
    ["PagerDuty", "enabled", "8 tools"],
    ["GitHub", "enabled", "6 tools"],
    ["Datadog", "degraded", "token refresh needed"],
  ],
};

let selectedID = data.incidents[0].id;

function byID(id) {
  return document.getElementById(id);
}

function textElement(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  node.textContent = text;
  return node;
}

function tableCell(text) {
  const cell = document.createElement("td");
  cell.textContent = text;
  return cell;
}

function renderMetrics() {
  const active = data.incidents.filter((incident) => incident.status !== "resolved");
  const p1 = active.filter((incident) => incident.severity === "P1").length;
  const p2 = active.filter((incident) => incident.severity === "P2").length;
  byID("active-count").textContent = String(active.length);
  byID("active-breakdown").textContent = `${p1} P1, ${p2} P2`;
  byID("nav-incident-count").textContent = String(active.length);
}

function filteredIncidents() {
  const severity = byID("severity-filter").value;
  const status = byID("status-filter").value;
  const search = byID("search-filter").value.trim().toLowerCase();
  return data.incidents.filter((incident) => {
    if (severity !== "all" && incident.severity !== severity) return false;
    if (status !== "all" && incident.status !== status) return false;
    if (search === "") return true;
    return `${incident.title} ${incident.service} ${incident.id}`.toLowerCase().includes(search);
  });
}

function renderIncidents() {
  const rows = byID("incident-rows");
  rows.replaceChildren();
  const incidents = filteredIncidents();
  if (!incidents.some((incident) => incident.id === selectedID) && incidents.length > 0) {
    selectedID = incidents[0].id;
  }
  for (const incident of incidents) {
    const tr = document.createElement("tr");
    tr.dataset.incidentId = incident.id;
    tr.className = incident.id === selectedID ? "selected" : "";
    const severityCell = document.createElement("td");
    severityCell.appendChild(textElement("span", `badge ${incident.severity.toLowerCase()}`, incident.severity));
    tr.appendChild(severityCell);
    tr.appendChild(tableCell(`${incident.id} · ${incident.title}`));
    tr.appendChild(tableCell(incident.service));
    tr.appendChild(tableCell(incident.age));
    tr.appendChild(tableCell(incident.status));
    tr.appendChild(tableCell(incident.agent));
    tr.addEventListener("click", () => {
      selectedID = incident.id;
      renderIncidents();
      renderDetail();
    });
    rows.appendChild(tr);
  }
  renderDetail();
}

function renderDetail() {
  const incident = data.incidents.find((item) => item.id === selectedID) || data.incidents[0];
  byID("detail-title").textContent = `${incident.id} · ${incident.service}`;
  byID("detail-severity").textContent = incident.severity;
  byID("detail-severity").className = `badge ${incident.severity.toLowerCase()}`;
  byID("detail-root").textContent = incident.rootCause;
  byID("detail-confidence").textContent = `${incident.confidence}%`;
  byID("detail-owner").textContent = incident.owner;
  byID("detail-action").textContent = incident.action;
  byID("approval-copy").textContent =
    incident.status === "awaiting approval" ? incident.action : "No action pending";
  byID("approve-button").disabled = incident.status !== "awaiting approval";
}

function renderCharts() {
  const chart = byID("volume-chart");
  chart.replaceChildren();
  const max = Math.max(...data.volume);
  for (const value of data.volume) {
    const bar = document.createElement("span");
    bar.className = "bar";
    bar.style.height = `${Math.max(12, (value / max) * 100)}%`;
    bar.title = `${value} incidents`;
    chart.appendChild(bar);
  }

  const split = byID("severity-split");
  split.replaceChildren();
  const counts = data.incidents.reduce((acc, incident) => {
    acc[incident.severity] = (acc[incident.severity] || 0) + 1;
    return acc;
  }, {});
  for (const severity of ["P1", "P2", "P3", "P4"]) {
    const count = counts[severity] || 0;
    const row = document.createElement("div");
    row.className = "severity-row";
    row.appendChild(textElement("strong", "", severity));
    const track = document.createElement("span");
    track.className = "track";
    const fill = document.createElement("span");
    fill.className = "fill";
    fill.style.width = `${(count / data.incidents.length) * 100}%`;
    track.appendChild(fill);
    row.appendChild(track);
    row.appendChild(textElement("span", "", String(count)));
    split.appendChild(row);
  }
}

function renderLists() {
  const runbooks = byID("runbook-list");
  runbooks.replaceChildren();
  for (const [slug, title, status] of data.runbooks) {
    const item = document.createElement("li");
    item.appendChild(textElement("strong", "", title));
    item.appendChild(textElement("span", "", slug));
    item.appendChild(textElement("small", "", status));
    runbooks.appendChild(item);
  }

  const integrations = byID("integration-list");
  integrations.replaceChildren();
  for (const [name, status, detail] of data.integrations) {
    const item = document.createElement("li");
    item.appendChild(textElement("strong", "", name));
    item.appendChild(textElement("span", "", detail));
    item.appendChild(textElement("small", "", status));
    integrations.appendChild(item);
  }
}

function bindControls() {
  for (const id of ["severity-filter", "status-filter", "search-filter"]) {
    byID(id).addEventListener("input", renderIncidents);
  }
  byID("theme-toggle").addEventListener("click", (event) => {
    const dark = document.documentElement.dataset.theme !== "dark";
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    event.currentTarget.textContent = dark ? "Light" : "Dark";
    event.currentTarget.setAttribute("aria-pressed", String(dark));
  });
}

function init() {
  renderMetrics();
  renderIncidents();
  renderCharts();
  renderLists();
  bindControls();
}

init();
