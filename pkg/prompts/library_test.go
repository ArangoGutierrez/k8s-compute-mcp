// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package prompts

import (
	"strings"
	"testing"
	"text/template"
)

func TestGetPrompts_NonEmpty(t *testing.T) {
	prompts := GetPrompts()
	if len(prompts) == 0 {
		t.Fatal("GetPrompts() returned empty slice")
	}
}

func TestGetPrompts_RequiredFields(t *testing.T) {
	for _, p := range GetPrompts() {
		t.Run(p.Name, func(t *testing.T) {
			if p.Name == "" {
				t.Error("prompt Name is empty")
			}
			if p.Description == "" {
				t.Errorf("prompt %q has empty Description", p.Name)
			}
			if p.Template == "" {
				t.Errorf("prompt %q has empty Template", p.Name)
			}
			if len(p.Arguments) == 0 {
				t.Errorf("prompt %q has no Arguments", p.Name)
			}
		})
	}
}

func TestGetPrompts_ArgumentsWellFormed(t *testing.T) {
	for _, p := range GetPrompts() {
		t.Run(p.Name, func(t *testing.T) {
			for _, arg := range p.Arguments {
				if arg.Name == "" {
					t.Errorf("prompt %q has argument with empty Name", p.Name)
				}
				if arg.Description == "" {
					t.Errorf("prompt %q argument %q has empty Description", p.Name, arg.Name)
				}
			}
		})
	}
}

func TestGetPrompts_TemplatesCompile(t *testing.T) {
	for _, p := range GetPrompts() {
		t.Run(p.Name, func(t *testing.T) {
			tmpl, err := template.New(p.Name).Parse(p.Template)
			if err != nil {
				t.Fatalf("prompt %q template failed to parse: %v", p.Name, err)
			}

			// Verify template references match declared arguments
			for _, arg := range p.Arguments {
				placeholder := "{{." + arg.Name + "}}"
				if !strings.Contains(p.Template, placeholder) {
					t.Errorf("prompt %q declares argument %q but template does not contain %s",
						p.Name, arg.Name, placeholder)
				}
			}

			_ = tmpl // template compiles successfully
		})
	}
}

func TestGetPrompts_UniqueNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range GetPrompts() {
		if seen[p.Name] {
			t.Errorf("duplicate prompt name: %q", p.Name)
		}
		seen[p.Name] = true
	}
}
