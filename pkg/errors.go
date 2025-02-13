package pkg

type LinterError struct {
	LinterID    string
	ModuleID    string
	RuleID      string
	ObjectID    string
	ObjectValue any
	Text        string
	FilePath    string
	LineNumber  int
	Level       Level
}
