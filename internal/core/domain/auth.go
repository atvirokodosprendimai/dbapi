package domain

import "time"

type APIKey struct {
	TokenHash string
	TenantID  string
	Name      string
	Active    bool
	CreatedAt time.Time
}
