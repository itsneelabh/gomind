# 🔐 API Keys Management Guide

Complete guide for managing LLM provider API keys in GoMind Framework.

## 📋 Quick Start

### 🚀 **Automated Setup (Recommended)**

```bash
# Navigate to examples directory
cd examples

# Run the interactive setup script
./setup-api-keys.sh

# Or run specific setup
./setup-api-keys.sh local    # Local development only
./setup-api-keys.sh k8s      # Kubernetes only
./setup-api-keys.sh both     # Both environments
```

---

## 🏠 Local Development Setup

### **Method 1: Environment Variables File (.env)** ⭐

1. **Copy the example file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` with your API keys:**
   ```bash
   # Required for most examples
   OPENAI_API_KEY=sk-your-openai-key-here

   # Optional providers
   ANTHROPIC_API_KEY=sk-ant-your-anthropic-key-here
   GROQ_API_KEY=gsk_your-groq-key-here
   GOOGLE_AI_API_KEY=your-google-ai-key-here

   # External services
   WEATHER_API_KEY=your-weather-api-key-here
   ```

3. **Load environment variables:**
   ```bash
   # Option A: Source the file
   source .env

   # Option B: Auto-export all variables
   set -a; source .env; set +a
   ```

4. **Run examples:**
   ```bash
   cd agent-example
   go run main.go
   ```

### **Method 2: Direct Shell Environment**

```bash
export OPENAI_API_KEY="sk-your-key-here"
export ANTHROPIC_API_KEY="sk-ant-your-key-here"
export GROQ_API_KEY="gsk_your-key-here"
```

### **Method 3: Docker Compose**

```yaml
# docker-compose.override.yml
version: '3.8'
services:
  agent-example:
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    env_file:
      - .env
```

---

## ☸️ Kubernetes Deployment

### **Method 1: Automated Script (Recommended)**

```bash
# Setup local .env first, then create K8s secrets
./setup-api-keys.sh both
```

### **Method 2: Manual kubectl Commands**

1. **Create the namespace:**
   ```bash
   kubectl create namespace gomind-examples
   ```

2. **Create AI provider secrets:**
   ```bash
   kubectl create secret generic ai-provider-keys \
     --namespace=gomind-examples \
     --from-literal=OPENAI_API_KEY="sk-your-key-here" \
     --from-literal=ANTHROPIC_API_KEY="sk-ant-your-key-here" \
     --from-literal=GROQ_API_KEY="gsk_your-key-here" \
     --from-literal=GOOGLE_AI_API_KEY="your-google-key-here"
   ```

3. **Create external API secrets:**
   ```bash
   kubectl create secret generic external-api-keys \
     --namespace=gomind-examples \
     --from-literal=WEATHER_API_KEY="your-weather-key-here"
   ```

4. **Deploy examples:**
   ```bash
   kubectl apply -f agent-example/k8-deployment.yaml
   kubectl apply -f tool-example/k8-deployment.yaml
   ```

### **Method 3: Using YAML Files**

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-provider-keys
  namespace: gomind-examples
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-your-key-here"
  ANTHROPIC_API_KEY: "sk-ant-your-key-here"
  GROQ_API_KEY: "gsk_your-key-here"
```

```bash
kubectl apply -f secrets.yaml
```

---

## 🔐 API Key Providers

### **OpenAI** 🤖
- **Get Key:** https://platform.openai.com/api-keys
- **Format:** `sk-proj-...` or `sk-...`
- **Models:** GPT-4, GPT-3.5-turbo
- **Required for:** Most examples

### **Anthropic** 🧠
- **Get Key:** https://console.anthropic.com/
- **Format:** `sk-ant-api03-...`
- **Models:** Claude-3, Claude-2
- **Required for:** Multi-provider examples

### **Groq** ⚡
- **Get Key:** https://console.groq.com/keys
- **Format:** `gsk_...`
- **Models:** Llama-3, Mixtral, Gemma
- **Required for:** Fast inference examples

### **Google AI (Gemini)** 🌟
- **Get Key:** https://aistudio.google.com/app/apikey
- **Format:** Standard API key
- **Models:** Gemini Pro, Gemini Flash
- **Required for:** Gemini examples

### **Weather API** 🌤️
- **Get Key:** https://openweathermap.org/api
- **Format:** Standard API key
- **Required for:** Tool examples

---

## 🛡️ Security Best Practices

### **✅ Do's**
- ✅ Use separate keys for dev/staging/production
- ✅ Store keys in secure secret management systems
- ✅ Set up key rotation policies
- ✅ Monitor API usage and costs
- ✅ Use least-privilege access
- ✅ Enable key usage alerts

### **❌ Don'ts**
- ❌ Never commit API keys to version control
- ❌ Don't share keys in plain text
- ❌ Don't use production keys in development
- ❌ Don't hardcode keys in application code
- ❌ Don't store keys in container images

### **🔒 Additional Security**

1. **Add to `.gitignore`:**
   ```gitignore
   .env
   .env.*
   !.env.example
   *.key
   secrets/
   ```

2. **Use key restrictions (when available):**
   - IP restrictions
   - Domain restrictions
   - Usage quotas

3. **Monitor usage:**
   ```bash
   # Check current API usage
   curl -H "Authorization: Bearer $OPENAI_API_KEY" \
        https://api.openai.com/v1/usage
   ```

---

## 🔧 Troubleshooting

### **Common Issues**

**❌ "API key not found" error:**
```bash
# Check if environment variable is set
echo $OPENAI_API_KEY

# Check if secret exists in Kubernetes
kubectl get secret ai-provider-keys -n gomind-examples -o yaml
```

**❌ "Invalid API key format":**
- OpenAI: Should start with `sk-`
- Anthropic: Should start with `sk-ant-`
- Groq: Should start with `gsk_`

**❌ "Permission denied" in Kubernetes:**
```bash
# Check if namespace exists
kubectl get namespace gomind-examples

# Check pod logs for more details
kubectl logs -n gomind-examples deployment/ai-first-agent
```

### **Debugging Commands**

```bash
# Local environment
env | grep -E "(OPENAI|ANTHROPIC|GROQ|GOOGLE)_API_KEY"

# Kubernetes secrets
kubectl get secrets -n gomind-examples
kubectl describe secret ai-provider-keys -n gomind-examples

# Test API connectivity
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
     https://api.openai.com/v1/models
```

---

## 🚀 Advanced Setups

### **1. External Secret Management**

**HashiCorp Vault:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vault-secret
  annotations:
    vault.hashicorp.com/agent-inject: "true"
    vault.hashicorp.com/role: "gomind-app"
    vault.hashicorp.com/agent-inject-secret-openai: "secret/openai"
```

**AWS Secrets Manager:**
```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: aws-secrets
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-west-2
```

### **2. CI/CD Integration**

**GitHub Actions:**
```yaml
- name: Deploy to Kubernetes
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  run: |
    kubectl create secret generic ai-keys \
      --from-literal=OPENAI_API_KEY="$OPENAI_API_KEY"
```

**GitLab CI:**
```yaml
deploy:
  script:
    - kubectl create secret generic ai-keys
        --from-literal=OPENAI_API_KEY="$OPENAI_API_KEY"
  only:
    - main
```

### **3. Development Tools**

**direnv (.envrc):**
```bash
export OPENAI_API_KEY="sk-your-key"
export ANTHROPIC_API_KEY="sk-ant-your-key"
```

**VS Code settings.json:**
```json
{
  "terminal.integrated.env.osx": {
    "OPENAI_API_KEY": "sk-your-key-here"
  }
}
```

---

## 📚 API Key Management by Example

| Example | Required Keys | Optional Keys |
|---------|---------------|---------------|
| **agent-example** | `OPENAI_API_KEY` | `ANTHROPIC_API_KEY`, `GROQ_API_KEY` |
| **ai-multi-provider** | `OPENAI_API_KEY` | `ANTHROPIC_API_KEY`, `GROQ_API_KEY` |
| **tool-example** | `WEATHER_API_KEY` | `OPENAI_API_KEY` |
| **orchestration-example** | `OPENAI_API_KEY` | None |
| **workflow-example** | `OPENAI_API_KEY` | None |

---

## ✅ Quick Validation

Test your setup with this simple validation script:

```bash
# Create validation script
cat > validate-keys.sh << 'EOF'
#!/bin/bash
echo "🔍 Validating API Keys..."

# Check OpenAI
if [ -n "$OPENAI_API_KEY" ]; then
  echo "✅ OpenAI key found"
  curl -s -H "Authorization: Bearer $OPENAI_API_KEY" \
       https://api.openai.com/v1/models | jq '.data[0].id' 2>/dev/null
else
  echo "❌ OpenAI key missing"
fi

# Check Anthropic
if [ -n "$ANTHROPIC_API_KEY" ]; then
  echo "✅ Anthropic key found"
else
  echo "⚠️  Anthropic key missing (optional)"
fi

echo "🎉 Validation complete!"
EOF

chmod +x validate-keys.sh
./validate-keys.sh
```

---

**🎯 Need Help?**

- Run `./setup-api-keys.sh` for interactive setup
- Check logs: `kubectl logs -n gomind-examples <pod-name>`
- Verify secrets: `kubectl get secrets -n gomind-examples`