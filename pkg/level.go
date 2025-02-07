package pkg

type Level int

const (
	Init Level = iota
	Warn
	Error
	Critical
)

func ParseStringToLevel(str string) Level {
	lvl, ok := getStringLevelMappings()[str]
	if !ok {
		return Error
	}

	return lvl
}

func getStringLevelMappings() map[string]Level {
	return map[string]Level{
		"warn":     Warn,
		"error":    Error,
		"critical": Critical,
	}
}
