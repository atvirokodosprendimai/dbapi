package domain

import "regexp"

var pathSegmentPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type JSONPathFilter struct {
	Path  string
	Op    string
	Value string
}

func (f JSONPathFilter) Validate() error {
	if f.Path == "" {
		if f.Op == "" && f.Value == "" {
			return nil
		}
		return ErrInvalidFilter
	}

	segments := SplitJSONPath(f.Path)
	if len(segments) == 0 {
		return ErrInvalidFilter
	}
	for _, seg := range segments {
		if !pathSegmentPattern.MatchString(seg) {
			return ErrInvalidFilter
		}
	}

	if f.Op == "" {
		f.Op = "eq"
	}
	switch f.Op {
	case "eq", "ne", "contains":
		if f.Value == "" {
			return ErrInvalidFilter
		}
	case "exists":
		if f.Value != "" {
			return ErrInvalidFilter
		}
	default:
		return ErrInvalidFilter
	}

	return nil
}

type RecordListFilter struct {
	Prefix string
	After  string
	Limit  int
	JSON   JSONPathFilter
}

func SplitJSONPath(path string) []string {
	segments := make([]string, 0)
	current := ""
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if current == "" {
				return nil
			}
			segments = append(segments, current)
			current = ""
			continue
		}
		current += string(path[i])
	}
	if current == "" {
		return nil
	}
	segments = append(segments, current)
	return segments
}
