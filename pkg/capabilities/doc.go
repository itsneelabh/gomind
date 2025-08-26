// Package capabilities provides metadata management and discovery mechanisms for agent capabilities
// in the GoMind Agent Framework.
//
// This package implements a dual metadata system that combines reflection-based discovery with
// rich metadata annotations, enabling both human and AI agents to understand and utilize
// agent capabilities effectively.
//
// # Metadata System
//
// The framework supports two complementary metadata approaches:
//
// 1. Comment-based annotations in source code:
//
//	// @capability: market_analysis
//	// @description: Analyzes market trends and provides insights
//	// @input: market_data string "Historical market data"
//	// @output: analysis object "Market analysis report"
//	func (a *Agent) AnalyzeMarket(data string) Analysis { }
//
// 2. YAML-based metadata files for richer descriptions:
//
//	capabilities:
//	  - name: market_analysis
//	    description: Advanced market trend analysis
//	    business_value:
//	      - "Risk assessment"
//	      - "Investment decisions"
//	    complexity: medium
//	    latency: "100-500ms"
//
// # Capability Metadata Structure
//
// The CapabilityMetadata struct contains comprehensive information about each capability:
//   - Core identity (name, description, domain)
//   - Performance characteristics (latency, complexity, cost)
//   - Business context (use cases, business value, impact)
//   - Technical requirements (prerequisites, dependencies, resource needs)
//   - LLM-friendly fields for AI agent communication
//
// # Discovery Process
//
// The discovery process follows this hierarchy:
// 1. Check for embedded metadata (compile-time embedded)
// 2. Load YAML metadata files from agent directory
// 3. Extract metadata from source code comments
// 4. Fall back to reflection-based discovery
//
// Metadata from different sources is intelligently merged, with more specific
// sources taking precedence over generic ones.
//
// # AI Agent Integration
//
// Special fields optimize capabilities for AI agent consumption:
//   - llm_prompt: Natural language description for LLMs
//   - specialties: Keywords for semantic matching
//   - automation_level: Indicates autonomous operation support
//   - risk_profile: Safety and compliance considerations
//
// # Usage
//
// The framework automatically discovers and manages capabilities during agent
// initialization. Developers can access capability metadata through the agent's
// GetCapabilities() method for runtime introspection and dynamic invocation.
package capabilities