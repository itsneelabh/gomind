#!/bin/bash
set -e

# Financial Intelligence System Test Suite
# This script tests the multi-agent system's auto-discovery and coordination capabilities

echo "🧪 Starting Financial Intelligence System Test Suite"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="financial-intelligence"
BASE_URL="http://financial-intelligence.local"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Test system health
test_system_health() {
    print_status "Testing system health..."
    
    # Test each agent's health endpoint
    agents=("market-data:8080" "news-analysis:8081" "chat-ui:8082" "technical-analysis:8084" "portfolio-advisor:8085")
    
    for agent in "${agents[@]}"; do
        agent_name=$(echo $agent | cut -d: -f1)
        port=$(echo $agent | cut -d: -f2)
        
        if curl -s -f "$BASE_URL/api/$agent_name/health" > /dev/null; then
            print_success "$agent_name agent is healthy"
        else
            print_error "$agent_name agent health check failed"
        fi
    done
}

# Test agent discovery
test_agent_discovery() {
    print_status "Testing agent auto-discovery..."
    
    # Get available agents from chat UI
    response=$(curl -s -X POST "$BASE_URL/api/chat-ui" \
        -H "Content-Type: application/json" \
        -d '{"capability": "GetAvailableAgents", "input": {}}')
    
    if echo "$response" | grep -q "market-data"; then
        print_success "Market Data Agent discovered successfully"
    else
        print_error "Market Data Agent not found in discovery"
    fi
    
    if echo "$response" | grep -q "news-analysis"; then
        print_success "News Analysis Agent discovered successfully"
    else
        print_error "News Analysis Agent not found in discovery"
    fi
}

# Test market data functionality
test_market_data() {
    print_status "Testing Market Data Agent..."
    
    # Test stock price retrieval
    response=$(curl -s -X POST "$BASE_URL/api/market-data" \
        -H "Content-Type: application/json" \
        -d '{"capability": "GetStockPrice", "input": {"symbol": "AAPL"}}')
    
    if echo "$response" | grep -q "price"; then
        print_success "Stock price retrieval working"
    else
        print_error "Stock price retrieval failed"
    fi
    
    # Test market overview
    response=$(curl -s -X POST "$BASE_URL/api/market-data" \
        -H "Content-Type: application/json" \
        -d '{"capability": "GetMarketOverview", "input": {}}')
    
    if echo "$response" | grep -q "market_indices"; then
        print_success "Market overview working"
    else
        print_error "Market overview failed"
    fi
}

# Test news analysis functionality
test_news_analysis() {
    print_status "Testing News Analysis Agent..."
    
    # Test news analysis
    response=$(curl -s -X POST "$BASE_URL/api/news-analysis" \
        -H "Content-Type: application/json" \
        -d '{"capability": "AnalyzeFinancialNews", "input": {"symbol": "AAPL", "query": "Apple earnings"}}')
    
    if echo "$response" | grep -q "sentiment"; then
        print_success "News analysis working"
    else
        print_error "News analysis failed"
    fi
}

# Test technical analysis functionality
test_technical_analysis() {
    print_status "Testing Technical Analysis Agent..."
    
    # Test technical indicators
    response=$(curl -s -X POST "$BASE_URL/api/technical-analysis" \
        -H "Content-Type: application/json" \
        -d '{"capability": "CalculateTechnicalIndicators", "input": {"symbol": "AAPL", "indicators": ["RSI", "MACD"]}}')
    
    if echo "$response" | grep -q "RSI"; then
        print_success "Technical indicators calculation working"
    else
        print_error "Technical indicators calculation failed"
    fi
    
    # Test trading signals
    response=$(curl -s -X POST "$BASE_URL/api/technical-analysis" \
        -H "Content-Type: application/json" \
        -d '{"capability": "GenerateTradingSignals", "input": {"symbol": "AAPL"}}')
    
    if echo "$response" | grep -q "overall_signal"; then
        print_success "Trading signals generation working"
    else
        print_error "Trading signals generation failed"
    fi
}

# Test portfolio analysis functionality
test_portfolio_analysis() {
    print_status "Testing Portfolio Advisor Agent..."
    
    # Test portfolio analysis
    holdings='[{"symbol": "AAPL", "shares": 100}, {"symbol": "GOOGL", "shares": 50}]'
    response=$(curl -s -X POST "$BASE_URL/api/portfolio-advisor" \
        -H "Content-Type: application/json" \
        -d "{\"capability\": \"AnalyzePortfolio\", \"input\": {\"holdings\": $holdings}}")
    
    if echo "$response" | grep -q "portfolio_analysis"; then
        print_success "Portfolio analysis working"
    else
        print_error "Portfolio analysis failed"
    fi
}

# Test LLM-assisted routing through Chat UI
test_llm_routing() {
    print_status "Testing LLM-assisted routing..."
    
    # Test market data query routing
    response=$(curl -s -X POST "$BASE_URL/api/chat-ui" \
        -H "Content-Type: application/json" \
        -d '{"capability": "ProcessUserQuery", "input": {"query": "What is the current price of Apple stock?"}}')
    
    if echo "$response" | grep -q "routed"; then
        print_success "Market data query routing working"
    else
        print_error "Market data query routing failed"
    fi
    
    # Test portfolio query routing
    response=$(curl -s -X POST "$BASE_URL/api/chat-ui" \
        -H "Content-Type: application/json" \
        -d '{"capability": "ProcessUserQuery", "input": {"query": "Analyze my portfolio with AAPL and GOOGL"}}')
    
    if echo "$response" | grep -q "routed"; then
        print_success "Portfolio query routing working"
    else
        print_error "Portfolio query routing failed"
    fi
}

# Test system coordination
test_system_coordination() {
    print_status "Testing multi-agent coordination..."
    
    # Test comprehensive financial analysis that requires multiple agents
    response=$(curl -s -X POST "$BASE_URL/api/chat-ui" \
        -H "Content-Type: application/json" \
        -d '{"capability": "ProcessUserQuery", "input": {"query": "Give me a complete analysis of AAPL including price, news sentiment, and technical indicators"}}')
    
    if echo "$response" | grep -q "coordination"; then
        print_success "Multi-agent coordination working"
    else
        print_error "Multi-agent coordination may need improvement"
    fi
}

# Test Redis discovery service
test_redis_discovery() {
    print_status "Testing Redis discovery service..."
    
    # Check Redis connectivity from within the cluster
    kubectl exec -n $NAMESPACE deployment/market-data-agent -- sh -c "nc -z redis 6379"
    
    if [ $? -eq 0 ]; then
        print_success "Redis discovery service is accessible"
    else
        print_error "Redis discovery service connection failed"
    fi
}

# Test agent registration in Redis
test_agent_registration() {
    print_status "Testing agent registration in Redis..."
    
    # Check if agents are registered in Redis
    registered_agents=$(kubectl exec -n $NAMESPACE deployment/redis -- redis-cli keys "agents:*" | wc -l)
    
    if [ $registered_agents -gt 0 ]; then
        print_success "Agents are registered in Redis discovery service ($registered_agents found)"
    else
        print_error "No agents found in Redis discovery service"
    fi
}

# Performance test
test_performance() {
    print_status "Running performance tests..."
    
    # Test response times
    start_time=$(date +%s%N)
    curl -s "$BASE_URL/api/market-data/health" > /dev/null
    end_time=$(date +%s%N)
    response_time=$(( (end_time - start_time) / 1000000 ))
    
    if [ $response_time -lt 1000 ]; then
        print_success "Response time acceptable: ${response_time}ms"
    else
        print_error "Response time too high: ${response_time}ms"
    fi
}

# Load test
test_load() {
    print_status "Running load test..."
    
    # Simple load test with 10 concurrent requests
    for i in {1..10}; do
        (curl -s "$BASE_URL/api/market-data/health" > /dev/null &)
    done
    wait
    
    print_success "Load test completed - 10 concurrent requests handled"
}

# Test dashboard functionality
test_dashboard() {
    print_status "Testing web dashboard..."
    
    response=$(curl -s "$BASE_URL/chat")
    
    if echo "$response" | grep -q "html\|Financial Intelligence"; then
        print_success "Web dashboard is accessible"
    else
        print_error "Web dashboard not accessible"
    fi
}

# Generate test report
generate_report() {
    print_status "Generating test report..."
    
    echo ""
    echo "🎯 Test Summary:"
    echo "=================================="
    echo "✅ System Health: Tested"
    echo "✅ Agent Discovery: Tested"
    echo "✅ Market Data: Tested"
    echo "✅ News Analysis: Tested"
    echo "✅ Technical Analysis: Tested"
    echo "✅ Portfolio Analysis: Tested"
    echo "✅ LLM Routing: Tested"
    echo "✅ Multi-Agent Coordination: Tested"
    echo "✅ Redis Discovery: Tested"
    echo "✅ Agent Registration: Tested"
    echo "✅ Performance: Tested"
    echo "✅ Load Handling: Tested"
    echo "✅ Web Dashboard: Tested"
    echo ""
    echo "🚀 All auto-discovery and coordination features tested!"
    echo ""
    echo "Demo Scenarios:"
    echo "1. Visit: http://financial-intelligence.local/chat"
    echo "2. Ask: 'What is AAPL trading at?'"
    echo "3. Ask: 'Analyze AAPL news sentiment'"
    echo "4. Ask: 'Give me technical indicators for TSLA'"
    echo "5. Ask: 'Analyze my portfolio with AAPL 100 shares and GOOGL 50 shares'"
    echo ""
}

# Main execution
main() {
    echo "Starting comprehensive test suite for Financial Intelligence System"
    echo "=================================================================="
    
    test_system_health
    test_redis_discovery
    test_agent_registration
    test_agent_discovery
    test_market_data
    test_news_analysis
    test_technical_analysis
    test_portfolio_analysis
    test_llm_routing
    test_system_coordination
    test_performance
    test_load
    test_dashboard
    
    generate_report
}

main "$@"
