package middleware

import (
	"testing"
)

func TestResetAuditBuffer_DoesNotPanic(t *testing.T) {
	// ResetAuditBuffer should be safe to call even before StartAuditWorker
	// (resetChan may be nil)
	ResetAuditBuffer()
}
