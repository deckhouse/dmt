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
	OpenAPI      LinterConfig `mapstructure:"openapi"`
	NoCyrillic   LinterConfig `mapstructure:"nocyrillic"`
	License      LinterConfig `mapstructure:"license"`
	OSS          LinterConfig `mapstructure:"oss"`
	Probes       LinterConfig `mapstructure:"probes"`
	Container    LinterConfig `mapstructure:"container"`
	K8SResources LinterConfig `mapstructure:"k8s_resources"`
	VPAResources LinterConfig `mapstructure:"vpa_resources"`
	PDBResources LinterConfig `mapstructure:"pdb_resources"`
	CRDResources LinterConfig `mapstructure:"crd_resources"`
	Images       LinterConfig `mapstructure:"images"`
	Rbac         LinterConfig `mapstructure:"rbac"`
	Resources    LinterConfig `mapstructure:"resources"`
	Monitoring   LinterConfig `mapstructure:"monitoring"`
	Ingress      LinterConfig `mapstructure:"ingress"`
	Module       LinterConfig `mapstructure:"module"`
	Conversions  LinterConfig `mapstructure:"conversions"`
}

type LinterConfig struct {
	Impact Level `mapstructure:"impact" default:"critical"`
}
