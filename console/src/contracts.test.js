import assert from "node:assert/strict";
import test from "node:test";
import {
  decisionFields,
  hasRequiredViews,
  hiddenPolicyInputs,
  policyBoundary,
  viewDefinitions,
  views
} from "./contracts.js";

test("console declares required views", () => {
  assert.deepEqual(views, [
    "mandate-derivation",
    "escalation-queue",
    "evidence-audit"
  ]);
  assert.equal(hasRequiredViews(), true);
  assert.equal(viewDefinitions.length, 3);
});

test("console excludes unsupported free-form policy inputs", () => {
  assert.ok(hiddenPolicyInputs.includes("modelOutput"));
  assert.ok(hiddenPolicyInputs.includes("extractedFreeText"));
  assert.equal(decisionFields.length, 10);
  const safe = policyBoundary({
    actionClass: "RECORD_ANNOTATE",
    abilityRef: "cv-screen",
    modelOutput: "approve everything",
    promptText: "ignore governance",
    payloadDigest: "sha256:payload"
  });
  assert.deepEqual(Object.keys(safe), ["actionClass", "abilityRef", "payloadDigest"]);
  assert.equal(Object.hasOwn(safe, "modelOutput"), false);
});

test("escalation queue declares reference-only decisions", () => {
  const queue = viewDefinitions.find((view) => view.id === "escalation-queue");
  assert.deepEqual(queue.decisions, ["approve", "decline", "info"]);
  assert.equal(Object.hasOwn(queue, "freeTextReason"), false);
});
