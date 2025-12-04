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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// ToolConfig represents the configuration for a single tool
type ToolConfig struct {
	Type            string                 `yaml:"type"`
	Image           string                 `yaml:"image"`
	CommandTemplate string                 `yaml:"commandTemplate"`
	AssetTypeFlags  map[string]string      `yaml:"assetTypeFlags"`
	Separator       string                 `yaml:"separator"`
	Capabilities    map[string]interface{} `yaml:"capabilities"`
}

// ToolsConfigurator manages tool configurations loaded from ConfigMaps
type ToolsConfigurator struct {
	mu          sync.RWMutex
	tools       map[string]*ToolConfig
	client      client.Client
	configMapNS string
}

// NewToolsConfigurator creates a new ToolsConfigurator instance
func NewToolsConfigurator(c client.Client, configMapNS string) *ToolsConfigurator {
	return &ToolsConfigurator{
		tools:       make(map[string]*ToolConfig),
		client:      c,
		configMapNS: configMapNS,
	}
}

// LoadConfig loads the tool configurations from all ConfigMaps with the label kentra.sh/resource-type: tool-specs
func (tc *ToolsConfigurator) LoadConfig(ctx context.Context) error {
	log := log.FromContext(ctx)

	// List all ConfigMaps in the namespace with the label selector
	cmList := &corev1.ConfigMapList{}
	if err := tc.client.List(ctx, cmList,
		client.InNamespace(tc.configMapNS),
		client.MatchingLabels{"kentra.sh/resource-type": "tool-specs"},
	); err != nil {
		log.Error(err, "Failed to list tool-specs ConfigMaps", "Namespace", tc.configMapNS)
		return err
	}

	if len(cmList.Items) == 0 {
		log.Info("No ConfigMaps found with label kentra.sh/resource-type: tool-specs", "Namespace", tc.configMapNS)
		return fmt.Errorf("no tool-specs ConfigMaps found in namespace %s", tc.configMapNS)
	}

	log.Info("Found tool-specs ConfigMaps", "Count", len(cmList.Items), "Namespace", tc.configMapNS)

	// Merge tools from all ConfigMaps
	mergedTools := make(map[string]*ToolConfig)

	for _, cm := range cmList.Items {
		toolsYAML, ok := cm.Data["tools"]
		if !ok {
			log.Info("Skipping ConfigMap without 'tools' key", "ConfigMap", cm.Name)
			continue
		}

		// Parse YAML into tools map
		toolsMap := make(map[string]*ToolConfig)
		if err := yaml.Unmarshal([]byte(toolsYAML), &toolsMap); err != nil {
			log.Error(err, "Failed to unmarshal tools YAML from ConfigMap", "ConfigMap", cm.Name)
			continue
		}

		// Merge tools into the main map (later ConfigMaps override earlier ones)
		for toolName, toolConfig := range toolsMap {
			if _, exists := mergedTools[toolName]; exists {
				log.Info("Tool configuration overridden", "Tool", toolName, "ConfigMap", cm.Name)
			}
			mergedTools[toolName] = toolConfig
		}

		log.Info("Loaded tools from ConfigMap", "ConfigMap", cm.Name, "ToolCount", len(toolsMap))
	}

	// Update tools with lock
	tc.mu.Lock()
	tc.tools = mergedTools
	tc.mu.Unlock()

	log.Info("Successfully loaded all tool configurations", "TotalToolCount", len(mergedTools))
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
		"Item":   target, // Alias for Target, used by AssetPool-based commands
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("failed to execute command template: %w", err)
	}

	cmd := strings.Fields(out.String())
	return cmd, nil
}

// BuildCommandWithAssets builds command with assets as individual or grouped template variables
func (tc *ToolsConfigurator) BuildCommandWithAssets(toolName string, assets []securityv1alpha1.AssetItem, args []string) ([]string, error) {
	tc.mu.RLock()
	config, ok := tc.tools[toolName]
	tc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %q not found in configuration", toolName)
	}

	// Determine separator (default to space if not specified)
	separator := " "
	if config.Separator != "" {
		separator = config.Separator
	}

	// Build asset maps - supports multiple values per type
	assetMap := make(map[string]string)
	assetArrayMap := make(map[string][]string)

	for _, asset := range assets {
		// Store as single string (with custom separator for multiple values)
		if existing, exists := assetMap[asset.Type]; exists {
			assetMap[asset.Type] = existing + separator + asset.Value
		} else {
			assetMap[asset.Type] = asset.Value
		}

		// Also store as array for templates that need iteration
		assetArrayMap[asset.Type] = append(assetArrayMap[asset.Type], asset.Value)
	}

	tmpl, err := template.New("cmd").Parse(config.CommandTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command template: %w", err)
	}

	data := map[string]interface{}{
		"Args":      strings.Join(args, " "),
		"Item":      assetMap,
		"ItemArray": assetArrayMap,
		"Separator": separator,
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("failed to execute command template: %w", err)
	}

	resultString := out.String()
	resultString = strings.ReplaceAll(resultString, "<no value>", "")
	resultString = strings.Join(strings.Fields(resultString), " ")

	cmd := strings.Fields(resultString)
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
