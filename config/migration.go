package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type migrationOptions struct {
	cwd     string
	home    string
	dataDir string
	input   io.Reader
	output  io.Writer
}

type oldFiles struct {
	configPath string
	dbPath     string
}

// RunMigration ensures the fixed gohour data directory exists and offers to move
// legacy files from the current directory or home directory on first run.
func RunMigration() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	return runMigration(migrationOptions{
		cwd:     cwd,
		home:    home,
		dataDir: DataDir(),
		input:   os.Stdin,
		output:  os.Stdout,
	})
}

func runMigration(opts migrationOptions) error {
	opts = normalizeMigrationOptions(opts)
	newConfig := filepath.Join(opts.dataDir, "config.yaml")
	if _, err := os.Stat(newConfig); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config: %w", err)
	}

	found, err := findOldFiles(opts.cwd, opts.home)
	if err != nil {
		return err
	}
	if found.configPath == "" && found.dbPath == "" {
		return writeDefaultConfigIfMissing(opts.dataDir)
	}

	action, err := promptMigrationAction(opts.output, opts.input, opts.dataDir, found)
	if err != nil {
		return err
	}
	if action == "fresh" {
		return writeDefaultConfigIfMissing(opts.dataDir)
	}
	if err := os.MkdirAll(opts.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if found.configPath != "" {
		if err := copyAndBackup(found.configPath, filepath.Join(opts.dataDir, "config.yaml")); err != nil {
			return err
		}
	}
	if found.dbPath != "" {
		if err := copyAndBackup(found.dbPath, filepath.Join(opts.dataDir, "gohour.db")); err != nil {
			return err
		}
	}
	return nil
}

func normalizeMigrationOptions(opts migrationOptions) migrationOptions {
	if opts.dataDir == "" {
		opts.dataDir = DataDir()
	}
	if opts.input == nil {
		opts.input = os.Stdin
	}
	if opts.output == nil {
		opts.output = io.Discard
	}
	return opts
}

func findOldFiles(cwd, home string) (oldFiles, error) {
	if cwd != "" {
		candidate := oldFiles{
			configPath: filepath.Join(cwd, ".gohour.yaml"),
			dbPath:     filepath.Join(cwd, "gohour.db"),
		}
		found, err := existingOldFiles(candidate)
		if err != nil {
			return oldFiles{}, err
		}
		if found.configPath != "" || found.dbPath != "" {
			return found, nil
		}
	}
	if home != "" {
		return existingOldFiles(oldFiles{configPath: filepath.Join(home, ".gohour.yaml")})
	}
	return oldFiles{}, nil
}

func existingOldFiles(candidate oldFiles) (oldFiles, error) {
	var out oldFiles
	if candidate.configPath != "" {
		exists, err := fileExists(candidate.configPath)
		if err != nil {
			return oldFiles{}, err
		}
		if exists {
			out.configPath = candidate.configPath
		}
	}
	if candidate.dbPath != "" {
		exists, err := fileExists(candidate.dbPath)
		if err != nil {
			return oldFiles{}, err
		}
		if exists {
			out.dbPath = candidate.dbPath
		}
	}
	return out, nil
}

func fileExists(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
}

func promptMigrationAction(output io.Writer, input io.Reader, dataDir string, found oldFiles) (string, error) {
	fmt.Fprintf(output, "Found existing gohour files:\n")
	if found.configPath != "" {
		fmt.Fprintf(output, "- %s\n", found.configPath)
	}
	if found.dbPath != "" {
		fmt.Fprintf(output, "- %s\n", found.dbPath)
	}
	fmt.Fprintf(output, "Move existing files to %s? Enter \"move\" to move, or \"fresh\" to start fresh: ", dataDir)

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
		case "move", "m":
			return "move", nil
		case "fresh", "f", "":
			return "fresh", nil
		default:
			fmt.Fprint(output, "Please enter \"move\" or \"fresh\": ")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read migration response: %w", err)
	}
	return "fresh", nil
}

func writeDefaultConfigIfMissing(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	configPath := filepath.Join(dataDir, "config.yaml")
	exists, err := fileExists(configPath)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := os.WriteFile(configPath, []byte(ExampleYAML()), 0o600); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}
	return nil
}

func copyAndBackup(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read legacy file %s: %w", src, err)
	}
	if err := os.WriteFile(dst, content, 0o600); err != nil {
		return fmt.Errorf("write migrated file %s: %w", dst, err)
	}
	if err := os.Rename(src, src+".bak"); err != nil {
		return fmt.Errorf("backup legacy file %s: %w", src, err)
	}
	return nil
}
