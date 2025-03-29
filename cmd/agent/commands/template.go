package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/NethermindEth/chaoschain-launchpad/cmd/agent/templates"
	"github.com/spf13/cobra"
)

var (
	templateName        string
	templateRole        string
	templateTraits      string
	templateStyle       string
	templateDescription string
)

// TemplateCmd represents the template command
var TemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage agent templates",
	Long:  `Create, list, and manage agent templates.`,
}

// templateCreateCmd represents the template create command
var templateCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent template",
	Long:  `Create a new agent template for reuse.`,
	Run: func(cmd *cobra.Command, args []string) {
		createTemplate()
	},
}

// templateListCmd represents the template list command
var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent templates",
	Long:  `List all available agent templates.`,
	Run: func(cmd *cobra.Command, args []string) {
		listTemplates()
	},
}

// templateShowCmd represents the template show command
var templateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show template details",
	Long:  `Show details of a specific template.`,
	Run: func(cmd *cobra.Command, args []string) {
		if templateName == "" {
			fmt.Println("Error: template name is required")
			os.Exit(1)
		}
		showTemplate()
	},
}

func init() {
	// Add subcommands to template command
	TemplateCmd.AddCommand(templateCreateCmd)
	TemplateCmd.AddCommand(templateListCmd)
	TemplateCmd.AddCommand(templateShowCmd)
	
	// Add flags for template create command
	templateCreateCmd.Flags().StringVar(&templateName, "name", "", "Name for the template")
	templateCreateCmd.Flags().StringVar(&templateRole, "role", "validator", "Agent role (validator or producer)")
	templateCreateCmd.Flags().StringVar(&templateTraits, "traits", "", "Comma-separated list of traits")
	templateCreateCmd.Flags().StringVar(&templateStyle, "style", "balanced", "Agent style")
	templateCreateCmd.Flags().StringVar(&templateDescription, "description", "", "Template description")
	
	templateCreateCmd.MarkFlagRequired("name")
	templateCreateCmd.MarkFlagRequired("traits")
	
	// Add flags for template show command
	templateShowCmd.Flags().StringVar(&templateName, "name", "", "Name of the template to show")
	templateShowCmd.MarkFlagRequired("name")
	
	// Create default templates
	registry := templates.NewTemplateRegistry()
	registry.CreateDefaultTemplates()
}

// createTemplate creates a new agent template
func createTemplate() {
	if templateName == "" {
		fmt.Println("Error: template name is required")
		os.Exit(1)
	}
	
	if templateTraits == "" {
		fmt.Println("Error: traits are required")
		os.Exit(1)
	}
	
	template := &templates.AgentTemplate{
		Name:        templateName,
		Role:        templateRole,
		Traits:      strings.Split(templateTraits, ","),
		Style:       templateStyle,
		Description: templateDescription,
	}
	
	registry := templates.NewTemplateRegistry()
	if err := registry.SaveTemplate(templateName, template); err != nil {
		fmt.Printf("Error saving template: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Template '%s' created successfully!\n", templateName)
}

// listTemplates lists all available templates
func listTemplates() {
	registry := templates.NewTemplateRegistry()
	templateNames, err := registry.ListTemplates()
	if err != nil {
		fmt.Printf("Error listing templates: %v\n", err)
		os.Exit(1)
	}
	
	if len(templateNames) == 0 {
		fmt.Println("No templates found.")
		return
	}
	
	fmt.Println("Available templates:")
	for _, name := range templateNames {
		template, err := registry.GetTemplate(name)
		if err != nil {
			continue
		}
		
		fmt.Printf("- %s (%s): %s\n", name, template.Role, template.Description)
	}
}

// showTemplate shows details of a specific template
func showTemplate() {
	registry := templates.NewTemplateRegistry()
	template, err := registry.GetTemplate(templateName)
	if err != nil {
		fmt.Printf("Error: template '%s' not found\n", templateName)
		os.Exit(1)
	}
	
	fmt.Printf("Template: %s\n", templateName)
	fmt.Printf("Name: %s\n", template.Name)
	fmt.Printf("Role: %s\n", template.Role)
	fmt.Printf("Traits: %s\n", strings.Join(template.Traits, ", "))
	fmt.Printf("Style: %s\n", template.Style)
	
	if len(template.Influences) > 0 {
		fmt.Printf("Influences: %s\n", strings.Join(template.Influences, ", "))
	}
	
	if template.Description != "" {
		fmt.Printf("Description: %s\n", template.Description)
	}
} 