import assert from "node:assert/strict";
import test from "node:test";
import { hiddenPolicyInputs, views } from "./contracts.js";

test("console declares required views", () => {
  assert.deepEqual(views, [
    "mandate-derivation",
    "escalation-queue",
    "evidence-audit"
  ]);
});

test("console excludes unsupported free-form policy inputs", () => {
  assert.ok(hiddenPolicyInputs.includes("modelOutput"));
  assert.ok(hiddenPolicyInputs.includes("extractedFreeText"));
});

