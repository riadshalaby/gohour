package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	deleteDBPath string
)

var (
	deletePromptInput  io.Reader = os.Stdin
	deletePromptOutput io.Writer = os.Stdout
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the complete SQLite database file",
	Long: `Destructive database cleanup command.

This command always deletes the complete SQLite database file.
Before deletion, an interactive security prompt requires typing exactly "Y".`,
	Example: `
  # Delete the complete SQLite file (requires interactive confirmation)
  gohour delete --db ./gohour.db
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		confirmed, err := confirmDeletePrompt(deletePromptInput, deletePromptOutput, deleteDBPath)
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("delete aborted: confirmation was not 'Y'")
		}

		if err := removeDatabaseFile(deleteDBPath); err != nil {
			return err
		}
		fmt.Printf("Deleted database file: %s\n", deleteDBPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().StringVar(&deleteDBPath, "db", "./gohour.db", "Path to local SQLite database")
}

func confirmDeletePrompt(input io.Reader, output io.Writer, path string) (bool, error) {
	if input == nil {
		return false, fmt.Errorf("delete confirmation input is not available")
	}

	if output == nil {
		output = io.Discard
	}

	if _, err := fmt.Fprintf(output, "Delete database file %q? Type Y to confirm: ", path); err != nil {
		return false, fmt.Errorf("write delete confirmation prompt: %w", err)
	}

	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			line = strings.TrimSpace(line)
			return line == "Y", nil
		}
		return false, fmt.Errorf("read delete confirmation: %w", err)
	}
	return strings.TrimSpace(line) == "Y", nil
}

func removeDatabaseFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("database file not found: %s", path)
		}
		return fmt.Errorf("stat database file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("database path is a directory: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete database file: %w", err)
	}
	return nil
}
