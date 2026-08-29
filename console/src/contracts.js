export const views = ["mandate-derivation", "escalation-queue", "evidence-audit"];

export const viewDefinitions = [
  {
    id: "mandate-derivation",
    label: "Mandate Assistant",
    endpoint: "/api/assistant/evaluate",
    method: "POST",
    workflow: ["request", "interpret", "refuse-or-sign", "execute"]
  },
  {
    id: "escalation-queue",
    label: "Human Review",
    endpoint: "/api/escalations",
    decisions: ["approve", "decline", "info"]
  },
  {
    id: "evidence-audit",
    label: "Evidence",
    endpoints: ["/api/audit", "/api/evidence", "/api/privacy-scan", "/api/latency"]
  }
];

export const defaultHRPrompt = "Rank the candidates who are citizens of EEA.";

export const lawfulAlternativePrompt =
  "Record work-authorisation status for human planning without ranking, filtering, or excluding candidates.";

export const workflowStages = [
  { id: "request", label: "HR request" },
  { id: "interpret", label: "Interpret mandate" },
  { id: "refuse-or-sign", label: "Refuse or sign" },
  { id: "execute", label: "Run governed path" },
  { id: "review", label: "Human review" },
  { id: "evidence", label: "Evidence" }
];

export const hiddenPolicyInputs = [
  "modelOutput",
  "promptText",
  "extractedFreeText",
  "abilityNarrative"
];

export const decisionFields = [
  "actionClass",
  "abilityRef",
  "abilityVersion",
  "mandateRef",
  "mandateVersion",
  "confidence",
  "dataClasses",
  "reversibility",
  "counters",
  "payloadDigest"
];

export function policyBoundary(input) {
  return Object.fromEntries(
    decisionFields
      .filter((field) => Object.prototype.hasOwnProperty.call(input, field))
      .map((field) => [field, input[field]])
  );
}

export function hasRequiredViews(definitions = viewDefinitions) {
  return views.every((view) => definitions.some((definition) => definition.id === view));
}
