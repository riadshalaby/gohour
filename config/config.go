package config

import (
	"bytes"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

const (
	KeyUser                     = "user"
	KeyURL                      = "url"
	KeyPort                     = "port"
	KeyAutoReconcileAfterImport = "auto_reconcile_after_import"
	KeyEPMRules                 = "epm.rules"
)

type Config struct {
	User                     string    `mapstructure:"user" validate:"required"`
	URL                      string    `mapstructure:"url" validate:"required,url"`
	Port                     int       `mapstructure:"port" validate:"required,min=1,max=65535"`
	AutoReconcileAfterImport bool      `mapstructure:"auto_reconcile_after_import"`
	EPM                      EPMConfig `mapstructure:"epm"`

	// Runtime-only values resolved per imported file (not loaded from config).
	ImportProject  string `mapstructure:"-"`
	ImportActivity string `mapstructure:"-"`
	ImportSkill    string `mapstructure:"-"`
}

type EPMConfig struct {
	Rules []EPMRule `mapstructure:"rules"`
}

type EPMRule struct {
	Name         string `mapstructure:"name"`
	FileTemplate string `mapstructure:"file_template"`
	Project      string `mapstructure:"project"`
	Activity     string `mapstructure:"activity"`
	Skill        string `mapstructure:"skill"`
}

// SetDefaults sets default values if not provided
func SetDefaults() {
	viper.SetDefault(KeyUser, "default_user")
	viper.SetDefault(KeyURL, "http://localhost")
	viper.SetDefault(KeyPort, 8080)
	viper.SetDefault(KeyAutoReconcileAfterImport, true)
	viper.SetDefault(KeyEPMRules, []map[string]string{})
}

// LoadAndValidate loads config from Viper and validates it
func LoadAndValidate() (*Config, error) {
	return loadAndValidateFromViper(viper.GetViper())
}

// ValidateYAMLContent validates configuration from raw YAML content.
func ValidateYAMLContent(content []byte) (*Config, error) {
	local := viper.New()
	setDefaults(local)
	local.SetConfigType("yaml")
	if err := local.ReadConfig(bytes.NewReader(content)); err != nil {
		return nil, fmt.Errorf("read config content: %w", err)
	}
	return loadAndValidateFromViper(local)
}

// ExampleYAML returns an example configuration template.
func ExampleYAML() string {
	return `# gohour configuration
user: "your.user"
url: "https://company.example"
port: 443
auto_reconcile_after_import: true

epm:
  rules:
    - name: "rz"
      file_template: "EPMExportRZ*.xlsx"
      project: "MySpecial RZ Project"
      activity: "Delivery"
      skill: "Go"
`
}

func loadAndValidateFromViper(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault(KeyUser, "default_user")
	v.SetDefault(KeyURL, "http://localhost")
	v.SetDefault(KeyPort, 8080)
	v.SetDefault(KeyAutoReconcileAfterImport, true)
	v.SetDefault(KeyEPMRules, []map[string]string{})
}
