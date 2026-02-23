package domain

import (
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalidKey      = errors.New("invalid key")
	ErrInvalidCategory = errors.New("invalid category")
	ErrInvalidFilter   = errors.New("invalid filter")
	ErrNotFound        = errors.New("not found")
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9._:/-]+$`)

type Item struct {
	Key       string
	Category  string
	Value     json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (i Item) Validate() error {
	if err := ValidateKey(i.Key); err != nil {
		return err
	}
	if err := ValidateCategory(i.Category); err != nil {
		return err
	}
	if !json.Valid(i.Value) {
		return errors.New("value must be valid json")
	}
	return nil
}

func ValidateKey(key string) error {
	if key == "" || !keyPattern.MatchString(key) {
		return ErrInvalidKey
	}
	return nil
}

func ValidateCategory(category string) error {
	if category == "" || !keyPattern.MatchString(category) {
		return ErrInvalidCategory
	}
	return nil
}

type ScanFilter struct {
	Category string
	Prefix   string
	AfterKey string
	Limit    int
}

func (f ScanFilter) Validate() error {
	if f.Category != "" {
		if err := ValidateCategory(f.Category); err != nil {
			return err
		}
	}
	if f.Prefix != "" && !keyPattern.MatchString(f.Prefix) {
		return ErrInvalidKey
	}
	if f.AfterKey != "" {
		if err := ValidateKey(f.AfterKey); err != nil {
			return err
		}
	}
	return nil
}
