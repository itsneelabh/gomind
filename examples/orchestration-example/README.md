# Real-World AI Orchestration Example

This example demonstrates **real AI-powered orchestration** using the GoMind framework. It builds a Multi-Source Intelligence Platform that gathers data from multiple free APIs, orchestrates specialized agents, and provides AI-synthesized insights.

## 🎯 What This Example Shows

- **Real AI Planning**: OpenAI's GPT-4 analyzes requests and creates execution plans
- **Dynamic Agent Discovery**: Agents find each other via Redis at runtime
- **Parallel Execution**: Multiple data sources queried simultaneously
- **Intelligent Synthesis**: AI combines all results into actionable insights
- **Production Ready**: Full Kubernetes deployment with health checks

## 🏗️ Architecture Overview

```
User Request → AI Orchestrator → Multiple Agents → AI Synthesis → Response
                    ↓
             [Redis Discovery]
                ↙   ↓   ↘
         Market Data  News  Social Sentiment
```

### The Agents

1. **Market Data Agent** (Port 8081)
   - Gets real stock prices from Yahoo Finance
   - Fetches cryptocurrency data from CoinGecko (free, no key needed)
   - Provides market indices

2. **News Intelligence Agent** (Port 8082)
   - Searches news from HackerNews and Reddit
   - Gets trending topics
   - No API keys required!

3. **Social Sentiment Agent** (Port 8083)
   - Analyzes Reddit discussions
   - Measures social sentiment
   - Tracks engagement metrics

4. **AI Orchestrator** (Port 8080)
   - Uses OpenAI GPT-4 to understand requests
   - Creates intelligent execution plans
   - Orchestrates agent coordination
   - Synthesizes results into insights

## 📋 Prerequisites

You only need **ONE** thing:

```bash
export OPENAI_API_KEY="your-openai-api-key"
```

That's it! All other APIs are free and require no authentication.

### Software Requirements
- Go 1.25+ installed
- Docker (for Redis)
- Optional: Kind for Kubernetes deployment

## 🚀 Quick Start (5 Minutes)

### Step 1: Set Your OpenAI Key
```bash
export OPENAI_API_KEY="sk-your-key-here"
```

### Step 2: Start Redis
```bash
docker run -d -p 6379:6379 --name redis redis:latest
```

### Step 3: Run the Launch Script
```bash
cd examples/orchestration-example
chmod +x launch.sh
./launch.sh
```

That's it! The system is now running. Test it:

```bash
# Ask about Tesla
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"query": "What is happening with Tesla? Get stock price, news, and sentiment"}'

# Compare investments
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"query": "Compare Bitcoin vs Tesla as investments"}'
```

## 🎮 Step-by-Step Manual Setup

If you prefer to understand each step:

### 1. Build Each Agent

```bash
# Build Market Data Agent
cd examples/market-data-agent
go build -o market-data-agent .

# Build News Intelligence Agent
cd ../news-intelligence-agent
go build -o news-intelligence-agent .

# Build Social Sentiment Agent
cd ../social-sentiment-agent
go build -o social-sentiment-agent .
```

### 2. Start Agents (each in separate terminal)

```bash
# Terminal 1: Market Data
cd examples/market-data-agent
./market-data-agent

# Terminal 2: News Intelligence
cd examples/news-intelligence-agent
./news-intelligence-agent

# Terminal 3: Social Sentiment
cd examples/social-sentiment-agent
./social-sentiment-agent

# Terminal 4: Orchestrator
cd examples/orchestration-example
go run main.go
```

### 3. Test the System

```bash
# Check available agents
curl http://localhost:8080/agents

# Run orchestration
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"query": "Analyze Apple stock and provide investment recommendation"}'
```

## ☸️ Kubernetes Deployment (Kind)

Deploy everything to a local Kubernetes cluster:

### 1. Setup Kind Cluster

```bash
cd examples/orchestration-example
chmod +x setup-kind.sh
./setup-kind.sh
```

This script:
- Creates a Kind cluster with port mappings
- Builds all Docker images
- Loads images into Kind
- Deploys everything automatically
- Sets up networking for local access

### 2. Verify Deployment

```bash
# Check pods
kubectl get pods -n intelligence-platform

# Wait for all pods to be ready
kubectl wait --for=condition=ready pod --all -n intelligence-platform --timeout=300s
```

### 3. Test Kubernetes Deployment

```bash
# Services are available on same ports via NodePort
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the current Bitcoin price and market sentiment?"}'
```

### 4. Clean Up

```bash
kind delete cluster --name intelligence-platform
```

## 🔍 How It Actually Works

### Example: "Analyze Tesla stock and news"

1. **Request Arrives**: User sends natural language query
2. **AI Planning**: GPT-4 creates execution plan:
   ```json
   {
     "steps": [
       {"agent": "market-data", "capability": "get_stock_quote", "params": {"symbol": "TSLA"}},
       {"agent": "news-intelligence", "capability": "search_news", "params": {"query": "Tesla"}},
       {"agent": "social-sentiment", "capability": "analyze_reddit", "params": {"topic": "Tesla"}}
     ]
   }
   ```
3. **Parallel Execution**: All three agents called simultaneously
4. **Real Data Gathered**:
   - Stock price from Yahoo Finance
   - News from HackerNews & Reddit
   - Sentiment from Reddit discussions
5. **AI Synthesis**: GPT-4 combines all data into insights:
   > "Tesla (TSLA) is trading at $263.45, up 2.3%. Recent news highlights strong Q3 deliveries and Cybertruck production ramp-up. Reddit sentiment is 68% positive, focused on Full Self-Driving progress. Investment outlook: Moderate buy for growth portfolios."

## 📝 Example Queries to Try

```bash
# Stock Analysis
"Analyze NVIDIA stock with latest news and social sentiment"

# Crypto Research
"What's happening with Bitcoin? Include price, trends, and sentiment"

# Market Comparison
"Compare Tesla and Apple stocks for investment"

# Trend Analysis
"What are the trending topics in tech news today?"

# Investment Research
"Should I invest in cryptocurrency? Analyze market conditions"
```

## 🛠️ Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENAI_API_KEY` | **Yes** | - | Your OpenAI API key |
| `REDIS_URL` | No | `redis://localhost:6379` | Redis connection URL |
| `PORT` | No | `8080` | Orchestrator port |

### Free APIs Used

- **CoinGecko**: Cryptocurrency prices (no key needed)
- **Reddit**: Public API for discussions (no auth)
- **HackerNews**: News and discussions (completely free)
- **Yahoo Finance**: Stock data via HTML parsing (no key)

## 🐛 Troubleshooting

### "No agents found"
- **Solution**: Wait 5-10 seconds after starting agents for Redis registration
- Check Redis is running: `docker ps | grep redis`

### "Orchestration failed"
- **Solution**: Verify your OpenAI API key is valid
- Check you have API credits available

### "Failed to fetch data"
- **Solution**: Ensure internet connectivity
- APIs might be temporarily down, system will use fallback data

### Port already in use
```bash
# Kill process on port 8080
lsof -ti:8080 | xargs kill -9
```

## 📂 Project Structure

```
examples/
├── orchestration-example/
│   ├── main.go                 # Real orchestrator implementation
│   ├── README.md               # This file
│   ├── launch.sh              # Local launch script
│   ├── setup-kind.sh          # Kubernetes setup script
│   ├── k8s-deployment-kind.yaml  # Kubernetes manifests
│   └── docker-compose.yml      # Docker Compose setup
├── market-data-agent/
│   ├── main.go                # Market data from Yahoo/CoinGecko
│   └── Dockerfile
├── news-intelligence-agent/
│   ├── main.go                # News from HackerNews/Reddit
│   └── Dockerfile
└── social-sentiment-agent/
    ├── main.go                # Reddit sentiment analysis
    └── Dockerfile
```

## 🚢 Production Deployment

For production Kubernetes clusters:

1. **Update Image Registry**:
   ```yaml
   image: your-registry/market-data-agent:v1.0.0
   ```

2. **Add Ingress**:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: orchestrator
   spec:
     rules:
     - host: orchestrator.example.com
       http:
         paths:
         - path: /
           backend:
             service:
               name: orchestrator
               port: 80
   ```

3. **Enable Monitoring**:
   - Add Prometheus ServiceMonitor
   - Configure Grafana dashboards
   - Set up alerts

4. **Security**:
   - Use Kubernetes Secrets for API keys
   - Enable NetworkPolicies
   - Add rate limiting

## 📊 Performance

- **Startup Time**: < 5 seconds for all agents
- **Response Time**: 2-4 seconds for full orchestration
- **Container Size**: ~15MB per agent
- **Memory Usage**: ~50MB per agent
- **Concurrent Requests**: Handles 100+ requests/second

## 🔗 Related Documentation

- [Full Technical Guide](REAL_ORCHESTRATION_GUIDE.md) - Detailed implementation
- [Framework Documentation](../../README.md) - GoMind framework docs
- [Orchestration Module](../../orchestration/README.md) - Orchestration patterns

## 💡 Tips

1. **Start Simple**: Try single-agent queries first
2. **Watch Logs**: Each agent shows what it's doing
3. **Experiment**: The AI understands natural language - be creative!
4. **Cache Works**: Repeated queries are faster (5-minute cache)

## 🤝 Contributing

To add new agents or capabilities:

1. Create new agent following the pattern in `market-data-agent/`
2. Register capabilities with clear descriptions
3. Ensure agent registers with Redis discovery
4. The orchestrator will automatically find and use it!

## 🎉 Success!

You now have a real AI orchestration system running! The orchestrator uses GPT-4 to understand your requests, coordinates multiple agents to gather data from real APIs, and synthesizes everything into useful insights.

Try asking complex questions and watch how the AI plans and executes the solution!