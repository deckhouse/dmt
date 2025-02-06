package pkg

import "fmt"

var _ fmt.Stringer = (*Level)(nil)

type Level string

const (
	Warn     Level = "warn"
	Critical Level = "critical"
)

func (l Level) String() string {
	return string(l)
}

func (l Level) IsValid() bool {
	switch l {
	case Warn, Critical:
		return true
	default:
		return false
	}
}
