package config

import (
	"bytes"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	KeyOnePointURL = "onepoint.url"
	KeyRules       = "rules"
)

type Config struct {
	OnePoint OnePointConfig `mapstructure:"onepoint" yaml:"onepoint" validate:"required"`
	Rules    []Rule         `mapstructure:"rules" yaml:"rules"`

	// Runtime-only values resolved per imported file (not loaded from config).
	ImportProject  string `mapstructure:"-"`
	ImportActivity string `mapstructure:"-"`
	ImportSkill    string `mapstructure:"-"`
	ImportBillable bool   `mapstructure:"-"`
}

type OnePointConfig struct {
	URL string `mapstructure:"url" yaml:"url" validate:"required,url"`
}

type Rule struct {
	Name         string `mapstructure:"name" yaml:"name"`
	Mapper       string `mapstructure:"mapper" yaml:"mapper"`
	FileTemplate string `mapstructure:"file_template" yaml:"file_template"`
	Billable     *bool  `mapstructure:"billable" yaml:"billable,omitempty"`
	ProjectID    int64  `mapstructure:"project_id" yaml:"project_id"`
	Project      string `mapstructure:"project" yaml:"project"`
	ActivityID   int64  `mapstructure:"activity_id" yaml:"activity_id"`
	Activity     string `mapstructure:"activity" yaml:"activity"`
	SkillID      int64  `mapstructure:"skill_id" yaml:"skill_id"`
	Skill        string `mapstructure:"skill" yaml:"skill"`
}

// IsBillable returns whether entries from this rule should be billable.
// Defaults to true when the field is not set.
func (r Rule) IsBillable() bool {
	if r.Billable == nil {
		return true
	}
	return *r.Billable
}

// SetDefaults sets default values if not provided
func SetDefaults() {
	viper.SetDefault(KeyOnePointURL, "https://onepoint.virtual7.io/onepoint/faces/home")
	viper.SetDefault(KeyRules, []map[string]any{})
}

// DataDir returns the directory where gohour stores config, database, and auth state files.
func DataDir() string {
	if override := strings.TrimSpace(os.Getenv("GOHOUR_DATA_DIR")); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".gohour")
	}
	return filepath.Join(home, ".gohour")
}

func ConfigPath() string {
	return filepath.Join(DataDir(), "config.yaml")
}

func DBPath() string {
	return filepath.Join(DataDir(), "gohour.db")
}

func AuthStatePath() string {
	return filepath.Join(DataDir(), "onepoint-auth-state.json")
}

// WriteConfig writes the current configuration to the fixed gohour config path.
func WriteConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if err := os.MkdirAll(DataDir(), 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	content, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(ConfigPath(), content, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
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
	v.SetDefault(KeyRules, []map[string]any{})
}

func validateRules(rules []Rule) error {
	validMappers := map[string]bool{
		"epm":     true,
		"generic": true,
		"atwork":  true,
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
				"validation failed: rules[%d].mapper %q is not supported (valid: epm, generic, atwork)",
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
