# Financial Intelligence Multi-Agent System

# Financial Intelligence Multi-Agent System

A comprehensive demonstration of the GoMind Agent Framework's **autonomous agent discovery and LLM-powered communication** through a real-world financial intelligence system. This example provides **interactive UI and detailed logging** to prove that agents are making autonomous decisions and communicating intelligently.

## 🎯 System Overview

This multi-agent system demonstrates the framework's core auto-discovery features with **full transparency and interaction**:

- **Autonomous Agent Discovery**: Agents automatically find and register with each other using Redis service registry
- **LLM-Assisted Routing**: Natural language queries are intelligently routed using OpenAI with full decision logging
- **Interactive Web Dashboard**: Real-time UI to test agent communication and view decision processes
- **Comprehensive Logging**: Detailed audit trail of all LLM decisions and agent communications
- **Dynamic Capability Matching**: Real-time discovery of agent capabilities with confidence scoring
- **Resilient Coordination**: Fault-tolerant multi-agent communication with fallback mechanisms
- **Production-Ready Infrastructure**: Full Kubernetes deployment with monitoring and analytics

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Chat UI       │    │ Portfolio       │    │ Technical       │
│   Agent         │    │ Advisor Agent   │    │ Analysis Agent  │
│                 │    │                 │    │                 │
│ LLM Prompt:     │    │ LLM Prompt:     │    │ LLM Prompt:     │
│ "I help users   │    │ "I provide      │    │ "I analyze      │
│ ask financial   │    │ investment      │    │ chart patterns  │
│ questions"      │    │ advice"         │    │ and indicators" │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Framework       │
                    │ Auto-Discovery  │
                    │ (Redis + LLM)   │
                    └─────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Market Data     │    │ News Analysis   │    │ Redis Discovery │
│ Agent           │    │ Agent           │    │ Service         │
│                 │    │                 │    │                 │
│ LLM Prompt:     │    │ LLM Prompt:     │    │ Stores agent    │
│ "Ask me for     │    │ "I analyze      │    │ capabilities    │
│ stock prices"   │    │ financial news" │    │ and metadata    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🚀 **Quick Start & Interactive Testing**

### **Prerequisites**
- Kind cluster running
- OpenAI API key (for LLM-assisted routing)
- Alpha Vantage API key (for real market data)
- News API key (for sentiment analysis)
- Docker and kubectl installed

### **1. One-Command Deployment**
```bash
# Clone and deploy the complete system
./deploy.sh

# The script will:
# ✅ Create Kind cluster with ingress
# ✅ Build all 5 agent Docker images  
# ✅ Deploy Redis service discovery
# ✅ Deploy all agents with health checks
# ✅ Configure ingress and load balancers
# ✅ Update /etc/hosts for local access
```

### **2. Interactive Testing**
```bash
# Run comprehensive test suite
./test.sh

# The test suite validates:
# ✅ Agent auto-discovery functionality
# ✅ LLM-assisted query routing
# ✅ Multi-agent coordination
# ✅ Redis service registry
# ✅ Performance and load handling
# ✅ Decision audit trails
```

### **3. Access Interactive Dashboard**
```bash
# Main chat interface with live decision tracking
open http://financial-intelligence.local/chat

# Agent discovery monitor
open http://financial-intelligence.local/dashboard/discovery

# Decision audit trail viewer  
open http://financial-intelligence.local/dashboard/audit

# Network topology and performance monitor
open http://financial-intelligence.local/dashboard/topology
```

### **4. Test Autonomous Decision Making**

#### **Ready-to-Use Test Queries**
```bash
# Copy and paste these into the chat interface:

# Simple routing test
"What is the current price of Apple stock?"

# Multi-agent coordination test  
"Analyze Tesla stock including price, news sentiment, and technical indicators"

# Complex portfolio test
"I have $50,000 to invest in tech stocks. Give me analysis of AAPL, GOOGL, and MSFT with portfolio recommendations"

# Anomaly detection test
"Something unusual is happening with Amazon stock today. Investigate what's going on"

# Portfolio rebalancing test
"My portfolio is 40% AAPL, 35% MSFT, 25% GOOGL. Should I rebalance given current market conditions?"
```

#### **Evidence Collection Points**
- **LLM Decision Logs**: See exactly how queries are analyzed and routed
- **Agent Discovery Traces**: Watch real-time agent finding and selection
- **Communication Audit**: Complete trail of inter-agent messages
- **Performance Metrics**: Response times, success rates, confidence scores

## 🤖 **Agent Specifications & Autonomous Capabilities**

### **Market Data Agent** (Port 8080)
- **LLM Prompt**: `"I provide real-time stock prices, market data, and historical charts. Ask me about stock prices, market indices, or trading volumes."`
- **Specialties**: `["real-time-quotes", "NYSE", "NASDAQ", "market-indices", "after-hours-trading", "historical-data"]`
- **Autonomous Capabilities**:
  - Self-registers with Redis discovery service
  - Automatically handles Alpha Vantage API rate limiting
  - Provides real-time market data with sub-second latency
  - Auto-validates stock symbols and provides corrections
- **API Integration**: Alpha Vantage for live market data
- **Evidence Logging**: API call traces, response caching, error handling

### **News Analysis Agent** (Port 8081)
- **LLM Prompt**: `"I analyze financial news and provide sentiment scores. Ask me about news sentiment, market headlines, or earnings impact analysis."`
- **Specialties**: `["sentiment-analysis", "financial-news", "earnings-reports", "market-catalysts", "economic-indicators"]`
- **Autonomous Capabilities**:
  - Intelligent keyword extraction and relevance scoring
  - Multi-source news aggregation with bias detection
  - Real-time sentiment scoring with confidence intervals
  - Automatic event impact assessment
- **API Integration**: News API for financial news feeds
- **Evidence Logging**: Sentiment calculation algorithms, source reliability scores

### **Technical Analysis Agent** (Port 8084)
- **LLM Prompt**: `"I perform technical analysis on stock charts and indicators. Ask me about RSI, MACD, chart patterns, or trading signals."`
- **Specialties**: `["technical-analysis", "chart-patterns", "trading-signals", "price-analysis", "volume-analysis", "indicators", "resistance-support"]`
- **Autonomous Capabilities**:
  - Real-time technical indicator calculations
  - Pattern recognition with confidence scoring
  - Adaptive timeframe analysis based on query context
  - Risk-adjusted signal generation
- **Advanced Features**: RSI, MACD, Moving Averages, Pattern Recognition, Support/Resistance
- **Evidence Logging**: Calculation methodologies, pattern matching algorithms, signal confidence

### **Portfolio Advisor Agent** (Port 8085)
- **LLM Prompt**: `"I provide investment advice and portfolio optimization. Ask me about asset allocation, risk assessment, or portfolio rebalancing strategies."`
- **Specialties**: `["portfolio-management", "asset-allocation", "risk-assessment", "investment-strategy", "diversification", "rebalancing"]`
- **Autonomous Capabilities**:
  - Multi-agent data synthesis for comprehensive analysis
  - Risk-adjusted portfolio optimization
  - Dynamic allocation strategies based on market conditions
  - Personalized advice based on risk tolerance and goals
- **Advanced Features**: Modern Portfolio Theory, Risk Parity, Factor Analysis
- **Evidence Logging**: Optimization algorithms, risk calculations, recommendation reasoning

### **Chat UI Agent** (Port 8082) - **The Autonomous Orchestrator**
- **LLM Prompt**: `"I help users ask financial questions and intelligently route them to specialized agents. I understand natural language and coordinate complex multi-agent responses."`
- **Specialties**: `["natural-language-processing", "query-routing", "agent-coordination", "response-synthesis", "user-interaction"]`
- **Autonomous Capabilities**:
  - **Advanced LLM Integration**: Uses OpenAI for sophisticated query analysis and routing decisions
  - **Dynamic Agent Discovery**: Real-time capability matching using Redis registry
  - **Intelligent Query Decomposition**: Breaks complex queries into agent-specific subtasks
  - **Multi-Agent Coordination**: Orchestrates parallel agent execution with dependency management
  - **Conflict Resolution**: Handles disagreements between agents with weighted decision making
  - **Learning Adaptation**: Improves routing decisions based on success rates and user feedback
- **Evidence Logging**: Complete LLM conversation logs, routing decision trees, coordination workflows

## 🔍 **Proof of Autonomous Agent Communication**

### **Interactive Testing Dashboard**

The system includes a **comprehensive web dashboard** that provides real-time visibility into autonomous agent decision-making:

#### **1. Real-time Chat Interface**
- **Natural Language Queries**: Ask complex financial questions in plain English
- **Live Decision Tracking**: See how the LLM routes your query to appropriate agents
- **Agent Response Visualization**: Watch multiple agents collaborate on complex requests
- **Typing Indicators**: Real-time feedback as agents process requests

#### **2. Agent Discovery Visualization**
- **Live Agent Registry**: Real-time view of all discovered agents and their capabilities
- **Capability Mapping**: Visual representation of which agents can handle which requests
- **Health Monitoring**: Agent status, response times, and availability metrics
- **Network Topology**: Interactive diagram of agent relationships and communication flows

#### **3. Decision Audit Dashboard**
- **LLM Decision Logs**: Complete transparency into routing decisions with reasoning
- **Agent Communication Traces**: Full audit trail of inter-agent messages
- **Confidence Scoring**: See how confident the system is in its routing decisions
- **Performance Analytics**: Success rates, response times, and system efficiency metrics

### **Autonomous Decision Making Evidence**

The system captures and displays comprehensive evidence of autonomous behavior:

#### **LLM Routing Decisions**
```json
{
  "timestamp": "2025-08-15T10:30:00Z",
  "event": "llm_routing_decision",
  "user_query": "What is AAPL trading at and should I buy it?",
  "llm_prompt": "Analyze this query and determine which agents to route to...",
  "llm_response": "This requires market data for price and technical analysis for buy recommendation...",
  "confidence": 0.95,
  "selected_agents": ["market-data-agent", "technical-analysis-agent"],
  "reasoning": "Query contains price request and investment decision components"
}
```

#### **Agent Discovery Process**
```json
{
  "timestamp": "2025-08-15T10:30:01Z",
  "event": "agent_discovery",
  "requested_capability": "get-stock-price",
  "discovery_method": "redis_capability_search",
  "discovered_agents": [
    {
      "id": "market-data-agent-1",
      "capabilities": ["get-stock-price", "get-market-overview", "get-historical-data"],
      "status": "healthy",
      "response_time_avg": "150ms",
      "specialties": ["real-time-quotes", "NYSE", "NASDAQ"]
    }
  ],
  "selection_criteria": "healthy_status_and_best_response_time"
}
```

#### **Inter-Agent Communication**
```json
{
  "timestamp": "2025-08-15T10:30:02Z",
  "event": "agent_communication",
  "from": "chat-ui-agent",
  "to": "market-data-agent",
  "request": {
    "capability": "GetStockPrice",
    "input": {"symbol": "AAPL"}
  },
  "response_time": "200ms",
  "status": "success",
  "response_preview": "AAPL: $175.50 (+2.3%)"
}
```

### **Demonstration Scenarios**

#### **Scenario 1: Investment Research Assistant**
**User Query**: *"I'm thinking of investing $10,000 in tech stocks. Give me a complete analysis of AAPL including current price, recent news sentiment, technical indicators, and how it would fit in a balanced portfolio."*

**Autonomous Flow Evidence**:
1. **LLM Analysis**: Breaks down query into 4 distinct capability requirements
2. **Agent Discovery**: Finds Market Data, News Analysis, Technical Analysis, and Portfolio Advisor agents
3. **Parallel Coordination**: Simultaneously requests data from multiple agents
4. **Intelligent Synthesis**: Combines responses into comprehensive investment advice
5. **Decision Logging**: Full audit trail of every decision and communication

#### **Scenario 2: Market Anomaly Detection**
**User Query**: *"Something seems weird with Tesla stock today. Can you investigate what's happening?"*

**Autonomous Flow Evidence**:
1. **Intent Recognition**: LLM identifies "weird" as anomaly detection requirement
2. **Multi-Agent Strategy**: Routes to Market Data for price movements, News for breaking news, Technical Analysis for patterns
3. **Correlation Analysis**: Automatically correlates findings across agents
4. **Anomaly Reasoning**: Provides evidence-based explanation of unusual market behavior

#### **Scenario 3: Portfolio Rebalancing Assistant**
**User Query**: *"My portfolio is 60% AAPL, 30% GOOGL, 10% TSLA. The market has been volatile lately. Should I rebalance?"*

**Autonomous Flow Evidence**:
1. **Portfolio Understanding**: Extracts current allocation from natural language
2. **Market Context**: Automatically gathers current prices and volatility data
3. **Risk Assessment**: Combines technical analysis with news sentiment
4. **Rebalancing Strategy**: Provides specific recommendations with reasoning

### **Real-time Monitoring Features**

#### **Agent Health Dashboard**
- Live status of all 5 specialized agents
- Response time monitoring and alerting
- Capability availability tracking
- Load balancing metrics

#### **Discovery Service Analytics**
- Redis connection status and performance
- Agent registration/deregistration events
- Capability query success rates
- Network partition recovery

#### **Query Analytics**
- Most popular query types and routing patterns
- Success rates by query complexity
- User satisfaction metrics
- System learning and improvement trends

## 🔍 Auto-Discovery Demonstration

### Example User Queries with Autonomous Decision Proof

#### **1. Simple Market Data Query**
**User**: *"What's the current price of Apple stock?"*

**Autonomous Decision Process**:
```
[LLM Analysis] → "This is a direct price query for AAPL"
[Agent Discovery] → Searches for "get-stock-price" capability
[Route Selection] → market-data-agent (confidence: 0.98)
[Execution] → GetStockPrice(symbol: "AAPL")
[Response] → "AAPL: $175.50 (+2.3%, +$3.95)"
```

**Logged Evidence**:
- LLM prompt and reasoning for routing decision
- Agent discovery query and results
- Direct API call to Alpha Vantage
- Response time and success metrics

#### **2. Complex Multi-Agent Query**
**User**: *"Is there any news affecting Tesla today, and what do the technical indicators suggest?"*

**Autonomous Decision Process**:
```
[LLM Analysis] → "Requires news sentiment + technical analysis for TSLA"
[Agent Discovery] → Finds news-analysis-agent + technical-analysis-agent
[Parallel Execution] → 
  ├── News Agent: AnalyzeFinancialNews(symbol: "TSLA")
  └── Technical Agent: CalculateTechnicalIndicators(symbol: "TSLA")
[Synthesis] → Correlates news sentiment with technical signals
[Response] → Combined analysis with conflict resolution
```

**Logged Evidence**:
- Multi-agent coordination decision tree
- Parallel execution timing and synchronization
- Cross-agent data correlation logic
- Confidence scoring for combined recommendations

#### **3. Investment Advisory Query**
**User**: *"Should I buy Amazon stock based on technical analysis and current market conditions?"*

**Autonomous Decision Process**:
```
[LLM Analysis] → "Investment decision requiring multiple data sources"
[Agent Coordination] → 
  ├── Market Data: Current AMZN price and volume
  ├── News Analysis: Recent sentiment and catalysts  
  ├── Technical Analysis: RSI, MACD, support/resistance
  └── Portfolio Advisor: Investment recommendation synthesis
[Decision Synthesis] → Weighted recommendation with risk factors
[Response] → "Based on analysis... BUY with 0.85 confidence"
```

**Logged Evidence**:
- Complete agent coordination workflow
- Data dependency resolution between agents
- Investment decision reasoning with supporting evidence
- Risk assessment and confidence intervals

#### **4. Portfolio Management Query**
**User**: *"Give me a complete analysis of Microsoft for my $50K tech portfolio"*

**Autonomous Decision Process**:
```
[LLM Analysis] → "Comprehensive analysis requiring all agent types"
[Discovery Phase] → Validates all 4 specialist agents are available
[Execution Strategy] → 
  ├── Phase 1: Market Data (price, volume, historical)
  ├── Phase 2: News Analysis (sentiment, recent events)
  ├── Phase 3: Technical Analysis (indicators, patterns)
  └── Phase 4: Portfolio Integration (allocation advice)
[Synthesis] → Creates comprehensive investment profile
[Response] → "MSFT Analysis Report with 23 data points"
```

**Logged Evidence**:
- Multi-phase execution planning
- Data flow between agents with dependency management
- Comprehensive analysis synthesis algorithm
- Portfolio fit assessment with specific allocation advice

### **Live Dashboard Access Points**

#### **Real-time Chat Interface**
- **URL**: `http://financial-intelligence.local/chat`
- **Features**: Live agent discovery, decision tracking, response synthesis
- **Evidence**: Real-time logs of LLM decisions and agent communications

#### **Agent Discovery Monitor**
- **URL**: `http://financial-intelligence.local/dashboard/discovery`
- **Features**: Live agent registry, capability mapping, health status
- **Evidence**: Agent registration events, capability queries, load balancing

#### **Decision Audit Trail**
- **URL**: `http://financial-intelligence.local/dashboard/audit`
- **Features**: Complete decision history, confidence scores, performance metrics
- **Evidence**: LLM prompts/responses, routing decisions, success rates

- **Network Topology Viewer**
- **URL**: `http://financial-intelligence.local/dashboard/topology`
- **Features**: Visual agent relationships, communication flows, bottleneck detection
- **Evidence**: Inter-agent message flows, latency analysis, failure recovery

## 🏗️ **Production-Ready Infrastructure**

### **Kubernetes Deployment Architecture**
```
financial-intelligence namespace
├── Redis Cluster (Service Discovery)
│   ├── Agent registry and capability index
│   ├── Health monitoring and heartbeats  
│   ├── Load balancing and failover
│   └── Performance metrics collection
├── Agent Deployments (2+ replicas each)
│   ├── market-data-agent (2 replicas)
│   ├── news-analysis-agent (2 replicas) 
│   ├── chat-ui-agent (2 replicas)
│   ├── technical-analysis-agent (1 replica)
│   └── portfolio-advisor-agent (1 replica)
├── Ingress Controller (NGINX)
│   ├── External access routing
│   ├── Load balancing across replicas
│   ├── SSL termination ready
│   └── Rate limiting and security
└── Monitoring & Logging
    ├── Agent health dashboards
    ├── Performance metrics
    ├── Decision audit trails
    └── Real-time log streaming
```

### **Auto-Discovery Infrastructure**
- **Redis Service Registry**: Central registry for agent capabilities and health
- **Health Check System**: Automatic agent health monitoring with failover
- **Load Balancing**: Intelligent routing to least-loaded healthy agents
- **Dynamic Scaling**: Agents can be scaled up/down without service interruption
- **Network Resilience**: Automatic reconnection and partition recovery

## 🧪 **Testing & Validation Framework**

### **Automated Test Scenarios**
```bash
# Comprehensive test suite execution
./test.sh

# Individual component testing
./test.sh --component market-data
./test.sh --component llm-routing  
./test.sh --component multi-agent-coordination
./test.sh --component discovery-service
./test.sh --component performance-benchmarks
```

### **Performance Benchmarks**
- **Query Response Time**: < 2 seconds for simple queries, < 5 seconds for complex
- **Agent Discovery Time**: < 100ms for capability lookups
- **LLM Decision Time**: < 500ms for routing decisions
- **System Availability**: 99.9% uptime with automatic recovery
- **Concurrent Users**: Support for 100+ simultaneous users

---

## 🎯 **Getting Started - Your Journey to Autonomous Agents**

### **Step 1: Deploy the System**
```bash
git clone <repository>
cd financial-intelligence-system
./deploy.sh
```

### **Step 2: Open the Interactive Dashboard**
```bash
open http://financial-intelligence.local/chat
```

### **Step 3: Test Autonomous Decision Making**
Try these queries to see the autonomous agent communication in action:

1. **"What is AAPL trading at?"** - Watch simple routing decisions
2. **"Analyze TSLA including news and technical indicators"** - See multi-agent coordination
3. **"Should I invest $10K in tech stocks?"** - Experience complex decision synthesis

### **Step 4: Examine the Evidence**
- **Dashboard Logs**: See real-time LLM decisions and agent discoveries
- **Audit Trail**: Review complete communication traces
- **Performance Metrics**: Monitor system efficiency and accuracy

### **Step 5: Extend the System**
- Add new agents with custom capabilities
- Enhance LLM prompts for better routing
- Integrate additional external APIs
- Scale to production workloads

---

**🚀 Start exploring autonomous agent communication now!**

**Ready to see intelligent agents working together? Deploy the system and watch as your queries automatically discover the right agents, coordinate complex responses, and provide intelligent financial insights - all with complete transparency into the decision-making process.**

## 📊 Observability

### OpenTelemetry Tracing
- Automatic instrumentation of all agent capabilities
- LLM-specific trace attributes (`ai.capability.llm_prompt`, `ai.capability.specialties`)
- Cross-agent request tracing
- Performance metrics for each capability

### Metrics Available
- `capability_executions_total` - Capability usage counters
- `discovery_queries_total` - Agent discovery requests  
- `llm_routing_decisions_total` - LLM routing decisions
- `agent_response_time_seconds` - Response time histograms

### Health Checks
- `/health` - Agent health status
- `/capabilities` - Available agent capabilities
- `/discovery` - Discovery service status

## 🔧 Configuration

### Environment Variables
```bash
# API Keys
OPENAI_API_KEY=your_openai_key
ALPHA_VANTAGE_API_KEY=your_alpha_vantage_key
NEWS_API_KEY=your_news_api_key

# Framework Configuration
REDIS_URL=redis://redis:6379
DISCOVERY_ENABLED=true
LLM_ROUTING_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:14268/api/traces
```

### Agent Metadata
Each agent uses the framework's dual metadata system:
- **Go Comments**: `@llm_prompt`, `@specialties`, `@domain`
- **YAML Config**: Business impact, quality metrics, resource requirements
- **Auto-discovery**: Reflection-based capability detection

## 🧪 Testing Scenarios

### 1. Basic Auto-Discovery
```bash
curl http://localhost:8080/api/chat \
  -d '{"message": "What is the price of AAPL?"}'
```

### 2. Multi-Agent Coordination
```bash
curl http://localhost:8080/api/chat \
  -d '{"message": "Should I invest in TSLA? Give me a complete analysis."}'
```

### 3. Agent Catalog Generation
```bash
curl http://localhost:8080/api/agents
# Returns LLM-optimized agent directory
```

## 📁 Project Structure

```
financial-intelligence-system/
├── agents/
│   ├── market-data/          # Alpha Vantage integration
│   ├── news-analysis/        # News API + sentiment analysis
│   ├── technical-analysis/   # Chart patterns & indicators
│   ├── portfolio-advisor/    # Investment recommendations
│   └── chat-ui/             # Web interface + LLM routing
├── k8s/
│   ├── redis/               # Redis deployment
│   ├── agents/              # Agent deployments
│   └── ingress/             # Load balancer config
├── docker/
│   └── Dockerfile.agent     # Multi-stage agent builds
├── config/
│   ├── capabilities.yaml   # Agent capability definitions
│   └── environment/        # Environment-specific configs
└── scripts/
    ├── build.sh            # Build all agents
    ├── deploy.sh           # Deploy to Kind
    └── test.sh             # Integration tests
```

This system demonstrates:
- ✅ **Zero-config agent discovery** via framework auto-registration
- ✅ **LLM-assisted routing** without hardcoded agent mappings  
- ✅ **Real-world API integration** with external data sources
- ✅ **Natural language interface** for non-technical users
- ✅ **Production deployment** on Kubernetes with observability
- ✅ **Cross-agent coordination** for complex financial analysis
