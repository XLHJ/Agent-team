package main

import "context"

func t10(_ context.Context) {
	code, m := postJSON("/selftest/approvals/respond", map[string]any{"approval_id": "ap-1", "decision": "deny"})
	if code == 409 && m["error"] == "APPROVAL_ALREADY_RESOLVED" {
		pass("T10 conflict rejected")
	} else {
		fail("T10 conflict rejected", code, m["error"])
	}
}
