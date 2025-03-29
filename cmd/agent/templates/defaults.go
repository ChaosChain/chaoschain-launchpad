package templates

// DefaultTemplates returns a map of default template names to their definitions
func DefaultTemplates() map[string]*AgentTemplate {
	return map[string]*AgentTemplate{
		"chaotic_validator": {
			Name:        "Chaotic Validator",
			Role:        "validator",
			Traits:      []string{"unpredictable", "emotional", "dramatic"},
			Style:       "chaotic",
			Description: "A validator that introduces chaos and unpredictability into the consensus process.",
		},
		"conservative_validator": {
			Name:        "Conservative Validator",
			Role:        "validator",
			Traits:      []string{"honest", "diligent", "cautious"},
			Style:       "conservative",
			Influences:  []string{},
			Description: "A conservative validator that prioritizes security and stability over innovation.",
		},
		"innovative_producer": {
			Name:        "Innovative Producer",
			Role:        "producer",
			Traits:      []string{"creative", "risk-taking", "efficient"},
			Style:       "innovative",
			Description: "An innovative producer that prioritizes new transaction types and experimental features.",
		},
		"dramatic_validator": {
			Name:        "Dramatic Validator",
			Role:        "validator",
			Traits:      []string{"theatrical", "exaggerating", "passionate"},
			Style:       "dramatic",
			Description: "A validator that adds drama and excitement to the validation process.",
		},
		"skeptical_validator": {
			Name:        "Skeptical Validator",
			Role:        "validator",
			Traits:      []string{"questioning", "analytical", "cautious"},
			Style:       "skeptical",
			Description: "A validator that questions everything and requires strong evidence.",
		},
		"efficient_producer": {
			Name:        "Efficient Producer",
			Role:        "producer",
			Traits:      []string{"organized", "practical", "methodical"},
			Style:       "efficient",
			Description: "A producer focused on creating well-structured, efficient blocks.",
		},
	}
}

// UpdateCreateDefaultTemplates to use the DefaultTemplates function
func (r *TemplateRegistry) CreateDefaultTemplates() error {
	templates, err := r.ListTemplates()
	if err != nil {
		return err
	}
	
	// Only create default templates if no templates exist
	if len(templates) > 0 {
		return nil
	}
	
	// Create all default templates
	for name, template := range DefaultTemplates() {
		if err := r.SaveTemplate(name, template); err != nil {
			return err
		}
	}
	
	return nil
} 