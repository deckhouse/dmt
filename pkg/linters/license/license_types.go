package license

import (
	"github.com/deckhouse/dmt/pkg"
)

const (
	FilesRuleName = "files"
)

func NewFilesRule(excludeRules []pkg.StringRuleExclude) *FilesRule {
	return &FilesRule{
		RuleMeta: pkg.RuleMeta{
			Name: FilesRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
	}
}

type FilesRule struct {
	pkg.RuleMeta
	pkg.StringRule
}
