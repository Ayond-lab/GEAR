import { viewDefinitions } from "./contracts.js";

const state = {
  mandate: null,
  escalations: null,
  audit: null,
  evidence: null,
  privacy: null,
  latency: null,
  activeView: "mandate-derivation"
};

const tabs = document.querySelector("#tabs");
const runtimeStatus = document.querySelector("#runtime-status");

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function fetchJSON(path) {
  const response = await fetch(path, { headers: { accept: "application/json" } });
  if (!response.ok) {
    throw new Error(`${path} returned ${response.status}`);
  }
  return response.json();
}

function renderTabs() {
  tabs.innerHTML = viewDefinitions
    .map(
      (view) => `<button class="tab ${view.id === state.activeView ? "active" : ""}" id="tab-${view.id}" data-view="${view.id}" type="button">${escapeHTML(view.label)}</button>`
    )
    .join("");
  tabs.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      state.activeView = tab.dataset.view;
      document.querySelectorAll(".view").forEach((view) => view.classList.remove("active"));
      document.querySelector(`#view-${state.activeView}`).classList.add("active");
      renderTabs();
    });
  });
}

function metric(label, value, tone = "") {
  return `<div class="metric"><div class="metric-label">${escapeHTML(label)}</div><div class="metric-value ${tone}">${escapeHTML(value)}</div></div>`;
}

function pill(value) {
  return `<span class="pill ${escapeHTML(String(value).toLowerCase())}">${escapeHTML(value)}</span>`;
}

function renderMandate() {
  const target = document.querySelector("#view-mandate-derivation");
  const data = state.mandate;
  if (!data) {
    target.innerHTML = loadingBand("Mandate");
    return;
  }
  const grants = data.actionGrants
    .map((grant) => `<tr><td>${escapeHTML(grant.class)}</td><td>${pill(grant.disposition)}</td></tr>`)
    .join("");
  const clauses = data.clauses
    .map((clause) => `<li><strong>${escapeHTML(clause.ID)}</strong><div>${escapeHTML(clause.Text)}</div><div class="muted">${escapeHTML(clause.Reason)}</div></li>`)
    .join("");
  const alternatives = (data.refusal?.alternatives ?? [])
    .map((item) => `<li>${escapeHTML(item)}</li>`)
    .join("");
  target.innerHTML = `
    <section class="band">
      <div class="band-header"><h2>Mandate Derivation</h2><span>${pill(data.narrowedMandate?.spec?.mandateId ?? "MND-2026-021")}</span></div>
      <div class="band-body metrics">
        ${metric("Ability", data.abilityRef)}
        ${metric("Version", data.abilityVersion)}
        ${metric("Manifest", data.manifestDigest)}
        ${metric("Daily cap", data.caps?.dailyActions ?? 0)}
        ${metric("Threshold", data.thresholds?.extractionConfidence ?? "0.70")}
      </div>
    </section>
    <div class="grid-2">
      <section class="band">
        <div class="band-header"><h3>Refusal Record</h3><span>${pill("refused")}</span></div>
        <div class="band-body">
          <div class="metrics">
            ${metric("Purpose", data.refusedPurposeLabel)}
            ${metric("Criterion", data.refusal?.criterion ?? "")}
            ${metric("Verb", data.refusal?.verb ?? "")}
            ${metric("Audit ref", data.refusalAuditRef)}
          </div>
          <ul class="field-list">${alternatives}</ul>
        </div>
      </section>
      <section class="band">
        <div class="band-header"><h3>Action Grants</h3><span>${pill("subsumed")}</span></div>
        <div class="band-body table-wrap"><table><thead><tr><th>Class</th><th>Disposition</th></tr></thead><tbody>${grants}</tbody></table></div>
      </section>
    </div>
    <section class="band">
      <div class="band-header"><h3>Clause Derivations</h3><span>${pill("signed")}</span></div>
      <div class="band-body"><ul class="clause-list">${clauses}</ul></div>
    </section>
    <section class="band">
      <div class="band-header"><h3>Policy Boundary</h3><span>${pill("10 fields")}</span></div>
      <div class="band-body grid-2">
        <ul class="field-list">${data.policyFields.map((field) => `<li>${escapeHTML(field)}</li>`).join("")}</ul>
        <ul class="field-list">${data.hiddenInputs.map((field) => `<li>${escapeHTML(field)}</li>`).join("")}</ul>
      </div>
    </section>`;
}

function renderEscalations() {
  const target = document.querySelector("#view-escalation-queue");
  const data = state.escalations;
  if (!data) {
    target.innerHTML = loadingBand("Escalations");
    return;
  }
  const rows = data.items
    .map(
      (item) => `<tr>
        <td>${escapeHTML(item.applicationId)}</td>
        <td>${escapeHTML(item.actionRef)}</td>
        <td>${escapeHTML(item.confidence)}</td>
        <td>${pill(item.ruleFired.id)}</td>
        <td>${pill(item.status)}</td>
        <td><div class="button-row">
          <button class="command" data-decision="approve" data-action="${escapeHTML(item.actionRef)}" type="button">Approve</button>
          <button class="command" data-decision="decline" data-action="${escapeHTML(item.actionRef)}" type="button">Decline</button>
          <button class="command" data-decision="info" data-action="${escapeHTML(item.actionRef)}" type="button">Info</button>
        </div></td>
      </tr>`
    )
    .join("");
  target.innerHTML = `
    <section class="band">
      <div class="band-header"><h2>Escalation Queue</h2><span>${pill(`${data.summary.pendingEscalations} pending`)}</span></div>
      <div class="band-body metrics">
        ${metric("Applications", data.summary.applications)}
        ${metric("Triggered", data.summary.triggeredActions)}
        ${metric("Authorised", data.summary.authorised, "authorise")}
        ${metric("Escalated", data.summary.escalated, "escalate")}
      </div>
    </section>
    <section class="band">
      <div class="band-header"><h3>Unclear Cases</h3><span>${pill("human review")}</span></div>
      <div class="band-body table-wrap"><table><thead><tr><th>Application</th><th>Action ref</th><th>Confidence</th><th>Rule</th><th>Status</th><th>Decision</th></tr></thead><tbody>${rows}</tbody></table></div>
    </section>`;
  target.querySelectorAll("button.command").forEach((button) => {
    button.addEventListener("click", () => decideEscalation(button.dataset.action, button.dataset.decision));
  });
}

async function decideEscalation(actionRef, decision) {
  const reasonRef = `fixture://synthetic-cv-lab/reasons/${encodeURIComponent(actionRef)}-${decision}`;
  const response = await fetch("/api/escalations/decision", {
    method: "POST",
    headers: { "content-type": "application/json", accept: "application/json" },
    body: JSON.stringify({ actionRef, decision, decidedBy: "hiring-manager-1", reasonRef })
  });
  if (!response.ok) {
    runtimeStatus.textContent = "Decision rejected";
    return;
  }
  state.escalations = await fetchJSON("/api/escalations");
  runtimeStatus.textContent = "Decision recorded";
  renderEscalations();
}

function renderEvidence() {
  const target = document.querySelector("#view-evidence-audit");
  if (!state.audit || !state.evidence || !state.privacy || !state.latency) {
    target.innerHTML = loadingBand("Evidence");
    return;
  }
  const criteria = state.evidence.criteria
    .map((item) => `<tr><td>${escapeHTML(item.id)}</td><td>${pill(item.verdict)}</td><td>${escapeHTML(item.latestRun || "")}</td><td>${escapeHTML((item.files || []).length)}</td></tr>`)
    .join("");
  const entries = state.audit.entries
    .slice(0, 30)
    .map((entry) => `<tr><td>${escapeHTML(entry.seq)}</td><td>${escapeHTML(entry.type)}</td><td>${escapeHTML(entry.actionRef)}</td><td>${pill(entry.decision)}</td><td>${escapeHTML(entry.rule)}</td><td>${escapeHTML(entry.inputsDigest)}</td></tr>`)
    .join("");
  target.innerHTML = `
    <section class="band">
      <div class="band-header"><h2>Evidence And Audit</h2><span>${pill(state.audit.verification.ok ? "verified" : "failed")}</span></div>
      <div class="band-body metrics">
        ${metric("Audit entries", state.audit.entries.length)}
        ${metric("Decisions", state.audit.summary.decisionEntries)}
        ${metric("Effects", state.audit.summary.effectEntries)}
        ${metric("Privacy scan", state.privacy.ok ? "clear" : "finding", state.privacy.ok ? "pass" : "fail")}
        ${metric("A8 p95 us", state.latency.p95Micros)}
      </div>
    </section>
    <div class="grid-2">
      <section class="band">
        <div class="band-header"><h3>Acceptance Evidence</h3><span>${pill("A1-A10")}</span></div>
        <div class="band-body table-wrap"><table><thead><tr><th>ID</th><th>Verdict</th><th>Latest run</th><th>Files</th></tr></thead><tbody>${criteria}</tbody></table></div>
      </section>
      <section class="band">
        <div class="band-header"><h3>Latency Histogram</h3><span>${pill(`${state.latency.trials} trials`)}</span></div>
        <div class="band-body"><div class="histogram">${renderHistogram(state.latency.histogram)}</div></div>
      </section>
    </div>
    <section class="band">
      <div class="band-header"><h3>Audit Entries</h3><span>${pill("sample")}</span></div>
      <div class="band-body table-wrap"><table><thead><tr><th>Seq</th><th>Type</th><th>Action</th><th>Decision</th><th>Rule</th><th>Input digest</th></tr></thead><tbody>${entries}</tbody></table></div>
    </section>`;
}

function renderHistogram(histogram) {
  const max = Math.max(1, ...histogram.map((bucket) => bucket.count));
  return histogram
    .map((bucket) => {
      const label = bucket.upperBoundMicros < 0 ? ">50000" : `<=${bucket.upperBoundMicros}`;
      const width = Math.max(2, Math.round((bucket.count / max) * 100));
      return `<div class="bar-row"><span>${escapeHTML(label)}</span><div class="bar-track"><div class="bar-fill" style="width:${width}%"></div></div><span>${escapeHTML(bucket.count)}</span></div>`;
    })
    .join("");
}

function loadingBand(title) {
  return `<section class="band"><div class="band-header"><h2>${escapeHTML(title)}</h2><span>${pill("loading")}</span></div><div class="band-body empty">Loading</div></section>`;
}

function renderAll() {
  renderTabs();
  renderMandate();
  renderEscalations();
  renderEvidence();
}

async function boot() {
  renderAll();
  try {
    const [mandate, escalations, audit, evidence, privacy, latency] = await Promise.all([
      fetchJSON("/api/mandate"),
      fetchJSON("/api/escalations"),
      fetchJSON("/api/audit"),
      fetchJSON("/api/evidence"),
      fetchJSON("/api/privacy-scan"),
      fetchJSON("/api/latency")
    ]);
    Object.assign(state, { mandate, escalations, audit, evidence, privacy, latency });
    runtimeStatus.textContent = "Ready";
  } catch (error) {
    runtimeStatus.textContent = "API unavailable";
    console.error(error);
  }
  renderAll();
}

boot();
