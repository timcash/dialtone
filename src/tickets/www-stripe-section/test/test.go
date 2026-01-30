package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
	"os"
	"strings"
)

func init() {
	dialtest.RegisterTicket("www-stripe-section")
	dialtest.AddSubtaskTest("init", RunInitTest, nil)
	dialtest.AddSubtaskTest("component-exists", ComponentExists, []string{"init"})
	dialtest.AddSubtaskTest("html-section-exists", HTMLSectionExists, []string{"init"})
	dialtest.AddSubtaskTest("main-registered", MainRegistered, []string{"init"})
}

// RunInitTest validates initial setup
func RunInitTest() error {
	// Verify the www plugin structure exists
	path := "src/plugins/www/app/src/components"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("www components directory not found")
	}
	return nil
}

// ComponentExists verifies stripe.ts component file exists
func ComponentExists() error {
	path := "src/plugins/www/app/src/components/stripe.ts"
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("stripe.ts not found: %w", err)
	}

	// Check for required exports
	if !strings.Contains(string(content), "export function mountStripe") {
		return fmt.Errorf("stripe.ts missing mountStripe export")
	}

	// Check for VisualizationControl implementation
	if !strings.Contains(string(content), "VisualizationControl") {
		return fmt.Errorf("stripe.ts missing VisualizationControl implementation")
	}

	return nil
}

// HTMLSectionExists verifies the stripe section is in index.html
func HTMLSectionExists() error {
	path := "src/plugins/www/app/index.html"
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("index.html not found: %w", err)
	}

	// Check for stripe section
	if !strings.Contains(string(content), `id="s-stripe"`) {
		return fmt.Errorf("index.html missing s-stripe section")
	}

	// Check for stripe container
	if !strings.Contains(string(content), `id="stripe-container"`) {
		return fmt.Errorf("index.html missing stripe-container")
	}

	return nil
}

// MainRegistered verifies stripe section is registered in main.ts
func MainRegistered() error {
	path := "src/plugins/www/app/src/main.ts"
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("main.ts not found: %w", err)
	}

	// Check for section registration
	if !strings.Contains(string(content), `sections.register('s-stripe'`) {
		return fmt.Errorf("main.ts missing s-stripe registration")
	}

	// Check for stripe import
	if !strings.Contains(string(content), "mountStripe") {
		return fmt.Errorf("main.ts missing mountStripe import")
	}

	return nil
}
