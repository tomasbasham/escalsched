package escalsched

import "fmt"

// Priority represents the priority of a [Task].
type Priority struct {
	priority
}

// ParsePriority creates a new [Priority] from the given value.
func ParsePriority(p any) Priority {
	switch v := p.(type) {
	case Priority:
		return v
	case string:
		return Priority{stringToPriority(v)}
	case fmt.Stringer:
		return Priority{stringToPriority(v.String())}
	case int:
		return Priority{priority(v)}
	case int64:
		return Priority{priority(int(v))}
	case int32:
		return Priority{priority(int(v))}
	default:
		return Priority{priorityUnknown}
	}
}

func (p Priority) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.String() + `"`), nil
}

func (p *Priority) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	*p = ParsePriority(s)
	return nil
}

// Priorities is a more typical enum like structure from other languages, ported
// to Go. It may be used to reference a [Priority] value by name.
var Priorities = priorityContainer{
	Unknown: Priority{priorityUnknown},
	VeryLow: Priority{priorityVeryLow},
	Low:     Priority{priorityLow},
	Normal:  Priority{priorityNormal},
	High:    Priority{priorityHigh},
}

// All returns all possible priorities.
func (c priorityContainer) All() []Priority {
	return []Priority{c.Unknown, c.VeryLow, c.Low, c.Normal, c.High}
}

type priority int

const (
	priorityUnknown priority = 0
	priorityVeryLow priority = 10
	priorityLow     priority = 20
	priorityNormal  priority = 30
	priorityHigh    priority = 40
)

var (
	strPriorityMap = map[priority]string{
		priorityUnknown: "unknown",
		priorityVeryLow: "very-low",
		priorityLow:     "low",
		priorityNormal:  "normal",
		priorityHigh:    "high",
	}

	typePriorityMap = map[string]priority{
		"unknown":  priorityUnknown,
		"very-low": priorityVeryLow,
		"low":      priorityLow,
		"normal":   priorityNormal,
		"high":     priorityHigh,
	}
)

func (p priority) String() string {
	return strPriorityMap[p]
}

func (p priority) IsValid() bool {
	_, ok := strPriorityMap[p]
	return ok
}

func stringToPriority(s string) priority {
	if v, ok := typePriorityMap[s]; ok {
		return v
	}
	return priorityUnknown
}

type priorityContainer struct {
	Unknown Priority
	VeryLow Priority
	Low     Priority
	Normal  Priority
	High    Priority
}
