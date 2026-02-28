package config

import (
	"bytes"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"strings"
)

const (
	KeyOnePointURL              = "onepoint.url"
	KeyImportAutoReconcileAfter = "import.auto_reconcile_after_import"
	KeyRules                    = "rules"
)

type Config struct {
	OnePoint OnePointConfig `mapstructure:"onepoint" validate:"required"`
	Import   ImportConfig   `mapstructure:"import"`
	Rules    []Rule         `mapstructure:"rules"`

	// Runtime-only values resolved per imported file (not loaded from config).
	ImportProject  string `mapstructure:"-"`
	ImportActivity string `mapstructure:"-"`
	ImportSkill    string `mapstructure:"-"`
}

type OnePointConfig struct {
	URL string `mapstructure:"url" validate:"required,url"`
}

type ImportConfig struct {
	AutoReconcileAfterImport bool `mapstructure:"auto_reconcile_after_import"`
}

type Rule struct {
	Name         string `mapstructure:"name"`
	Mapper       string `mapstructure:"mapper"`
	FileTemplate string `mapstructure:"file_template"`
	ProjectID    int64  `mapstructure:"project_id"`
	Project      string `mapstructure:"project"`
	ActivityID   int64  `mapstructure:"activity_id"`
	Activity     string `mapstructure:"activity"`
	SkillID      int64  `mapstructure:"skill_id"`
	Skill        string `mapstructure:"skill"`
}

// SetDefaults sets default values if not provided
func SetDefaults() {
	viper.SetDefault(KeyOnePointURL, "https://onepoint.virtual7.io/onepoint/faces/home")
	viper.SetDefault(KeyImportAutoReconcileAfter, true)
	viper.SetDefault(KeyRules, []map[string]any{})
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

// ExampleYAML returns the default configuration template.
func ExampleYAML() string {
	return `# gohour configuration
onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"

import:
  auto_reconcile_after_import: true

rules: []
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
	if err := validateRules(cfg.Rules); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault(KeyOnePointURL, "https://onepoint.virtual7.io/onepoint/faces/home")
	v.SetDefault(KeyImportAutoReconcileAfter, true)
	v.SetDefault(KeyRules, []map[string]any{})
}

func validateRules(rules []Rule) error {
	validMappers := map[string]bool{
		"epm":     true,
		"generic": true,
	}
	seen := make(map[string]struct{}, len(rules))
	for i, rule := range rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return fmt.Errorf("validation failed: rules[%d].name is required", i)
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("validation failed: duplicate rule name %q", name)
		}
		seen[key] = struct{}{}
		mapper := strings.ToLower(strings.TrimSpace(rule.Mapper))
		if mapper == "" {
			return fmt.Errorf("validation failed: rules[%d].mapper is required", i)
		}
		if !validMappers[mapper] {
			return fmt.Errorf(
				"validation failed: rules[%d].mapper %q is not supported (valid: epm, generic)",
				i,
				rule.Mapper,
			)
		}
		if strings.TrimSpace(rule.FileTemplate) == "" {
			return fmt.Errorf("validation failed: rules[%d].file_template is required", i)
		}
		if strings.TrimSpace(rule.Project) == "" || strings.TrimSpace(rule.Activity) == "" || strings.TrimSpace(rule.Skill) == "" {
			return fmt.Errorf("validation failed: rules[%d] requires project/activity/skill names", i)
		}
		if rule.ProjectID <= 0 || rule.ActivityID <= 0 || rule.SkillID <= 0 {
			return fmt.Errorf("validation failed: rules[%d] requires project_id/activity_id/skill_id > 0", i)
		}
	}
	return nil
}
