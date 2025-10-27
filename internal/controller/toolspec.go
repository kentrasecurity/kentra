package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ------------------------------
// Types
// ------------------------------

type ToolSpec struct {
	Image           string `yaml:"image"`
	CommandTemplate string `yaml:"commandTemplate"`
	Capabilities    struct {
		Add []string `yaml:"add,omitempty"`
	} `yaml:"capabilities,omitempty"`
}

type ToolCapabilities struct {
	Add []string
}

// ------------------------------
// ToolSpecManager
// ------------------------------

type ToolSpecManager struct {
	client    client.Client
	namespace string
	mu        sync.RWMutex
	tools     map[string]ToolSpec
	loaded    bool
}

func NewToolSpecManager(c client.Client, ns string) *ToolSpecManager {
	return &ToolSpecManager{
		client:    c,
		namespace: ns,
		tools:     make(map[string]ToolSpec),
	}
}

// ------------------------------
// Loading and parsing ConfigMap
// ------------------------------

func (m *ToolSpecManager) LoadToolSpecs(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		return nil
	}

	var cm corev1.ConfigMap
	if err := m.client.Get(ctx, client.ObjectKey{Name: "tool-specs", Namespace: m.namespace}, &cm); err != nil {
		return fmt.Errorf("failed to get tool-specs ConfigMap: %w", err)
	}

	raw, ok := cm.Data["tools"]
	if !ok {
		return fmt.Errorf("ConfigMap missing 'tools' key")
	}

	parsed := make(map[string]ToolSpec)
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("failed to unmarshal tools YAML: %w", err)
	}

	m.tools = parsed
	m.loaded = true
	return nil
}

// ------------------------------
// Accessors
// ------------------------------

func (m *ToolSpecManager) GetToolImage(tool string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tools[tool]
	if !ok {
		return "", fmt.Errorf("tool %q not found in configuration", tool)
	}
	return t.Image, nil
}

func (m *ToolSpecManager) BuildCommand(tool string, target string, args []string) ([]string, error) {
	m.mu.RLock()
	t, ok := m.tools[tool]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %q not found in configuration", tool)
	}

	tmpl, err := template.New("cmd").Parse(t.CommandTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command template: %w", err)
	}

	data := map[string]interface{}{
		"Target": target,
		"Args":   strings.Join(args, " "),
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("failed to execute command template: %w", err)
	}

	cmd := strings.Fields(out.String())
	return cmd, nil
}

func (m *ToolSpecManager) GetToolCapabilities(tool string) (*ToolCapabilities, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tools[tool]
	if !ok {
		return nil, fmt.Errorf("tool %q not found in configuration", tool)
	}

	if len(t.Capabilities.Add) == 0 {
		return nil, nil
	}

	return &ToolCapabilities{
		Add: t.Capabilities.Add,
	}, nil
}
