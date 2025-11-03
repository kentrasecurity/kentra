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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestToolsConfiguratorLoadConfig tests loading tool configurations from ConfigMap
func TestToolsConfiguratorLoadConfig(t *testing.T) {
	// Create fake client with ConfigMap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tool-specs",
			Namespace: "kttack-system",
		},
		Data: map[string]string{
			"tools": `nmap:
  type: "enumeration"
  image: "instrumentisto/nmap:latest"
  commandTemplate: "/bin/nmap {{.Args}} {{.Target}}"
  capabilities:
    add:
      - NET_RAW
nikto:
  type: "scanning"
  image: "ghcr.io/sullo/nikto:latest"
  commandTemplate: "/nikto -h {{.Target}} {{.Args}}"
  capabilities:
    add: []`,
		},
	}

	client := fake.NewClientBuilder().WithObjects(cm).Build()
	configurator := NewToolsConfigurator(client, "tool-specs", "kttack-system")

	// Load config
	err := configurator.LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify tools are loaded
	tools := configurator.GetAllTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// Check nmap config
	nmap, err := configurator.GetToolConfig("nmap")
	if err != nil {
		t.Fatalf("Failed to get nmap config: %v", err)
	}
	if nmap.Image != "instrumentisto/nmap:latest" {
		t.Errorf("Expected image 'instrumentisto/nmap:latest', got %s", nmap.Image)
	}
	if nmap.Type != "enumeration" {
		t.Errorf("Expected type 'enumeration', got %s", nmap.Type)
	}
}

// TestToolsConfiguratorBuildCommand tests command building from templates
func TestToolsConfiguratorBuildCommand(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tool-specs",
			Namespace: "kttack-system",
		},
		Data: map[string]string{
			"tools": `nmap:
  type: "enumeration"
  image: "instrumentisto/nmap:latest"
  commandTemplate: "/bin/nmap {{.Args}} {{.Target}}"
  capabilities:
    add:
      - NET_RAW`,
		},
	}

	client := fake.NewClientBuilder().WithObjects(cm).Build()
	configurator := NewToolsConfigurator(client, "tool-specs", "kttack-system")
	configurator.LoadConfig(context.Background())

	// Test command building
	cmd, err := configurator.BuildCommand("nmap", "192.168.1.0/24", []string{"-sV", "-p", "22,80,443"})
	if err != nil {
		t.Fatalf("Failed to build command: %v", err)
	}

	// Verify command structure
	if len(cmd) < 2 {
		t.Errorf("Expected at least 2 command parts, got %d", len(cmd))
	}
	if cmd[0] != "/bin/nmap" {
		t.Errorf("Expected first part to be '/bin/nmap', got %s", cmd[0])
	}

	// Check that target and args are present
	cmdStr := ""
	for _, part := range cmd {
		cmdStr += part + " "
	}
	if !contains(cmd, "192.168.1.0/24") {
		t.Errorf("Expected target in command, got: %s", cmdStr)
	}
}

// TestToolsConfiguratorGetCapabilities tests capability extraction
func TestToolsConfiguratorGetCapabilities(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tool-specs",
			Namespace: "kttack-system",
		},
		Data: map[string]string{
			"tools": `nmap:
  type: "enumeration"
  image: "instrumentisto/nmap:latest"
  commandTemplate: "/bin/nmap {{.Target}}"
  capabilities:
    add:
      - NET_RAW`,
		},
	}

	client := fake.NewClientBuilder().WithObjects(cm).Build()
	configurator := NewToolsConfigurator(client, "tool-specs", "kttack-system")
	configurator.LoadConfig(context.Background())

	// Get capabilities
	caps, err := configurator.GetCapabilities("nmap")
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	if len(caps) != 1 {
		t.Errorf("Expected 1 capability, got %d", len(caps))
	}
	if caps[0] != "NET_RAW" {
		t.Errorf("Expected capability 'NET_RAW', got %s", caps[0])
	}
}

// TestToolsConfiguratorGetToolsByType tests filtering by type
func TestToolsConfiguratorGetToolsByType(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tool-specs",
			Namespace: "kttack-system",
		},
		Data: map[string]string{
			"tools": `nmap:
  type: "enumeration"
  image: "instrumentisto/nmap:latest"
  commandTemplate: "/bin/nmap {{.Target}}"
  capabilities:
    add: []
nikto:
  type: "scanning"
  image: "ghcr.io/sullo/nikto:latest"
  commandTemplate: "/nikto {{.Target}}"
  capabilities:
    add: []
masscan:
  type: "enumeration"
  image: "robertdavidgraham/masscan:latest"
  commandTemplate: "/bin/masscan {{.Target}}"
  capabilities:
    add:
      - NET_RAW`,
		},
	}

	client := fake.NewClientBuilder().WithObjects(cm).Build()
	configurator := NewToolsConfigurator(client, "tool-specs", "kttack-system")
	configurator.LoadConfig(context.Background())

	// Get enumeration tools
	enumTools := configurator.GetToolsByType("enumeration")
	if len(enumTools) != 2 {
		t.Errorf("Expected 2 enumeration tools, got %d", len(enumTools))
	}

	// Get scanning tools
	scanTools := configurator.GetToolsByType("scanning")
	if len(scanTools) != 1 {
		t.Errorf("Expected 1 scanning tool, got %d", len(scanTools))
	}
}

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
