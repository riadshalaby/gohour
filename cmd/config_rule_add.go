package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gohour/config"
	"gohour/importer"
	"gohour/onepoint"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	configRuleAddAuthStateFile  string
	configRuleAddURL            string
	configRuleAddTimeout        time.Duration
	configRuleAddIncludeArchive bool
	configRuleAddIncludeLocked  bool
)

var configRuleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Interactively add one import rule from OnePoint lookups.",
	Long: `Fetch projects, activities, and skills from OnePoint for the logged-in user,
let you choose each entry interactively, then store a new rules entry in config.`,
	Example: `
  # Add one rule interactively using onepoint.url from config and default auth state file
  gohour config rule add

  # Override OnePoint URL for this run
  gohour config rule add --url https://onepoint.virtual7.io/onepoint/faces/home

  # Use custom auth state file
  gohour config rule add --state-file ./artifacts/playwright/onepoint-auth-state.json
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := resolveConfigEditPath(cfgFile, viper.ConfigFileUsed())
		if err != nil {
			return err
		}

		_, err = ensureConfigFileWithTemplate(configPath)
		if err != nil {
			return err
		}

		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("read config %q: %w", configPath, err)
		}

		baseURL, homeURL, host, err := resolveOnePointURLs(configRuleAddURL)
		if err != nil {
			return err
		}

		stateFile, err := resolveDefaultAuthStatePath(configRuleAddAuthStateFile)
		if err != nil {
			return err
		}
		cookieHeader, err := onepoint.SessionCookieHeaderFromStateFile(stateFile, host)
		if err != nil {
			return fmt.Errorf("read auth state %q: %w", stateFile, err)
		}

		client, err := onepoint.NewClient(onepoint.ClientConfig{
			BaseURL:        baseURL,
			RefererURL:     homeURL,
			SessionCookies: cookieHeader,
			UserAgent:      "gohour-config-rule/1.0",
		})
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), configRuleAddTimeout)
		defer cancel()

		snapshot, err := client.FetchLookupSnapshot(ctx)
		if err != nil {
			return fmt.Errorf("fetch OnePoint lookup values: %w", err)
		}

		reader := bufio.NewReader(os.Stdin)
		mapperNames := importer.SupportedMapperNames()
		if len(mapperNames) == 0 {
			return fmt.Errorf("no mappers are available")
		}
		selectedMapperIdx, err := promptSelectIndex(
			reader,
			os.Stdout,
			"Select mapper:",
			mapperNames,
		)
		if err != nil {
			return err
		}
		selectedMapper := mapperNames[selectedMapperIdx]

		projects := filterProjects(snapshot.Projects, configRuleAddIncludeArchive)
		if len(projects) == 0 {
			return fmt.Errorf("no selectable projects found")
		}
		sort.Slice(projects, func(i, j int) bool {
			leftArchived := projects[i].IsArchived()
			rightArchived := projects[j].IsArchived()
			if leftArchived != rightArchived {
				return !leftArchived
			}
			left := strings.ToLower(strings.TrimSpace(projects[i].Name))
			right := strings.ToLower(strings.TrimSpace(projects[j].Name))
			if left == right {
				return projects[i].ID < projects[j].ID
			}
			return left < right
		})

		selectedProjectIdx, err := promptSelectIndex(
			reader,
			os.Stdout,
			"Select project:",
			projectOptionLines(projects),
		)
		if err != nil {
			return err
		}
		selectedProject := projects[selectedProjectIdx]

		activities := filterActivities(snapshot.Activities, selectedProject.ID, configRuleAddIncludeLocked)
		if len(activities) == 0 {
			return fmt.Errorf("no selectable activities found for project %q", selectedProject.Name)
		}
		sort.Slice(activities, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(activities[i].Name))
			right := strings.ToLower(strings.TrimSpace(activities[j].Name))
			if left == right {
				return activities[i].ID < activities[j].ID
			}
			return left < right
		})

		selectedActivityIdx, err := promptSelectIndex(
			reader,
			os.Stdout,
			fmt.Sprintf("Select activity for project %q:", selectedProject.Name),
			activityOptionLines(activities),
		)
		if err != nil {
			return err
		}
		selectedActivity := activities[selectedActivityIdx]

		skills := filterSkills(snapshot.Skills, selectedActivity.ID)
		if len(skills) == 0 {
			return fmt.Errorf("no selectable skills found for activity %q", selectedActivity.Name)
		}
		sort.Slice(skills, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(skills[i].Name))
			right := strings.ToLower(strings.TrimSpace(skills[j].Name))
			if left == right {
				return skills[i].SkillID < skills[j].SkillID
			}
			return left < right
		})

		selectedSkillIdx, err := promptSelectIndex(
			reader,
			os.Stdout,
			fmt.Sprintf("Select skill for activity %q:", selectedActivity.Name),
			skillOptionLines(skills),
		)
		if err != nil {
			return err
		}
		selectedSkill := skills[selectedSkillIdx]

		ruleName, err := promptRequiredString(reader, os.Stdout, "Rule name")
		if err != nil {
			return err
		}
		fileTemplate, err := promptRequiredString(reader, os.Stdout, "File template (example: EPMExportRZ*.xlsx)")
		if err != nil {
			return err
		}

		newRule := config.Rule{
			Name:         ruleName,
			Mapper:       strings.ToLower(strings.TrimSpace(selectedMapper)),
			FileTemplate: fileTemplate,
			ProjectID:    selectedProject.ID,
			Project:      selectedProject.Name,
			ActivityID:   selectedActivity.ID,
			Activity:     selectedActivity.Name,
			SkillID:      selectedSkill.SkillID,
			Skill:        selectedSkill.Name,
		}

		current, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read config file: %w", err)
		}

		updated, err := appendRuleToConfigYAML(current, newRule)
		if err != nil {
			return err
		}

		if err := os.WriteFile(configPath, updated, 0o600); err != nil {
			return fmt.Errorf("write config file: %w", err)
		}

		fmt.Println("Rule added successfully.")
		fmt.Printf("Config:   %s\n", configPath)
		fmt.Printf("Name:     %s\n", newRule.Name)
		fmt.Printf("Mapper:   %s\n", newRule.Mapper)
		fmt.Printf("Template: %s\n", newRule.FileTemplate)
		fmt.Printf("Project:  %s (id=%d)\n", newRule.Project, newRule.ProjectID)
		fmt.Printf("Activity: %s (id=%d)\n", newRule.Activity, newRule.ActivityID)
		fmt.Printf("Skill:    %s (id=%d)\n", newRule.Skill, newRule.SkillID)
		return nil
	},
}

func filterProjects(projects []onepoint.Project, includeArchived bool) []onepoint.Project {
	if includeArchived {
		return append([]onepoint.Project(nil), projects...)
	}
	out := make([]onepoint.Project, 0, len(projects))
	for _, project := range projects {
		if project.IsArchived() {
			continue
		}
		out = append(out, project)
	}
	return out
}

func filterActivities(activities []onepoint.Activity, projectID int64, includeLocked bool) []onepoint.Activity {
	out := make([]onepoint.Activity, 0, len(activities))
	for _, activity := range activities {
		if activity.ProjectNodeID != projectID {
			continue
		}
		if activity.Locked && !includeLocked {
			continue
		}
		out = append(out, activity)
	}
	return out
}

func filterSkills(skills []onepoint.Skill, activityID int64) []onepoint.Skill {
	out := make([]onepoint.Skill, 0, len(skills))
	seen := make(map[int64]struct{})
	for _, skill := range skills {
		if skill.ActivityID != activityID {
			continue
		}
		if _, exists := seen[skill.SkillID]; exists {
			continue
		}
		seen[skill.SkillID] = struct{}{}
		out = append(out, skill)
	}
	return out
}

func projectOptionLines(projects []onepoint.Project) []string {
	lines := make([]string, 0, len(projects))
	for _, project := range projects {
		suffix := ""
		if project.IsArchived() {
			suffix = " [archived]"
		}
		lines = append(lines, fmt.Sprintf("%s (id=%d)%s", project.Name, project.ID, suffix))
	}
	return lines
}

func activityOptionLines(activities []onepoint.Activity) []string {
	lines := make([]string, 0, len(activities))
	for _, activity := range activities {
		suffix := ""
		if activity.Locked {
			suffix = " [locked]"
		}
		lines = append(lines, fmt.Sprintf("%s (id=%d)%s", activity.Name, activity.ID, suffix))
	}
	return lines
}

func skillOptionLines(skills []onepoint.Skill) []string {
	lines := make([]string, 0, len(skills))
	for _, skill := range skills {
		lines = append(lines, fmt.Sprintf("%s (id=%d)", skill.Name, skill.SkillID))
	}
	return lines
}

func promptSelectIndex(reader *bufio.Reader, out io.Writer, title string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, fmt.Errorf("no options available for %q", title)
	}

	for {
		fmt.Fprintln(out, title)
		for i, option := range options {
			fmt.Fprintf(out, "  %d) %s\n", i+1, option)
		}
		fmt.Fprintf(out, "Choose [1-%d]: ", len(options))

		input, err := reader.ReadString('\n')
		if err != nil {
			return -1, fmt.Errorf("read selection input: %w", err)
		}
		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Fprintln(out, "Invalid selection. Please enter a valid number.")
			continue
		}
		return choice - 1, nil
	}
}

func promptRequiredString(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		fmt.Fprintf(out, "%s: ", strings.TrimSpace(label))
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read %s: %w", strings.TrimSpace(strings.ToLower(label)), err)
		}
		value := strings.TrimSpace(input)
		if value == "" {
			fmt.Fprintln(out, "Value must not be empty.")
			continue
		}
		return value, nil
	}
}

func appendRuleToConfigYAML(content []byte, rule config.Rule) ([]byte, error) {
	if strings.TrimSpace(rule.Name) == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	if strings.TrimSpace(rule.Mapper) == "" {
		return nil, fmt.Errorf("mapper is required")
	}
	if strings.TrimSpace(rule.FileTemplate) == "" {
		return nil, fmt.Errorf("file template is required")
	}
	if strings.TrimSpace(rule.Project) == "" || strings.TrimSpace(rule.Activity) == "" || strings.TrimSpace(rule.Skill) == "" {
		return nil, fmt.Errorf("project, activity and skill are required")
	}
	if rule.ProjectID <= 0 || rule.ActivityID <= 0 || rule.SkillID <= 0 {
		return nil, fmt.Errorf("project_id, activity_id and skill_id must be > 0")
	}

	doc := map[string]any{}
	if strings.TrimSpace(string(content)) != "" {
		if err := yaml.Unmarshal(content, &doc); err != nil {
			return nil, fmt.Errorf("parse config yaml: %w", err)
		}
	}

	rulesList, err := ensureSliceAny(doc, "rules")
	if err != nil {
		return nil, err
	}

	for _, existing := range rulesList {
		ruleMap, ok := existing.(map[string]any)
		if !ok {
			continue
		}
		existingName, _ := ruleMap["name"].(string)
		if strings.EqualFold(strings.TrimSpace(existingName), strings.TrimSpace(rule.Name)) {
			return nil, fmt.Errorf("rule with name %q already exists", rule.Name)
		}
	}

	rulesList = append(rulesList, map[string]any{
		"name":          rule.Name,
		"mapper":        rule.Mapper,
		"file_template": rule.FileTemplate,
		"project_id":    rule.ProjectID,
		"project":       rule.Project,
		"activity_id":   rule.ActivityID,
		"activity":      rule.Activity,
		"skill_id":      rule.SkillID,
		"skill":         rule.Skill,
	})
	doc["rules"] = rulesList

	updated, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal updated config yaml: %w", err)
	}
	if _, err := config.ValidateYAMLContent(updated); err != nil {
		return nil, fmt.Errorf("updated config is invalid: %w", err)
	}
	return updated, nil
}

func ensureSliceAny(doc map[string]any, key string) ([]any, error) {
	raw, exists := doc[key]
	if !exists || raw == nil {
		result := []any{}
		doc[key] = result
		return result, nil
	}
	result, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("config key %q must be a list", key)
	}
	return result, nil
}

func init() {
	configRuleCmd.AddCommand(configRuleAddCmd)

	configRuleAddCmd.Flags().StringVar(&configRuleAddURL, "url", "", "Override OnePoint URL from config (full home URL)")
	configRuleAddCmd.Flags().StringVar(&configRuleAddAuthStateFile, "state-file", "", "Path to auth state JSON (default: $HOME/.gohour/onepoint-auth-state.json)")
	configRuleAddCmd.Flags().DurationVar(&configRuleAddTimeout, "timeout", 60*time.Second, "Timeout for OnePoint lookup API calls")
	configRuleAddCmd.Flags().BoolVar(&configRuleAddIncludeArchive, "include-archived-projects", false, "Include archived projects in project selection")
	configRuleAddCmd.Flags().BoolVar(&configRuleAddIncludeLocked, "include-locked-activities", false, "Include locked activities in activity selection")
}
