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

func getStringLevelMappings() map[string]Level {
	return map[string]Level{
		"warn":     Warn,
		"critical": Critical,
	}
}
