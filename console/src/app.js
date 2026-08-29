import {
  defaultHRPrompt,
  lawfulAlternativePrompt,
  viewDefinitions,
  workflowStages
} from "./contracts.js";

const state = {
  activeView: "mandate-derivation",
  prompt: defaultHRPrompt,
  assistant: null,
  escalations: null,
  audit: null,
  evidence: null,
  privacy: null,
  latency: null,
  evaluating: false
};

const tabs = document.querySelector("#tabs");
const workflowSteps = document.querySelector("#workflow-steps");
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

async function postJSON(path, body) {
  const response = await fetch(path, {
    method: "POST",
    headers: { "content-type": "application/json", accept: "application/json" },
    body: JSON.stringify(body)
  });
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
    tab.addEventListener("click", () => activateView(tab.dataset.view));
  });
}

function renderWorkflowSteps() {
  const completed = new Set();
  if (state.assistant) {
    ["request", "interpret", "refuse-or-sign", "execute"].forEach((stage) => completed.add(stage));
  }
  if (state.escalations) {
    completed.add("review");
  }
  if (state.evidence && state.audit && state.privacy) {
    completed.add("evidence");
  }

  workflowSteps.innerHTML = workflowStages
    .map((stage, index) => {
      const status = completed.has(stage.id) ? "complete" : "pending";
      return `<li class="${status}"><span>${index + 1}</span><div>${escapeHTML(stage.label)}</div></li>`;
    })
    .join("");
}

function activateView(view) {
  state.activeView = view;
  document.querySelectorAll(".view").forEach((node) => node.classList.remove("active"));
  document.querySelector(`#view-${view}`).classList.add("active");
  renderShell();
}

function renderShell() {
  renderTabs();
  renderWorkflowSteps();
}

function pill(value, tone = "") {
  return `<span class="pill ${tone || toneFor(value)}">${escapeHTML(value)}</span>`;
}

function toneFor(value) {
  const normalized = String(value ?? "").toLowerCase();
  if (["permit", "pass", "authorise", "complete", "approve", "signed", "verified", "clear"].includes(normalized)) {
    return "positive";
  }
  if (["forbid", "deny", "fail", "refused", "missing", "decline", "blocked"].includes(normalized)) {
    return "negative";
  }
  if (["escalate", "pending", "unknown", "info", "limit", "review"].includes(normalized)) {
    return "warning";
  }
  return "";
}

function metric(label, value, tone = "") {
  return `<div class="metric"><span>${escapeHTML(label)}</span><strong class="${tone}">${escapeHTML(value)}</strong></div>`;
}

function renderAssistant() {
  const target = document.querySelector("#view-mandate-derivation");
  const data = state.assistant;
  const prompt = escapeHTML(state.prompt);
  const assistantStatus = data ? data.outcome : "pending";

  target.innerHTML = `
    <section class="assistant-workbench">
      <div class="prompt-panel">
        <div class="section-heading">
          <div>
            <p class="eyebrow">HR request</p>
            <h1>Mandate Assistant</h1>
          </div>
          ${pill(assistantStatus)}
        </div>
        <textarea id="prompt-input" spellcheck="true">${prompt}</textarea>
        <div class="prompt-actions">
          <button class="primary-command" id="evaluate-prompt" type="button">${state.evaluating ? "Evaluating" : "Evaluate mandate"}</button>
          <button class="secondary-command" id="use-lawful-prompt" type="button">Use lawful alternative</button>
        </div>
      </div>
      <div class="summary-panel">
        ${data ? renderExecutiveSummary(data) : loadingBlock("Awaiting mandate evaluation")}
      </div>
    </section>
    ${data ? renderInterpretation(data) : ""}
    ${data ? renderMandateGuardrails(data) : ""}
    ${data ? renderRunSummary(data) : ""}
    ${data ? renderPolicyBoundary(data) : ""}`;

  const input = target.querySelector("#prompt-input");
  input.addEventListener("input", () => {
    state.prompt = input.value;
  });
  target.querySelector("#evaluate-prompt").addEventListener("click", () => evaluatePrompt(state.prompt));
  target.querySelector("#use-lawful-prompt").addEventListener("click", () => {
    state.prompt = lawfulAlternativePrompt;
    evaluatePrompt(lawfulAlternativePrompt);
  });
  target.querySelectorAll("[data-open-view]").forEach((button) => {
    button.addEventListener("click", () => activateView(button.dataset.openView));
  });
}

function renderExecutiveSummary(data) {
  const decisionTone = data.outcome === "refused" ? "negative" : "positive";
  const message =
    data.outcome === "refused"
      ? "GEAR refused the unsafe mandate, then prepared a narrowed lawful mandate for planning."
      : "GEAR signed the narrowed mandate and constrained the agent to governed record annotation.";
  return `
    <div class="decision-banner ${decisionTone}">
      <span>${escapeHTML(data.outcome === "refused" ? "Mandate refused" : "Mandate signed")}</span>
      <strong>${escapeHTML(message)}</strong>
    </div>
    <div class="summary-grid">
      ${metric("Understood action", data.interpretation.actionClass)}
      ${metric("Criterion", data.interpretation.criterion)}
      ${metric("Verb", data.interpretation.verb)}
      ${metric("Audit ref", data.refusalAuditRef || "signed mandate")}
    </div>`;
}

function renderInterpretation(data) {
  const alternatives = (data.refusal?.alternatives ?? [data.recommendedPrompt])
    .map((item) => `<li>${escapeHTML(item)}</li>`)
    .join("");
  return `
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Governance interpretation</p>
          <h2>Natural language does not grant authority</h2>
        </div>
        ${pill(data.interpretation.decision)}
      </div>
      <div class="interpretation-grid">
        <div class="interpretation-copy">
          <p>${escapeHTML(data.interpretation.reason)}</p>
          <p class="muted">${escapeHTML(data.interpretation.executionModel)}</p>
        </div>
        <div class="decision-table-wrap">
          <table>
            <tbody>
              <tr><th>Action class</th><td>${escapeHTML(data.interpretation.actionClass)}</td></tr>
              <tr><th>Criterion</th><td>${escapeHTML(data.interpretation.criterion)}</td></tr>
              <tr><th>Data boundary</th><td>${escapeHTML(data.interpretation.dataBoundary)}</td></tr>
              <tr><th>Assistant rewrite</th><td>${escapeHTML(data.recommendedPrompt)}</td></tr>
            </tbody>
          </table>
        </div>
      </div>
      <ul class="alternative-list">${alternatives}</ul>
    </section>`;
}

function renderMandateGuardrails(data) {
  const guardrails = data.guardrails
    .map(
      (item) => `<li>
        <span>${escapeHTML(item.label)}</span>
        <strong>${escapeHTML(item.value)}</strong>
        ${pill(item.disposition)}
      </li>`
    )
    .join("");
  const grants = data.actionGrants
    .map((grant) => `<tr><td>${escapeHTML(grant.class)}</td><td>${pill(grant.disposition)}</td></tr>`)
    .join("");
  const clauses = data.clauses
    .map((clause) => `<li><strong>${escapeHTML(clause.ID ?? clause.id)}</strong><span>${escapeHTML(clause.Text ?? clause.text)}</span></li>`)
    .join("");

  return `
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Narrowed mandate</p>
          <h2>${escapeHTML(data.recommendedMandate?.spec?.mandateId ?? "MND-2026-021")}</h2>
        </div>
        ${pill("subsumed")}
      </div>
      <div class="two-column">
        <ul class="guardrail-list">${guardrails}</ul>
        <div class="decision-table-wrap">
          <table>
            <thead><tr><th>Action</th><th>Disposition</th></tr></thead>
            <tbody>${grants}</tbody>
          </table>
        </div>
      </div>
      <ul class="clause-list">${clauses}</ul>
    </section>`;
}

function renderRunSummary(data) {
  return `
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Agent run</p>
          <h2>Synthetic CV screening path</h2>
        </div>
        ${pill("lab only", "warning")}
      </div>
      <div class="summary-grid">
        ${metric("Applications", data.run.applications)}
        ${metric("Triggered actions", data.run.triggeredActions)}
        ${metric("Authorised annotations", data.run.authorised, "positive")}
        ${metric("Pending reviews", data.run.pendingEscalations, "warning")}
      </div>
      <div class="action-strip">
        <button class="secondary-command" data-open-view="escalation-queue" type="button">Open human review</button>
        <button class="secondary-command" data-open-view="evidence-audit" type="button">Open evidence</button>
      </div>
    </section>`;
}

function renderPolicyBoundary(data) {
  return `
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Policy boundary</p>
          <h2>Exactly ten decision fields</h2>
        </div>
        ${pill("fixed input")}
      </div>
      <div class="two-column">
        <div>
          <h3>Allowed into policy</h3>
          <ul class="token-list">${data.policyFields.map((field) => `<li>${escapeHTML(field)}</li>`).join("")}</ul>
        </div>
        <div>
          <h3>Kept outside policy</h3>
          <ul class="token-list">${data.hiddenInputs.map((field) => `<li>${escapeHTML(field)}</li>`).join("")}</ul>
        </div>
      </div>
    </section>`;
}

function renderEscalations() {
  const target = document.querySelector("#view-escalation-queue");
  const data = state.escalations;
  if (!data) {
    target.innerHTML = loadingSurface("Human Review");
    return;
  }
  const rows = data.items
    .map(
      (item) => `<tr>
        <td>${escapeHTML(item.applicationId)}</td>
        <td>${escapeHTML(item.confidence)}</td>
        <td>${pill(item.ruleFired.id, "warning")}</td>
        <td>${pill(item.status)}</td>
        <td>${escapeHTML(item.reasonRef || item.evidenceRefs?.[0] || "")}</td>
        <td>
          <div class="button-row">
            <button class="table-command" data-decision="approve" data-action="${escapeHTML(item.actionRef)}" type="button">Approve</button>
            <button class="table-command" data-decision="decline" data-action="${escapeHTML(item.actionRef)}" type="button">Decline</button>
            <button class="table-command" data-decision="info" data-action="${escapeHTML(item.actionRef)}" type="button">Info</button>
          </div>
        </td>
      </tr>`
    )
    .join("");

  target.innerHTML = `
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Human review</p>
          <h1>Escalation Queue</h1>
        </div>
        ${pill(`${data.summary.pendingEscalations} pending`, "warning")}
      </div>
      <div class="summary-grid">
        ${metric("Applications", data.summary.applications)}
        ${metric("Triggered", data.summary.triggeredActions)}
        ${metric("Authorised", data.summary.authorised, "positive")}
        ${metric("Escalated", data.summary.escalated, "warning")}
      </div>
    </section>
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Reserved decision</p>
          <h2>Unclear work-authorisation cases</h2>
        </div>
        ${pill("reason refs only")}
      </div>
      <div class="decision-table-wrap">
        <table>
          <thead><tr><th>Application</th><th>Confidence</th><th>Rule</th><th>Status</th><th>Evidence or reason ref</th><th>Decision</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    </section>`;

  target.querySelectorAll("button.table-command").forEach((button) => {
    button.addEventListener("click", () => decideEscalation(button.dataset.action, button.dataset.decision));
  });
}

async function decideEscalation(actionRef, decision) {
  const reasonRef = `fixture://synthetic-cv-lab/reasons/${encodeURIComponent(actionRef)}-${decision}`;
  try {
    await postJSON("/api/escalations/decision", {
      actionRef,
      decision,
      decidedBy: "hiring-manager-1",
      reasonRef
    });
    state.escalations = await fetchJSON("/api/escalations");
    runtimeStatus.textContent = "Decision recorded";
  } catch (error) {
    runtimeStatus.textContent = "Decision rejected";
    console.error(error);
  }
  renderEscalations();
  renderWorkflowSteps();
}

function renderEvidence() {
  const target = document.querySelector("#view-evidence-audit");
  if (!state.audit || !state.evidence || !state.privacy || !state.latency) {
    target.innerHTML = loadingSurface("Evidence");
    return;
  }
  const verified = state.audit.verification?.OK ?? state.audit.verification?.ok;
  const criteria = state.evidence.criteria
    .map((item) => `<tr><td>${escapeHTML(item.id)}</td><td>${pill(item.verdict)}</td><td>${escapeHTML(item.latestRun || "")}</td><td>${escapeHTML((item.files || []).length)}</td></tr>`)
    .join("");
  const entries = state.audit.entries
    .slice(0, 24)
    .map((entry) => `<tr><td>${escapeHTML(entry.seq)}</td><td>${escapeHTML(entry.type)}</td><td>${escapeHTML(entry.actionRef)}</td><td>${pill(entry.decision)}</td><td>${escapeHTML(entry.rule)}</td><td>${escapeHTML(entry.inputsDigest)}</td></tr>`)
    .join("");

  target.innerHTML = `
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Evidence</p>
          <h1>TRL 4 Lab Evidence</h1>
        </div>
        ${pill(verified ? "verified" : "failed")}
      </div>
      <div class="summary-grid">
        ${metric("Audit entries", state.audit.entries.length)}
        ${metric("Decisions", state.audit.summary.decisionEntries)}
        ${metric("Effects", state.audit.summary.effectEntries)}
        ${metric("Privacy scan", state.privacy.ok ? "clear" : "finding", state.privacy.ok ? "positive" : "negative")}
        ${metric("A8 p95 us", state.latency.p95Micros)}
      </div>
    </section>
    <div class="two-column">
      <section class="surface">
        <div class="section-heading">
          <div>
            <p class="eyebrow">Acceptance criteria</p>
            <h2>A1-A10 status</h2>
          </div>
          ${pill("evidence pack")}
        </div>
        <div class="decision-table-wrap">
          <table>
            <thead><tr><th>ID</th><th>Verdict</th><th>Latest run</th><th>Files</th></tr></thead>
            <tbody>${criteria}</tbody>
          </table>
        </div>
      </section>
      <section class="surface">
        <div class="section-heading">
          <div>
            <p class="eyebrow">Performance</p>
            <h2>Policy latency</h2>
          </div>
          ${pill(`${state.latency.trials} trials`)}
        </div>
        <div class="histogram">${renderHistogram(state.latency.histogram)}</div>
      </section>
    </div>
    <section class="surface">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Audit chain</p>
          <h2>Decision and effect sample</h2>
        </div>
        ${pill("privacy safe")}
      </div>
      <div class="decision-table-wrap">
        <table>
          <thead><tr><th>Seq</th><th>Type</th><th>Action</th><th>Decision</th><th>Rule</th><th>Input digest</th></tr></thead>
          <tbody>${entries}</tbody>
        </table>
      </div>
    </section>`;
}

function renderHistogram(histogram = []) {
  const max = Math.max(1, ...histogram.map((bucket) => bucket.count));
  return histogram
    .map((bucket) => {
      const label = bucket.upperBoundMicros < 0 ? ">50000" : `<=${bucket.upperBoundMicros}`;
      const width = Math.max(2, Math.round((bucket.count / max) * 100));
      return `<div class="bar-row"><span>${escapeHTML(label)}</span><div class="bar-track"><div class="bar-fill" style="width:${width}%"></div></div><span>${escapeHTML(bucket.count)}</span></div>`;
    })
    .join("");
}

function loadingSurface(title) {
  return `<section class="surface"><div class="section-heading"><div><p class="eyebrow">Loading</p><h1>${escapeHTML(title)}</h1></div>${pill("loading")}</div>${loadingBlock("Preparing view")}</section>`;
}

function loadingBlock(text) {
  return `<div class="empty-state">${escapeHTML(text)}</div>`;
}

function renderAll() {
  renderShell();
  renderAssistant();
  renderEscalations();
  renderEvidence();
}

async function evaluatePrompt(prompt) {
  state.evaluating = true;
  runtimeStatus.textContent = "Evaluating";
  renderAssistant();
  try {
    state.assistant = await postJSON("/api/assistant/evaluate", { prompt });
    state.prompt = state.assistant.prompt;
    runtimeStatus.textContent = state.assistant.outcome === "refused" ? "Mandate refused" : "Mandate signed";
  } catch (error) {
    runtimeStatus.textContent = "Assistant unavailable";
    console.error(error);
  }
  state.evaluating = false;
  renderAssistant();
  renderWorkflowSteps();
}

async function boot() {
  renderAll();
  try {
    const [assistant, escalations, audit, evidence, privacy, latency] = await Promise.all([
      postJSON("/api/assistant/evaluate", { prompt: state.prompt }),
      fetchJSON("/api/escalations"),
      fetchJSON("/api/audit"),
      fetchJSON("/api/evidence"),
      fetchJSON("/api/privacy-scan"),
      fetchJSON("/api/latency")
    ]);
    Object.assign(state, { assistant, escalations, audit, evidence, privacy, latency });
    runtimeStatus.textContent = assistant.outcome === "refused" ? "Mandate refused" : "Ready";
  } catch (error) {
    runtimeStatus.textContent = "API unavailable";
    console.error(error);
  }
  renderAll();
}

boot();
