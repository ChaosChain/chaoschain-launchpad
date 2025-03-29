package templates

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/NethermindEth/chaoschain-launchpad/core"
)

// AgentTemplate defines a template for creating new agents
type AgentTemplate struct {
	Name        string   `json:"name"`
	Role        string   `json:"role"` // "producer" or "validator"
	Traits      []string `json:"traits"`
	Style       string   `json:"style"`
	Influences  []string `json:"influences,omitempty"`
	Description string   `json:"description"`
}

// TemplateRegistry manages agent templates
type TemplateRegistry struct {
	templatesDir string
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() *TemplateRegistry {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	
	// Create .chaoschain/templates directory if it doesn't exist
	templatesDir := filepath.Join(homeDir, ".chaoschain", "templates")
	os.MkdirAll(templatesDir, 0755)
	
	return &TemplateRegistry{
		templatesDir: templatesDir,
	}
}

// SaveTemplate saves a template to the filesystem
func (r *TemplateRegistry) SaveTemplate(name string, template *AgentTemplate) error {
	templatePath := filepath.Join(r.templatesDir, name+".json")
	templateData, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(templatePath, templateData, 0644)
}

// GetTemplate loads a template from the filesystem
func (r *TemplateRegistry) GetTemplate(name string) (*AgentTemplate, error) {
	templatePath := filepath.Join(r.templatesDir, name+".json")
	templateData, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}
	
	var template AgentTemplate
	if err := json.Unmarshal(templateData, &template); err != nil {
		return nil, err
	}
	
	return &template, nil
}

// ListTemplates returns all available templates
func (r *TemplateRegistry) ListTemplates() ([]string, error) {
	var templates []string
	
	files, err := ioutil.ReadDir(r.templatesDir)
	if err != nil {
		return nil, err
	}
	
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			templates = append(templates, file.Name()[:len(file.Name())-5])
		}
	}
	
	return templates, nil
}

// Note: CreateDefaultTemplates is now implemented in defaults.go 

// ToAgentStruct converts a template to the core.Agent struct
func (t *AgentTemplate) ToAgentStruct() core.Agent {
	return core.Agent{
		Name:       t.Name,
		Role:       t.Role,
		Traits:     t.Traits,
		Style:      t.Style,
		Influences: t.Influences,
		Mood:       "neutral", // Default mood
	}
} 