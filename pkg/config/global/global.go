package global

type Level string

const (
	Warn     Level = "warn"
	Critical Level = "critical"
)

func (l Level) IsValid() bool {
	switch l {
	case Warn, Critical:
		return true
	default:
		return false
	}
}

type Global struct {
	Linters Linters `mapstructure:"linters"`
}

type Linters struct {
	Probes Probes `mapstructure:"probes"`
	Images Images `mapstructure:"images"`
}

type Probes struct {
	Impact Level `mapstructure:"impact" default:"critical"`
}

type Images struct {
	Impact Level `mapstructure:"impact" default:"critical"`
}
