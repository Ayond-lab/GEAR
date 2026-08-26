package auditprivacy

import "testing"

func TestScanAllowsDigestOnlyAuditEntry(t *testing.T) {
	findings := Scan(`{"subjectRef":"sha256:abc","dataAccessed":["sha256:def"],"mandate":"MND-2026-021"}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}

func TestScanFlagsRawEmail(t *testing.T) {
	findings := Scan(`{"candidate":"Synthetic-001","email":"synthetic@example.test"}`)
	if len(findings) == 0 {
		t.Fatal("expected raw email finding")
	}
}

