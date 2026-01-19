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

package config

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

const (
	toolSpecsLabel = "kentra.sh/resource-type"
	toolSpecsValue = "tool-specs"
	toolsDataKey   = "tools"
)

// ToolConfig represents the configuration for a single tool
type ToolConfig struct {
	Type              string                 `yaml:"type"`
	Image             string                 `yaml:"image"`
	CommandTemplate   string                 `yaml:"commandTemplate"`
	AssetTypeFlags    map[string]string      `yaml:"assetTypeFlags"`
	Separator         string                 `yaml:"separator"`
	EndpointSeparator string                 `yaml:"endpointSeparator"`
	PortSeparator     string                 `yaml:"portSeparator"`
	Capabilities      map[string]interface{} `yaml:"capabilities"`
}

// ToolsConfigurator manages tool configurations loaded from ConfigMaps
type ToolsConfigurator struct {
	mu          sync.RWMutex
	tools       map[string]*ToolConfig
	client      client.Client
	configMapNS string
}

// constructor for a new ToolsConfigurator instance
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
		client.MatchingLabels{toolSpecsLabel: toolSpecsValue},
	); err != nil {
		log.Error(err, "Failed to list tool-specs ConfigMaps", "Namespace", tc.configMapNS)
		return err
	}

	if len(cmList.Items) == 0 {
		log.Info("No ConfigMaps found with label", "label", fmt.Sprintf("%s: %s", toolSpecsLabel, toolSpecsValue), "Namespace", tc.configMapNS)
		return fmt.Errorf("no tool-specs ConfigMaps found in namespace %s", tc.configMapNS)
	}

	log.Info("Found tool-specs ConfigMaps", "Count", len(cmList.Items), "Namespace", tc.configMapNS)

	// Merge tools from all ConfigMaps
	mergedTools, err := tc.mergeToolsFromConfigMaps(ctx, cmList.Items)
	if err != nil {
		return err
	}

	// Update tools with lock
	tc.mu.Lock()
	tc.tools = mergedTools
	tc.mu.Unlock()

	log.Info("Successfully loaded all tool configurations", "TotalToolCount", len(mergedTools))
	return nil
}

// mergeToolsFromConfigMaps merges tools from multiple ConfigMaps
func (tc *ToolsConfigurator) mergeToolsFromConfigMaps(ctx context.Context, configMaps []corev1.ConfigMap) (map[string]*ToolConfig, error) {
	log := log.FromContext(ctx)
	mergedTools := make(map[string]*ToolConfig)

	for _, cm := range configMaps {
		toolsYAML, ok := cm.Data[toolsDataKey]
		if !ok {
			log.Info("Skipping ConfigMap without tools key", "ConfigMap", cm.Name, "Key", toolsDataKey)
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

	return mergedTools, nil
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

	return tc.filterTools(func(config *ToolConfig) bool {
		return config.Type == toolType
	})
}

// GetAllTools returns all configured tools
func (tc *ToolsConfigurator) GetAllTools() map[string]*ToolConfig {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	return tc.filterTools(func(config *ToolConfig) bool {
		return true // Return all tools
	})
}

// filterTools is a helper that filters tools based on a predicate
func (tc *ToolsConfigurator) filterTools(predicate func(*ToolConfig) bool) map[string]*ToolConfig {
	result := make(map[string]*ToolConfig)
	for name, config := range tc.tools {
		if predicate(config) {
			result[name] = config
		}
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

// templateData represents common template data structure
type templateData map[string]interface{}

// BuildCommand builds the command from the template using Go's text/template
func (tc *ToolsConfigurator) BuildCommand(toolName string, target string, port string, args []string) ([]string, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	// Get separator with default
	separator := config.Separator
	if separator == "" {
		separator = " "
	}

	// Create structured Target object (ONLY structured syntax supported)
	targetObj := map[string]string{
		"endpoint": target,
		"port":     port,
	}

	data := templateData{
		"Target":    targetObj, // Structured object with .endpoint and .port
		"Args":      strings.Join(args, " "),
		"Separator": separator,
	}

	return tc.executeTemplate(toolName, data)
}

// BuildCommandWithModule builds command with Module and Payload for exploit resources
func (tc *ToolsConfigurator) BuildCommandWithModule(toolName string, target string, port string, module string, payload string, args []string) ([]string, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	// Get separator with default
	separator := config.Separator
	if separator == "" {
		separator = " "
	}

	data := templateData{
		"Target":    target,
		"Port":      port,
		"Module":    module,
		"Payload":   payload,
		"Args":      strings.Join(args, " "),
		"Item":      target,
		"Separator": separator,
	}

	return tc.executeTemplate(toolName, data)
}

// BuildCommandWithAssets builds command with assets as individual or grouped template variables
func (tc *ToolsConfigurator) BuildCommandWithAssets(toolName string, assets []securityv1alpha1.AssetItem, args []string) ([]string, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	// Determine separator (default to space if not specified)
	separator := config.Separator
	if separator == "" {
		separator = " "
	}

	// Build asset maps with separator support
	assetMap, assetArrayMap := tc.buildAssetMaps(assets, separator)

	data := templateData{
		"Args":      strings.Join(args, " "),
		"Item":      assetMap,
		"ItemArray": assetArrayMap,
		"Separator": separator,
	}

	return tc.executeTemplate(toolName, data)
}

// buildAssetMaps creates both string and array representations of assets
func (tc *ToolsConfigurator) buildAssetMaps(assets []securityv1alpha1.AssetItem, separator string) (map[string]string, map[string][]string) {
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

	return assetMap, assetArrayMap
}

// getConfigSafe safely retrieves a tool config with read lock
func (tc *ToolsConfigurator) getConfigSafe(toolName string) (*ToolConfig, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	config, ok := tc.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("tool %q not found in configuration", toolName)
	}

	return config, nil
}

// executeTemplate executes the command template with the given data
func (tc *ToolsConfigurator) executeTemplate(toolName string, data templateData) ([]string, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("cmd").Parse(config.CommandTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command template: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("failed to execute command template: %w", err)
	}

	// Clean up the output
	resultString := out.String()
	resultString = strings.ReplaceAll(resultString, "<no value>", "")
	resultString = strings.Join(strings.Fields(resultString), " ")

	return strings.Fields(resultString), nil
}

// GetCapabilities extracts and returns capabilities for a tool
func (tc *ToolsConfigurator) GetCapabilities(toolName string) ([]string, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	if config.Capabilities == nil {
		return []string{}, nil
	}

	add, ok := config.Capabilities["add"].([]interface{})
	if !ok {
		return []string{}, nil
	}

	capabilities := make([]string, 0, len(add))
	for _, cap := range add {
		if capStr, ok := cap.(string); ok {
			capabilities = append(capabilities, capStr)
		}
	}

	return capabilities, nil
}

// GetRequiredAssetTypes parses the command template and extracts required asset types
func (tc *ToolsConfigurator) GetRequiredAssetTypes(toolName string) ([]string, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	// Parse the template to find all {{.Item.xxx}} references
	assetTypes := make(map[string]bool)

	// Use regex to find all {{.Item.xxx}} patterns
	// Match patterns like {{.Item.username}}, {{.Item.email}}, etc.
	re := regexp.MustCompile(`\{\{\.Item\.(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(config.CommandTemplate, -1)

	for _, match := range matches {
		if len(match) > 1 {
			assetType := match[1] // Extract the asset type (e.g., "username", "email")
			assetTypes[assetType] = true
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(assetTypes))
	for assetType := range assetTypes {
		result = append(result, assetType)
	}

	return result, nil
}

// GetTemplateVariables extracts all template variables from a tool's command template
// Returns a map of variable names (e.g., "Port", "Target", "Target.endpoint")
func (tc *ToolsConfigurator) GetTemplateVariables(toolName string) (map[string]bool, error) {
	config, err := tc.getConfigSafe(toolName)
	if err != nil {
		return nil, err
	}

	// Regex to match {{.VariableName}} and {{.VariableName.field}} patterns
	re := regexp.MustCompile(`\{\{\.(\w+(?:\.\w+)?)\}\}`)
	matches := re.FindAllStringSubmatch(config.CommandTemplate, -1)

	variables := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			// Extract the full variable path (e.g., "Port", "Target.endpoint", "Target.port")
			varPath := match[1]
			variables[varPath] = true
		}
	}

	return variables, nil
}

// UsesVariable checks if a tool's template uses a specific variable or nested field
func (tc *ToolsConfigurator) UsesVariable(toolName, variableName string) (bool, error) {
	variables, err := tc.GetTemplateVariables(toolName)
	if err != nil {
		return false, err
	}

	// Check exact match or prefix match for nested fields
	// e.g., "Target.endpoint" matches when checking "Target"
	for varPath := range variables {
		if varPath == variableName || strings.HasPrefix(varPath, variableName+".") {
			return true, nil
		}
	}

	return false, nil
}
