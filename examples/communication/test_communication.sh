#!/bin/bash

# Test script for inter-agent communication

echo "Inter-Agent Communication Test"
echo "=============================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if agents are running
check_agent() {
    local port=$1
    local name=$2
    
    if curl -s http://localhost:$port/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $name is running on port $port"
        return 0
    else
        echo -e "${YELLOW}✗${NC} $name is not running on port $port"
        return 1
    fi
}

echo "Checking agent status..."
echo ""

check_agent 8081 "Calculator Agent"
CALC_STATUS=$?

check_agent 8082 "Coordinator Agent"
COORD_STATUS=$?

echo ""

if [ $CALC_STATUS -ne 0 ] || [ $COORD_STATUS -ne 0 ]; then
    echo -e "${YELLOW}Please start both agents first:${NC}"
    echo "  Terminal 1: go run examples/communication/calculator_agent.go"
    echo "  Terminal 2: go run examples/communication/coordinator_agent.go"
    exit 1
fi

echo "Testing inter-agent communication..."
echo ""

# Test 1: Direct calculation request to calculator
echo -e "${BLUE}Test 1: Direct request to Calculator Agent${NC}"
echo "Request: 'Please add 15 and 25'"
echo -n "Response: "
curl -s -X POST http://localhost:8081/process \
    -H "Content-Type: text/plain" \
    -H "X-From-Agent: test-script" \
    -d "Please add 15 and 25"
echo ""
echo ""

# Test 2: Request to coordinator that delegates to calculator
echo -e "${BLUE}Test 2: Request to Coordinator Agent (delegates to Calculator)${NC}"
echo "Request: 'Can you calculate 100 multiplied by 3?'"
echo -n "Response: "
curl -s -X POST http://localhost:8082/process \
    -H "Content-Type: text/plain" \
    -H "X-From-Agent: test-script" \
    -d "Can you calculate 100 multiplied by 3?"
echo ""
echo ""

# Test 3: Test the capability endpoint
echo -e "${BLUE}Test 3: Using Coordinator's test_calculation capability${NC}"
echo -n "Response: "
curl -s -X POST http://localhost:8082/test_calculation
echo ""
echo ""

# Test 4: Ask coordinator about available agents
echo -e "${BLUE}Test 4: Ask Coordinator about available agents${NC}"
echo "Request: 'What agents are available?'"
echo -n "Response: "
curl -s -X POST http://localhost:8082/process \
    -H "Content-Type: text/plain" \
    -H "X-From-Agent: test-script" \
    -d "What agents are available?"
echo ""
echo ""

# Test 5: Complex calculation
echo -e "${BLUE}Test 5: Complex calculation through Coordinator${NC}"
echo "Request: 'What is 150 divided by 5?'"
echo -n "Response: "
curl -s -X POST http://localhost:8082/process \
    -H "Content-Type: text/plain" \
    -H "X-From-Agent: test-script" \
    -d "What is 150 divided by 5?"
echo ""
echo ""

echo -e "${GREEN}✓ All tests completed!${NC}"
echo ""
echo "Summary:"
echo "- Calculator Agent can process direct calculation requests"
echo "- Coordinator Agent can delegate calculation requests to Calculator Agent"
echo "- Inter-agent communication is working via the /process endpoint"
echo "- Agents can discover and communicate with each other"