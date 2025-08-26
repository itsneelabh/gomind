package capabilities

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CommentParser extracts metadata from Go comments in agent source files
type CommentParser struct {
	fileSet *token.FileSet
}

// NewCommentParser creates a new comment parser
func NewCommentParser() *CommentParser {
	return &CommentParser{
		fileSet: token.NewFileSet(),
	}
}

// ExtractFromComments parses Go source files and extracts capability metadata from comments
func (p *CommentParser) ExtractFromComments(agentType reflect.Type) ([]CapabilityMetadata, error) {
	var capabilities []CapabilityMetadata

	// First, try to load embedded metadata (for containerized/production environments)
	if embeddedCapabilities := p.loadEmbeddedMetadata(agentType); len(embeddedCapabilities) > 0 {
		return embeddedCapabilities, nil
	}

	// Get the package path to find source files
	pkgPath := agentType.PkgPath()
	if pkgPath == "" {
		// If no package path, return empty capabilities instead of error
		// This allows the framework to continue with other metadata sources
		return capabilities, nil
	}

	// Try multiple approaches to find source files
	var goFiles []string

	// Approach 1: Look in current working directory
	currentDirFiles, err1 := filepath.Glob("*.go")
	if err1 == nil && len(currentDirFiles) > 0 {
		goFiles = append(goFiles, currentDirFiles...)
	}

	// Approach 2: Look for agent source files in typical locations
	// Common patterns for Go agent files - updated to match actual project structure
	possiblePaths := []string{
		"financial-advisory-system/agents/*/main.go", // Main agent files
		"agents/*/main.go",                           // Alternative agent location
		"cmd/*/main.go",                              // Command-based agents
		"internal/agent/*.go",                        // Framework internal
		"pkg/agent/*.go",                             // Package-based agents
		"agent/*.go",                                 // Root-level agents
		"**/agent.go",                                // Named agent files
	}

	for _, pattern := range possiblePaths {
		if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
			goFiles = append(goFiles, matches...)
		}
	}

	// If no Go files found, return empty capabilities instead of error
	// This allows the framework to gracefully fall back to other metadata sources
	if len(goFiles) == 0 {
		return capabilities, nil
	}

	// Process found files
	for _, filename := range goFiles {
		fileCapabilities, err := p.parseFileComments(filename, agentType)
		if err != nil {
			// Log error but continue processing other files
			continue
		}
		capabilities = append(capabilities, fileCapabilities...)
	}

	return capabilities, nil
}

// loadEmbeddedMetadata attempts to load pre-built metadata for containerized environments
func (p *CommentParser) loadEmbeddedMetadata(agentType reflect.Type) []CapabilityMetadata {
	log.Printf("[DEBUG] loadEmbeddedMetadata called for agent type: %s", agentType.Name())
	// Try to load from embedded metadata
	capabilities := GetEmbeddedCapabilities(agentType)
	log.Printf("[DEBUG] loadEmbeddedMetadata returning %d capabilities", len(capabilities))
	return capabilities
}

// parseFileComments parses a single Go file and extracts capability metadata from comments
func (p *CommentParser) parseFileComments(filename string, agentType reflect.Type) ([]CapabilityMetadata, error) {
	var capabilities []CapabilityMetadata

	src, err := os.ReadFile(filename)
	if err != nil {
		return capabilities, err
	}

	file, err := parser.ParseFile(p.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return capabilities, err
	}

	// Look for methods of the agent type
	ast.Inspect(file, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if p.isAgentMethod(funcDecl, agentType) {
				if metadata := p.extractCapabilityFromFunc(funcDecl, filename); metadata != nil {
					capabilities = append(capabilities, *metadata)
				}
			}
		}
		return true
	})

	return capabilities, nil
}

// isAgentMethod checks if a function declaration is a method of the agent type
func (p *CommentParser) isAgentMethod(funcDecl *ast.FuncDecl, agentType reflect.Type) bool {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return false
	}

	// This is a simplified check - in practice, you'd want more robust type checking
	return strings.Contains(funcDecl.Name.Name, "Handle") ||
		strings.Contains(funcDecl.Name.Name, "Process") ||
		strings.Contains(funcDecl.Name.Name, "Execute")
}

// extractCapabilityFromFunc extracts capability metadata from function comments
func (p *CommentParser) extractCapabilityFromFunc(funcDecl *ast.FuncDecl, filename string) *CapabilityMetadata {
	if funcDecl.Doc == nil {
		return nil
	}

	metadata := &CapabilityMetadata{
		Name: funcDecl.Name.Name,
		Source: &MetadataSource{
			Type:        "comment",
			File:        filename,
			Line:        p.fileSet.Position(funcDecl.Pos()).Line,
			LastUpdated: time.Now().Format(time.RFC3339),
		},
	}

	commentText := funcDecl.Doc.Text()
	p.parseCommentAnnotations(commentText, metadata)

	return metadata
}

// parseCommentAnnotations parses structured annotations from comments
func (p *CommentParser) parseCommentAnnotations(commentText string, metadata *CapabilityMetadata) {
	lines := strings.Split(commentText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse @annotation: value patterns
		if strings.HasPrefix(line, "@") {
			p.parseAnnotationLine(line, metadata)
		} else if metadata.Description == "" {
			// First non-annotation line becomes description
			metadata.Description = line
		}
	}
}

// parseAnnotationLine parses a single annotation line
func (p *CommentParser) parseAnnotationLine(line string, metadata *CapabilityMetadata) {
	// Remove @ prefix
	line = strings.TrimPrefix(line, "@")

	// Split on first colon
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch key {
	case "domain":
		metadata.Domain = value
	case "complexity":
		metadata.Complexity = value
	case "latency":
		metadata.Latency = value
	case "cost":
		metadata.Cost = value
	case "confidence":
		if conf, err := strconv.ParseFloat(value, 64); err == nil {
			metadata.ConfidenceLevel = conf
		}
	case "business_value":
		metadata.BusinessValue = strings.Split(value, ",")
		for i := range metadata.BusinessValue {
			metadata.BusinessValue[i] = strings.TrimSpace(metadata.BusinessValue[i])
		}
	case "llm_prompt":
		metadata.LLMPrompt = value
	case "specialties":
		metadata.Specialties = strings.Split(value, ",")
		for i := range metadata.Specialties {
			metadata.Specialties[i] = strings.TrimSpace(metadata.Specialties[i])
		}
	case "use_cases":
		metadata.UseCases = strings.Split(value, ",")
		for i := range metadata.UseCases {
			metadata.UseCases[i] = strings.TrimSpace(metadata.UseCases[i])
		}
	case "automation_level":
		metadata.AutomationLevel = value
	case "input_types":
		metadata.InputTypes = strings.Split(value, ",")
		for i := range metadata.InputTypes {
			metadata.InputTypes[i] = strings.TrimSpace(metadata.InputTypes[i])
		}
	case "output_formats":
		metadata.OutputFormats = strings.Split(value, ",")
		for i := range metadata.OutputFormats {
			metadata.OutputFormats[i] = strings.TrimSpace(metadata.OutputFormats[i])
		}
	}
}

// YAMLLoader loads capability metadata from YAML configuration files
type YAMLLoader struct{}

// NewYAMLLoader creates a new YAML loader
func NewYAMLLoader() *YAMLLoader {
	return &YAMLLoader{}
}

// CapabilityConfig represents the YAML structure for capability configuration
type CapabilityConfig struct {
	Agent        AgentConfig          `yaml:"agent"`
	Capabilities []CapabilityMetadata `yaml:"capabilities"`
}

// AgentConfig represents agent-level configuration
type AgentConfig struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Tags        map[string]string `yaml:"tags"`
}

// LoadMetadata loads capability metadata from YAML files in the agent directory
func (l *YAMLLoader) LoadMetadata(agentDir string) ([]CapabilityMetadata, error) {
	var capabilities []CapabilityMetadata

	// Look for capability configuration files
	configFiles := []string{
		filepath.Join(agentDir, "capabilities.yaml"),
		filepath.Join(agentDir, "capabilities.yml"),
		filepath.Join(agentDir, "agent.yaml"),
		filepath.Join(agentDir, "agent.yml"),
	}

	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			fileCapabilities, err := l.loadFromFile(configFile)
			if err != nil {
				continue // Skip files with errors
			}
			capabilities = append(capabilities, fileCapabilities...)
		}
	}

	return capabilities, nil
}

// loadFromFile loads capabilities from a single YAML file
func (l *YAMLLoader) loadFromFile(filename string) ([]CapabilityMetadata, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config CapabilityConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Add source information to each capability
	for i := range config.Capabilities {
		config.Capabilities[i].Source = &MetadataSource{
			Type:        "yaml",
			File:        filename,
			LastUpdated: time.Now().Format(time.RFC3339),
		}
	}

	return config.Capabilities, nil
}

// MetadataMerger intelligently merges metadata from comments and YAML
type MetadataMerger struct {
	precedence map[string]string // field -> source type precedence
}

// NewMetadataMerger creates a new metadata merger with default precedence rules
func NewMetadataMerger() *MetadataMerger {
	return &MetadataMerger{
		precedence: map[string]string{
			// YAML takes precedence for business context
			"business_impact":       "yaml",
			"quality_metrics":       "yaml",
			"resource_requirements": "yaml",
			"risk_profile":          "yaml",

			// Comments take precedence for technical details
			"description":      "comment",
			"input_types":      "comment",
			"output_formats":   "comment",
			"automation_level": "comment",
		},
	}
}

// MergeMetadata merges capability metadata from multiple sources
func (m *MetadataMerger) MergeMetadata(commentMeta, yamlMeta []CapabilityMetadata) []CapabilityMetadata {
	// Create a map for quick lookup
	yamlMap := make(map[string]CapabilityMetadata)
	for _, meta := range yamlMeta {
		yamlMap[meta.Name] = meta
	}

	var merged []CapabilityMetadata

	// Start with comment metadata and enhance with YAML
	for _, commentCap := range commentMeta {
		if yamlCap, exists := yamlMap[commentCap.Name]; exists {
			mergedCap := m.mergeCapability(commentCap, yamlCap)
			merged = append(merged, mergedCap)
			delete(yamlMap, commentCap.Name) // Mark as processed
		} else {
			merged = append(merged, commentCap)
		}
	}

	// Add remaining YAML-only capabilities
	for _, yamlCap := range yamlMap {
		merged = append(merged, yamlCap)
	}

	return merged
}

// mergeCapability merges two capability metadata instances according to precedence rules
func (m *MetadataMerger) mergeCapability(comment, yaml CapabilityMetadata) CapabilityMetadata {
	merged := comment // Start with comment metadata

	// Apply YAML overrides based on precedence
	if m.shouldUseYAML("business_impact") && yaml.BusinessImpact != nil {
		merged.BusinessImpact = yaml.BusinessImpact
	}
	if m.shouldUseYAML("quality_metrics") && yaml.QualityMetrics != nil {
		merged.QualityMetrics = yaml.QualityMetrics
	}
	if m.shouldUseYAML("resource_requirements") && yaml.ResourceReqs != nil {
		merged.ResourceReqs = yaml.ResourceReqs
	}
	if m.shouldUseYAML("risk_profile") && yaml.RiskProfile != nil {
		merged.RiskProfile = yaml.RiskProfile
	}

	// Merge arrays intelligently
	merged.BusinessValue = m.mergeStringSlices(comment.BusinessValue, yaml.BusinessValue)
	merged.UseCases = m.mergeStringSlices(comment.UseCases, yaml.UseCases)
	merged.Prerequisites = m.mergeStringSlices(comment.Prerequisites, yaml.Prerequisites)
	merged.Dependencies = m.mergeStringSlices(comment.Dependencies, yaml.Dependencies)
	merged.Specialties = m.mergeStringSlices(comment.Specialties, yaml.Specialties)

	// LLM-specific fields - YAML takes precedence
	if yaml.LLMPrompt != "" {
		merged.LLMPrompt = yaml.LLMPrompt
	}

	// Set source to merged
	merged.Source = &MetadataSource{
		Type:        "merged",
		LastUpdated: time.Now().Format(time.RFC3339),
	}

	return merged
}

// shouldUseYAML checks if YAML should take precedence for a field
func (m *MetadataMerger) shouldUseYAML(field string) bool {
	return m.precedence[field] == "yaml"
}

// mergeStringSlices merges two string slices, removing duplicates
func (m *MetadataMerger) mergeStringSlices(slice1, slice2 []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice1 {
		if item != "" && !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}

	for _, item := range slice2 {
		if item != "" && !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}

	return result
}
