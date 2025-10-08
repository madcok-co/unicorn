// ============================================
// 1. CONFIG LOADER (YAML + Env Substitution)
// ============================================
package utils

import (
	"fmt"
	"os"
	"regexp"

	"github.com/madcok-co/unicorn"

	"gopkg.in/yaml.v2"
)

type ConfigLoader struct {
	envPrefix string
}

func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		envPrefix: "",
	}
}

func (l *ConfigLoader) Load(path string) (*unicorn.Config, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config unicorn.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func (l *ConfigLoader) LoadWithEnvSubstitution(path string) (*unicorn.Config, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	data = l.substituteEnvVars(data)

	// Parse YAML
	var config unicorn.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func (l *ConfigLoader) substituteEnvVars(data []byte) []byte {
	// Replace ${VAR_NAME} with environment variable value
	envRegex := regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

	return envRegex.ReplaceAllFunc(data, func(match []byte) []byte {
		// Extract variable name
		varName := string(match[2 : len(match)-1])

		// Get environment variable
		value := os.Getenv(varName)
		if value == "" {
			// Return original if not found
			return match
		}

		return []byte(value)
	})
}
