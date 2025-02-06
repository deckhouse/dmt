package pkg

// var _ fmt.Stringer = (*Level)(nil)

type Level int

const (
	Warn Level = iota
	Critical
)

func ParseStringToLevel(str string) Level {
	lvl, ok := getStringLevelMappings()[str]
	if !ok {
		return Critical
	}

	return lvl
}

func getLevelStringMappings() map[Level]string {
	return map[Level]string{
		Warn:     "warn",
		Critical: "critical",
	}
}

func getStringLevelMappings() map[string]Level {
	return map[string]Level{
		"warn":     Warn,
		"critical": Critical,
	}
}
