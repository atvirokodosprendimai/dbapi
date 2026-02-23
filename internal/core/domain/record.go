package domain

import (
	"encoding/json"
	"errors"
	"time"
)

type Record struct {
	TenantID   string
	Collection string
	ID         string
	Data       json.RawMessage
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (r Record) Validate() error {
	if err := ValidateKey(r.TenantID); err != nil {
		return err
	}
	if err := ValidateCategory(r.Collection); err != nil {
		return err
	}
	if err := ValidateKey(r.ID); err != nil {
		return err
	}
	if !json.Valid(r.Data) {
		return errors.New("data must be valid json")
	}
	return nil
}
