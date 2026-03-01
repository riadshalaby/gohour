package cmd

import (
	"fmt"
	"gohour/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the active config in an editor.",
	Long: `Open the active gohour config file in your editor.

Editor selection order:
1) $VISUAL
2) $EDITOR
3) vi

If no config file exists yet, this command creates one with an example template first.
After the editor exits, the content is validated as gohour YAML config.`,
	Example: `
  # Edit active config
  gohour config edit
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := resolveConfigEditPath(cfgFile, viper.ConfigFileUsed())
		if err != nil {
			return err
		}

		created, err := ensureConfigFileWithTemplate(configPath)
		if err != nil {
			return err
		}
		if created {
			fmt.Printf("No config file found. Created example config at: %s\n", configPath)
		}

		editor := resolveEditorValue(os.Getenv("VISUAL"), os.Getenv("EDITOR"))
		editorCommand, err := buildEditorCommand(editor, configPath)
		if err != nil {
			return err
		}
		editorCommand.Stdin = os.Stdin
		editorCommand.Stdout = os.Stdout
		editorCommand.Stderr = os.Stderr
		if err := editorCommand.Run(); err != nil {
			return fmt.Errorf("opening editor failed: %w", err)
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("reading edited config failed: %w", err)
		}
		if _, err := config.ValidateYAMLContent(content); err != nil {
			return fmt.Errorf("config validation failed in %s: %w", configPath, err)
		}

		fmt.Printf("Configuration saved and validated: %s\n", configPath)
		return nil
	},
}

func resolveConfigEditPath(configFileFlag, configFileUsed string) (string, error) {
	if strings.TrimSpace(configFileFlag) != "" {
		return configFileFlag, nil
	}
	if strings.TrimSpace(configFileUsed) != "" {
		return configFileUsed, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".gohour.yaml"), nil
}

func ensureConfigFileWithTemplate(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("checking config file failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("creating config directory failed: %w", err)
	}
	if err := os.WriteFile(path, []byte(config.ExampleYAML()), 0o600); err != nil {
		return false, fmt.Errorf("creating example config failed: %w", err)
	}

	return true, nil
}

func resolveEditorValue(visual, editor string) string {
	if strings.TrimSpace(visual) != "" {
		return visual
	}
	if strings.TrimSpace(editor) != "" {
		return editor
	}
	return "vi"
}

func buildEditorCommand(editorValue, configPath string) (*exec.Cmd, error) {
	fields := strings.Fields(strings.TrimSpace(editorValue))
	if len(fields) == 0 {
		return nil, fmt.Errorf("editor command is empty")
	}

	args := append(fields[1:], configPath)
	return exec.Command(fields[0], args...), nil
}

func init() {
	configCmd.AddCommand(configEditCmd)
}
