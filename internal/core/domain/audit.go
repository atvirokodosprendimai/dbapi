package domain

import "time"

type AuditEvent struct {
	TenantID   string
	Collection string
	RecordID   string
	Action     string
	Actor      string
	At         time.Time
}
