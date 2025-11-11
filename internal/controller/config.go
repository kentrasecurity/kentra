/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// ToolConfig represents the configuration for a single tool
type ToolConfig struct {
	Type            string                 `yaml:"type"`
	Image           string                 `yaml:"image"`
	CommandTemplate string                 `yaml:"commandTemplate"`
	Capabilities    map[string]interface{} `yaml:"capabilities"`
}

// ToolsConfigurator manages tool configurations loaded from ConfigMap
type ToolsConfigurator struct {
	mu            sync.RWMutex
	tools         map[string]*ToolConfig
	client        client.Client
	configMapName string
	configMapNS   string
}

// NewToolsConfigurator creates a new ToolsConfigurator instance
func NewToolsConfigurator(c client.Client, configMapName, configMapNS string) *ToolsConfigurator {
	return &ToolsConfigurator{
		tools:         make(map[string]*ToolConfig),
		client:        c,
		configMapName: configMapName,
		configMapNS:   configMapNS,
	}
}

// LoadConfig loads the tool configurations from the ConfigMap
func (tc *ToolsConfigurator) LoadConfig(ctx context.Context) error {
	log := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	if err := tc.client.Get(ctx, types.NamespacedName{
		Name:      tc.configMapName,
		Namespace: tc.configMapNS,
	}, cm); err != nil {
		log.Error(err, "Failed to get tool-specs ConfigMap", "ConfigMap", fmt.Sprintf("%s/%s", tc.configMapNS, tc.configMapName))
		return err
	}

	toolsYAML, ok := cm.Data["tools"]
	if !ok {
		err := fmt.Errorf("'tools' key not found in ConfigMap")
		log.Error(err, "Missing 'tools' key in ConfigMap data")
		return err
	}

	// Parse YAML into tools map
	toolsMap := make(map[string]*ToolConfig)
	if err := yaml.Unmarshal([]byte(toolsYAML), &toolsMap); err != nil {
		log.Error(err, "Failed to unmarshal tools YAML")
		return err
	}

	// Update tools with lock
	tc.mu.Lock()
	tc.tools = toolsMap
	tc.mu.Unlock()

	log.Info("Successfully loaded tool configurations", "ToolCount", len(toolsMap))
	return nil
}

// GetToolConfig retrieves the configuration for a specific tool
func (tc *ToolsConfigurator) GetToolConfig(toolName string) (*ToolConfig, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	config, ok := tc.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("tool configuration not found: %s", toolName)
	}

	return config, nil
}

// GetToolsByType retrieves all tools of a specific type (e.g., "enumeration")
func (tc *ToolsConfigurator) GetToolsByType(toolType string) map[string]*ToolConfig {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	result := make(map[string]*ToolConfig)
	for name, config := range tc.tools {
		if config.Type == toolType {
			result[name] = config
		}
	}
	return result
}

// GetAllTools returns all configured tools
func (tc *ToolsConfigurator) GetAllTools() map[string]*ToolConfig {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	result := make(map[string]*ToolConfig)
	for name, config := range tc.tools {
		result[name] = config
	}
	return result
}

// IsToolAvailable checks if a tool is configured
func (tc *ToolsConfigurator) IsToolAvailable(toolName string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	_, ok := tc.tools[toolName]
	return ok
}

// BuildCommand builds the command from the template using Go's text/template
// This replaces the simple string replacement approach with proper template handling
func (tc *ToolsConfigurator) BuildCommand(toolName string, target string, port string, args []string) ([]string, error) {
	tc.mu.RLock()
	config, ok := tc.tools[toolName]
	tc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %q not found in configuration", toolName)
	}

	tmpl, err := template.New("cmd").Parse(config.CommandTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command template: %w", err)
	}

	data := map[string]interface{}{
		"Target": target,
		"Port":   port,
		"Args":   strings.Join(args, " "),
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("failed to execute command template: %w", err)
	}

	cmd := strings.Fields(out.String())
	return cmd, nil
}

// GetCapabilities extracts and returns capabilities for a tool
func (tc *ToolsConfigurator) GetCapabilities(toolName string) ([]string, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	config, ok := tc.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("tool %q not found in configuration", toolName)
	}

	if config.Capabilities == nil {
		return []string{}, nil
	}

	if add, ok := config.Capabilities["add"].([]interface{}); ok {
		capabilities := make([]string, 0, len(add))
		for _, cap := range add {
			if capStr, ok := cap.(string); ok {
				capabilities = append(capabilities, capStr)
			}
		}
		return capabilities, nil
	}

	return []string{}, nil
}
