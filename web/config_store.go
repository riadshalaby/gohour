package web

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/importer"
)

type configStore struct {
	mu  sync.RWMutex
	cfg config.Config
}

func newConfigStore(cfg config.Config) *configStore {
	return &configStore{cfg: cloneConfig(cfg)}
}

func (c *configStore) Snapshot() config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneConfig(c.cfg)
}

func (c *configStore) Update(mutator func(*config.Config) error) (config.Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := cloneConfig(c.cfg)
	if err := mutator(&next); err != nil {
		return config.Config{}, err
	}
	if err := config.WriteConfig(&next); err != nil {
		return config.Config{}, fmt.Errorf("persist config: %w", err)
	}
	c.cfg = next
	return cloneConfig(c.cfg), nil
}

func cloneConfig(cfg config.Config) config.Config {
	cfg.Rules = append([]config.Rule(nil), cfg.Rules...)
	return cfg
}

func configResponseFromConfig(cfg config.Config) configAPIResponse {
	return configAPIResponse{
		OnePointURL: cfg.OnePoint.URL,
		Rules:       rulePayloadsFromRules(cfg.Rules),
	}
}

func rulePayloadsFromRules(rules []config.Rule) []rulePayload {
	out := make([]rulePayload, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rulePayloadFromRule(rule))
	}
	return out
}

func rulePayloadFromRule(rule config.Rule) rulePayload {
	return rulePayload{
		Name:         rule.Name,
		Mapper:       rule.Mapper,
		FileTemplate: rule.FileTemplate,
		Billable:     rule.Billable,
		ProjectID:    rule.ProjectID,
		Project:      rule.Project,
		ActivityID:   rule.ActivityID,
		Activity:     rule.Activity,
		SkillID:      rule.SkillID,
		Skill:        rule.Skill,
	}
}

func ruleFromPayload(payload rulePayload, fallbackName string, requireName bool) (config.Rule, error) {
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}
	if name == "" && requireName {
		return config.Rule{}, fmt.Errorf("rule name is required")
	}
	mapper := strings.ToLower(strings.TrimSpace(payload.Mapper))
	if _, err := importer.MapperByName(mapper); err != nil {
		return config.Rule{}, err
	}
	fileTemplate := strings.TrimSpace(payload.FileTemplate)
	if fileTemplate == "" {
		return config.Rule{}, fmt.Errorf("fileTemplate is required")
	}
	project := strings.TrimSpace(payload.Project)
	activity := strings.TrimSpace(payload.Activity)
	skill := strings.TrimSpace(payload.Skill)
	if project == "" || activity == "" || skill == "" {
		return config.Rule{}, fmt.Errorf("project, activity, and skill are required")
	}
	if payload.ProjectID <= 0 || payload.ActivityID <= 0 || payload.SkillID <= 0 {
		return config.Rule{}, fmt.Errorf("projectId, activityId, and skillId must be > 0")
	}
	return config.Rule{
		Name:         name,
		Mapper:       mapper,
		FileTemplate: fileTemplate,
		Billable:     payload.Billable,
		ProjectID:    payload.ProjectID,
		Project:      project,
		ActivityID:   payload.ActivityID,
		Activity:     activity,
		SkillID:      payload.SkillID,
		Skill:        skill,
	}, nil
}

func findRuleIndex(rules []config.Rule, name string) int {
	for i, rule := range rules {
		if sameRuleName(rule.Name, name) {
			return i
		}
	}
	return -1
}

func sameRuleName(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func validateOnePointURL(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("onepointUrl must be a valid absolute URL")
	}
	return nil
}
