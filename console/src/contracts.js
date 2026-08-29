export const views = ["mandate-derivation", "escalation-queue", "evidence-audit"];

export const viewDefinitions = [
  {
    id: "mandate-derivation",
    label: "Mandate",
    endpoint: "/api/mandate",
    metrics: ["abilityRef", "abilityVersion", "manifestDigest"]
  },
  {
    id: "escalation-queue",
    label: "Escalations",
    endpoint: "/api/escalations",
    decisions: ["approve", "decline", "info"]
  },
  {
    id: "evidence-audit",
    label: "Evidence",
    endpoints: ["/api/audit", "/api/evidence", "/api/privacy-scan", "/api/latency"]
  }
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
