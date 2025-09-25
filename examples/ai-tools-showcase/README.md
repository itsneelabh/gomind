# AI Tools Showcase Example

A comprehensive demonstration of the **4 built-in AI-powered tools** included in the GoMind framework. This example shows how to deploy and use production-ready AI tools that provide translation, summarization, sentiment analysis, and code review capabilities.

## 🎯 What This Example Demonstrates

### Built-in AI Tools

The GoMind framework includes **4 production-ready AI tools** that you can use immediately:

| Tool | Capability | Use Cases |
|------|------------|-----------|
| **🌐 Translation Tool** | Professional language translation | Internationalization, content localization, multilingual support |
| **📄 Summarization Tool** | Intelligent text summarization | Content processing, document analysis, key point extraction |
| **😊 Sentiment Analysis Tool** | Emotion and tone detection | Customer feedback, social media monitoring, content moderation |
| **🔍 Code Review Tool** | AI-powered code analysis | Code quality, security review, best practices, bug detection |

### Key Features

- **🚀 Zero Setup**: Tools work out-of-the-box with any AI provider
- **🔄 Multi-Provider**: Automatic detection of OpenAI, Groq, Anthropic, Gemini
- **🎯 Framework Integration**: Full GoMind framework support with discovery
- **📊 Production Ready**: Error handling, logging, monitoring, health checks
- **🌐 Web API**: RESTful endpoints with JSON request/response
- **🔗 Discoverable**: Other agents can find and use these tools

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                AI Tools Showcase                        │
├─────────────────────────────────────────────────────────┤
│ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────┐ │
│ │Translation  │ │Summarization│ │Sentiment    │ │Code  │ │
│ │Tool         │ │Tool         │ │Analysis Tool│ │Review│ │
│ │             │ │             │ │             │ │Tool  │ │
│ │🌐 Multi-lang│ │📄 Key Points│ │😊 Emotions  │ │🔍 QA │ │
│ │Context-aware│ │Smart Summary│ │Confidence   │ │Security│ │
│ └─────────────┘ └─────────────┘ └─────────────┘ └──────┘ │
├─────────────────────────────────────────────────────────┤
│               Universal AI Provider Layer               │
│          OpenAI • Groq • Claude • Gemini               │
├─────────────────────────────────────────────────────────┤
│                  GoMind Framework                       │
│    Discovery • Registry • Health • Capabilities        │
└─────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.25 or later  
- **AI Provider API Key (REQUIRED)** - Choose one:
  - `OPENAI_API_KEY` - OpenAI GPT models (recommended)
  - `GROQ_API_KEY` - Ultra-fast inference (free tier available)  
  - `ANTHROPIC_API_KEY` - Claude models
  - `DEEPSEEK_API_KEY` - Advanced reasoning
  - `GEMINI_API_KEY` - Google AI models
- Redis (optional, for service discovery)

### 1. Local Development

```bash
# Navigate to the example
cd examples/ai-tools-showcase

# Install dependencies  
go mod tidy

# Set up AI provider (REQUIRED - choose one)
export OPENAI_API_KEY="sk-your-openai-key"        # OpenAI (recommended)
export GROQ_API_KEY="gsk-your-groq-key"           # Ultra-fast (free tier)
export ANTHROPIC_API_KEY="sk-ant-your-key"        # Claude models

# Optional: Configure Redis for service discovery
export REDIS_URL="redis://localhost:6379"

# Run the AI tools showcase
go run main.go
```

The service starts with all 4 AI tools available:

```
🤖 Creating AI Tools with provider: OpenAI
✅ Translation Tool created successfully
✅ Summarization Tool created successfully  
✅ Sentiment Analysis Tool created successfully
✅ Code Review Tool created successfully
🚀 AI Tools Composite Service starting on port 8084...
```

### 2. Test the Tools

#### Translation Tool
```bash
curl -X POST http://localhost:8084/api/capabilities/demo_translation \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Hello, how are you today?",
    "target_language": "Spanish"
  }'
```

**Response:**
```json
{
  "tool": "ai_translation",
  "source_text": "Hello, how are you today?",
  "target_language": "Spanish", 
  "translated_text": "Hola, ¿cómo estás hoy?",
  "ai_powered": true,
  "demonstration": true
}
```

#### Summarization Tool
```bash
curl -X POST http://localhost:8084/api/capabilities/demo_summarization \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Artificial Intelligence has revolutionized modern technology by enabling machines to process vast amounts of data, identify patterns, and make predictions that would be impossible for humans to detect manually. Machine learning algorithms are being applied across healthcare, finance, and transportation to create more efficient systems.",
    "max_length": 50
  }'
```

**Response:**
```json
{
  "tool": "ai_summarization",
  "original_text": "Artificial Intelligence has revolutionized...",
  "original_length": 312,
  "summary": "AI revolutionizes technology through data processing, pattern recognition, and predictive capabilities across healthcare, finance, and transportation.",
  "summary_length": 138,
  "compression_ratio": 0.44,
  "ai_powered": true
}
```

#### Sentiment Analysis Tool
```bash
curl -X POST http://localhost:8084/api/capabilities/demo_sentiment \
  -H "Content-Type: application/json" \
  -d '{
    "text": "I absolutely love this framework! It makes AI integration so simple.",
    "detailed": true
  }'
```

**Response:**
```json
{
  "tool": "ai_sentiment_analysis",
  "analyzed_text": "I absolutely love this framework! It makes AI integration so simple.",
  "sentiment": "POSITIVE",
  "confidence": 0.94,
  "detailed_result": "POSITIVE sentiment with high confidence (94%). Expression shows enthusiasm and satisfaction with strong positive indicators.",
  "ai_powered": true
}
```

#### Code Review Tool
```bash
curl -X POST http://localhost:8084/api/capabilities/demo_code_review \
  -H "Content-Type: application/json" \
  -d '{
    "code": "func processUsers(users []User) {\n  for i := 0; i < len(users); i++ {\n    fmt.Println(users[i].name)\n  }\n}",
    "language": "Go",
    "focus": "performance"
  }'
```

**Response:**
```json
{
  "tool": "ai_code_review",
  "reviewed_code": "func processUsers(users []User) {...}",
  "language": "Go",
  "focus_area": "performance",
  "review_result": "Performance Issues: 1) Use range loop instead of index loop for better readability and performance. 2) Direct field access without validation. 3) Consider using structured logging. Suggested: for _, user := range users { log.Printf(\"User: %s\", user.Name) }",
  "lines_reviewed": 4,
  "ai_powered": true
}
```

### 3. Interactive Showcase

```bash
# Get overview of all tools
curl -X POST http://localhost:8084/ai-showcase

# Run specific demonstrations  
curl -X POST http://localhost:8084/ai-showcase \
  -d '{"demo": "translation"}'
  
curl -X POST http://localhost:8084/ai-showcase \
  -d '{"demo": "all"}'
```

## 📊 AI Tools Detailed Guide

### 1. 🌐 Translation Tool

**Purpose**: Professional-quality language translation with context preservation

**Capabilities**:
- Multi-language support (50+ languages)
- Context-aware translation
- Professional tone preservation
- Technical term handling

**API**:
```json
{
  "text": "Text to translate",
  "target_language": "Spanish|French|German|etc.",
  "source_language": "English (optional)"
}
```

**Use Cases**:
- Content internationalization
- Real-time chat translation
- Document localization
- Customer support

### 2. 📄 Summarization Tool  

**Purpose**: Intelligent text summarization and key point extraction

**Capabilities**:
- Length-controlled summaries
- Key point extraction
- Compression ratio analysis
- Context preservation

**API**:
```json
{
  "text": "Long text to summarize",
  "max_length": 100,
  "key_points": true
}
```

**Use Cases**:
- Document processing
- News article summaries
- Meeting notes
- Content curation

### 3. 😊 Sentiment Analysis Tool

**Purpose**: Emotion and tone detection with confidence scoring

**Capabilities**:
- Positive/Negative/Neutral classification
- Confidence scoring (0-100%)
- Detailed emotion analysis
- Intensity measurement

**API**:
```json
{
  "text": "Text to analyze",
  "detailed": true
}
```

**Use Cases**:
- Customer feedback analysis
- Social media monitoring
- Content moderation
- Brand sentiment tracking

### 4. 🔍 Code Review Tool

**Purpose**: AI-powered code quality, security, and best practice analysis

**Capabilities**:
- Security vulnerability detection
- Performance optimization suggestions
- Code style and best practices
- Bug detection and fixes
- Multi-language support

**API**:
```json
{
  "code": "Source code to review",
  "language": "Go|Python|JavaScript|etc.",
  "focus": "security|performance|style|bugs"
}
```

**Use Cases**:
- Automated code review
- Security audits
- Performance optimization
- Developer education
- CI/CD integration

## 🔧 Configuration & Deployment

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `OPENAI_API_KEY` | OpenAI API key | - | Yes (or alternative) |
| `GROQ_API_KEY` | Groq API key | - | Alternative |
| `ANTHROPIC_API_KEY` | Anthropic API key | - | Alternative |
| `DEEPSEEK_API_KEY` | DeepSeek API key | - | Alternative |
| `GEMINI_API_KEY` | Google Gemini API key | - | Alternative |
| `PORT` | HTTP server port | 8084 | No |
| `REDIS_URL` | Redis connection URL | redis://localhost:6379 | No |

### AI Provider Auto-Detection

The showcase automatically detects available AI providers:

```
Priority Order:
1. OpenAI (if OPENAI_API_KEY is set)
2. Groq (if GROQ_API_KEY is set) - Ultra-fast
3. Anthropic (if ANTHROPIC_API_KEY is set) - Claude
4. DeepSeek (if DEEPSEEK_API_KEY is set) - Advanced reasoning  
5. Gemini (if GEMINI_API_KEY is set) - Google AI
```

### Docker Deployment

```bash
# Build image
docker build -t ai-tools-showcase:latest .

# Run with AI provider
docker run -p 8084:8080 \
  -e OPENAI_API_KEY="your-key" \
  -e REDIS_URL="redis://your-redis:6379" \
  ai-tools-showcase:latest
```

### Kind Cluster Deployment (Complete Self-Contained)

This example is **completely self-sufficient** for Kind cluster deployment. All dependencies (including Redis) are included.

#### Prerequisites
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [Docker](https://docs.docker.com/get-docker/) running
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed

#### Step-by-Step Kind Deployment

```bash
# 1. Create Kind cluster
kind create cluster --name gomind-ai-tools

# 2. Build Docker image
docker build -t ai-tools-showcase:latest .

# 3. Load image into Kind cluster
kind load docker-image ai-tools-showcase:latest --name gomind-ai-tools

# 4. Create secrets with your AI API keys (REQUIRED - choose at least one)
kubectl create secret generic ai-tools-secrets \
  --from-literal=OPENAI_API_KEY="sk-your-openai-key" \
  --from-literal=GROQ_API_KEY="gsk-your-groq-key" \
  --from-literal=ANTHROPIC_API_KEY="sk-ant-your-claude-key" \
  --dry-run=client -o yaml | kubectl apply -f -

# 5. Deploy everything (includes Redis + AI Tools)
kubectl apply -f k8-deployment.yaml

# 6. Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=ai-tools-showcase -n gomind-ai-tools --timeout=300s

# 7. Verify Redis is working
kubectl exec -it deployment/redis -n gomind-ai-tools -- redis-cli ping

# 8. Check deployment status
kubectl get pods,svc -n gomind-ai-tools
```

#### Access the AI Tools

```bash
# Port forward to access locally
kubectl port-forward svc/ai-tools-service 8084:80 -n gomind-ai-tools

# In another terminal, test the endpoints
curl http://localhost:8084/health
curl http://localhost:8084/api/capabilities

# Test AI translation
curl -X POST http://localhost:8084/ai/translate \
  -H "Content-Type: text/plain" \
  -d "Hello world! Translate to Spanish."
```

#### Troubleshooting Kind Deployment

**Pod not starting:**
```bash
# Check pod logs
kubectl logs -f deployment/ai-tools-showcase -n gomind-ai-tools

# Check Redis connectivity
kubectl exec -it deployment/ai-tools-showcase -n gomind-ai-tools -- sh
# Inside pod: redis-cli -h redis-service ping
```

**Image not found:**
```bash
# Verify image loaded
docker exec -it gomind-ai-tools-control-plane crictl images | grep ai-tools-showcase

# If missing, reload image
kind load docker-image ai-tools-showcase:latest --name gomind-ai-tools
kubectl rollout restart deployment/ai-tools-showcase -n gomind-ai-tools
```

**AI Provider errors:**
```bash
# Check if API keys are set correctly
kubectl get secret ai-tools-secrets -n gomind-ai-tools -o yaml

# Update secrets if needed
kubectl patch secret ai-tools-secrets -n gomind-ai-tools \
  --patch='{"data":{"OPENAI_API_KEY":"'$(echo -n "sk-your-new-key" | base64)'"}}'
```

#### Clean Up

```bash
# Delete the Kind cluster (removes everything)
kind delete cluster --name gomind-ai-tools
```

## 🧪 Advanced Usage Examples

### Batch Processing Example

```bash
# Process multiple translations
for lang in Spanish French German Italian; do
  curl -X POST http://localhost:8084/api/capabilities/demo_translation \
    -d "{\"text\":\"Welcome to our service\",\"target_language\":\"$lang\"}"
done
```

### Content Pipeline Example

```bash
# 1. Summarize content
SUMMARY=$(curl -X POST http://localhost:8084/api/capabilities/demo_summarization \
  -d '{"text":"Very long article content..."}' | jq -r '.summary')

# 2. Analyze sentiment  
curl -X POST http://localhost:8084/api/capabilities/demo_sentiment \
  -d "{\"text\":\"$SUMMARY\"}"

# 3. Translate summary
curl -X POST http://localhost:8084/api/capabilities/demo_translation \
  -d "{\"text\":\"$SUMMARY\",\"target_language\":\"Spanish\"}"
```

### Code Quality Pipeline

```bash
# Review code for different aspects
for focus in security performance style bugs; do
  echo "=== $focus Review ==="
  curl -X POST http://localhost:8084/api/capabilities/demo_code_review \
    -d "{\"code\":\"$(cat myfile.go)\",\"language\":\"Go\",\"focus\":\"$focus\"}"
done
```

## 🎯 Integration with Agents

These AI tools are designed to be **discovered and used by agents**:

```go
// Agent discovers and uses AI tools
services, _ := agent.Discovery.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeTool,
})

for _, service := range services {
    if strings.Contains(service.Name, "translation") {
        // Use translation tool
        result := agent.callService(service, translationRequest)
    }
}
```

**Benefits**:
- **Automatic Discovery**: Agents find tools via service registry
- **Dynamic Usage**: Agents adapt to available AI capabilities
- **Scalable Architecture**: Deploy tools independently
- **Fault Tolerance**: Agents handle tool failures gracefully

## 📊 Monitoring & Health

### Health Check

```bash
curl http://localhost:8084/health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "ai-tools-composite",
  "ai_provider": "OpenAI",
  "tools_available": 4,
  "capabilities": [
    "demo_translation",
    "demo_summarization", 
    "demo_sentiment",
    "demo_code_review",
    "interactive_showcase"
  ]
}
```

### Capabilities Endpoint

```bash
curl http://localhost:8084/api/capabilities
```

### Metrics (Prometheus)

- `ai_requests_total{tool="translation"}` - Request counts per tool
- `ai_request_duration_seconds{tool="sentiment"}` - Request latency
- `ai_errors_total{tool="code_review"}` - Error rates

## 🤝 Best Practices

### 1. **Error Handling**
```bash
# Always check for errors in responses
response=$(curl -s -w "%{http_code}" http://localhost:8084/api/capabilities/demo_translation -d '{}')
if [[ "${response: -3}" != "200" ]]; then
  echo "Request failed"
fi
```

### 2. **Rate Limiting**
```bash
# Implement delays for bulk processing
for text in "${texts[@]}"; do
  curl -X POST http://localhost:8084/api/capabilities/demo_sentiment -d "{\"text\":\"$text\"}"
  sleep 0.5  # Avoid overwhelming the service
done
```

### 3. **Response Validation**
```bash
# Validate AI responses
response=$(curl -X POST http://localhost:8084/api/capabilities/demo_translation -d '{"text":"hello","target_language":"Spanish"}')
if echo "$response" | jq -e '.ai_powered == true' > /dev/null; then
  echo "Valid AI response"
fi
```

## 🔍 Troubleshooting

### Common Issues

#### 1. **AI Provider Not Available**
```
Error: AI Tools require an API key. Please set one of: OPENAI_API_KEY, GROQ_API_KEY...
```
**Solution**: Set at least one AI provider API key

#### 2. **Translation Fails**
```
{"error": "Translation processing failed"}
```
**Solutions**:
- Check AI provider quota/billing
- Verify text is not empty
- Check target language spelling

#### 3. **Service Discovery Issues**
```
Tools not appearing in agent discovery
```
**Solutions**:
- Verify Redis connection: `redis-cli ping`
- Check service registration: `redis-cli KEYS "gomind:*"`
- Ensure same namespace for tools and agents

#### 4. **Performance Issues**
```
Slow response times from AI tools
```
**Solutions**:
- Try Groq provider for faster inference: `export GROQ_API_KEY="..."`
- Reduce text length for processing
- Implement caching for repeated requests

### Debug Mode

```bash
# Enable debug logging
export GOMIND_DEV_MODE=true
export LOG_LEVEL=debug
go run main.go
```

## 📚 Related Examples

- **[Agent Example](../agent-example/)** - Shows how agents discover and use these tools
- **[AI Agent Example](../ai-agent-example/)** - AI-first agent architecture
- **[Multi-Provider Example](../ai-multi-provider/)** - Provider fallback patterns
- **[Tool Example](../tool-example/)** - Basic tool development patterns

## 🚀 Next Steps

1. **🧪 Try All Tools**: Test each tool with different types of content
2. **🔗 Agent Integration**: Deploy with agents that can discover these tools
3. **📈 Production Deployment**: Use Docker/K8s for production deployment
4. **🎨 Customization**: Modify tools for your specific use cases
5. **📊 Monitoring**: Set up metrics and health monitoring

## 🎉 Key Takeaways

- **🚀 Zero Setup**: Built-in AI tools work immediately with any provider
- **🔄 Multi-Provider**: Automatic detection and failover
- **🎯 Production Ready**: Full framework integration with monitoring
- **🌐 Discoverable**: Other services can find and use these tools
- **📈 Scalable**: Deploy individually or as composite services

---

**Built with ❤️ using the GoMind Framework**  
*Showcasing production-ready AI tools for translation, summarization, sentiment analysis, and code review*