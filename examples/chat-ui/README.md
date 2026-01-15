# Chat UI

A web-based chat interface for the GoMind Travel Assistant. This frontend connects to the `travel-chat-agent` backend via Server-Sent Events (SSE) for real-time streaming responses.

## Features

- Real-time streaming responses via SSE
- Visual progress indicators for tool execution
- Session management with automatic reconnection
- Configurable backend URL
- Responsive design for mobile and desktop
- Suggestion chips for quick queries

## Prerequisites

- A modern web browser (Chrome, Firefox, Safari, Edge)
- [travel-chat-agent](../travel-chat-agent/) running on port 8095

## Quick Start

### 1. Start the Backend

First, ensure the travel-chat-agent is running:

```bash
cd ../travel-chat-agent
./setup.sh run
```

### 2. Open the UI

Simply open `index.html` in your browser:

```bash
# macOS
open index.html

# Linux
xdg-open index.html

# Windows
start index.html
```

Or serve it with a simple HTTP server:

```bash
# Python 3
python -m http.server 3000

# Then open http://localhost:3000
```

## Configuration

Click the gear icon in the header to configure:

- **Backend URL**: Default is `http://localhost:8095`

Settings are saved in localStorage.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      index.html                            │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌───────────────────┐  │  │
│  │  │   Header    │  │   Settings  │  │  Connection       │  │  │
│  │  │   + Title   │  │   Panel     │  │  Status           │  │  │
│  │  └─────────────┘  └─────────────┘  └───────────────────┘  │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │                                                      │  │  │
│  │  │              Chat Messages Area                      │  │  │
│  │  │    - User messages (right aligned)                   │  │  │
│  │  │    - Assistant messages (left aligned)               │  │  │
│  │  │    - Progress panels (tool execution)                │  │  │
│  │  │    - Streaming text chunks                           │  │  │
│  │  │                                                      │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │   Input Form  [________________________] [Send]     │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ SSE (POST /chat/stream)
                              ▼
                   ┌─────────────────────┐
                   │  travel-chat-agent  │
                   │    (Port 8095)      │
                   └─────────────────────┘
```

## SSE Event Handling

The UI handles the following SSE events from the backend:

| Event | Description |
|-------|-------------|
| `session` | New session ID created |
| `status` | Progress status (e.g., "Analyzing your request...") |
| `step` | Tool execution completion with timing |
| `chunk` | Response text chunk for streaming display |
| `done` | Request completed with metadata |
| `error` | Error with code and message |

## UI Components

### Connection Status
- 🟢 Connected - Backend is reachable
- 🟡 Connecting - Checking connection
- 🔴 Disconnected - Backend unreachable

### Progress Panel
Shows real-time tool execution:
- ⏳ Spinner - Tool executing
- ✅ Checkmark - Tool completed successfully
- ❌ Error - Tool failed

### Message Footer
After completion shows:
- Number of tools used
- Total response time

## Kubernetes Deployment

### One-Click Kind Cluster Setup

The chat-ui is deployed together with travel-chat-agent using the setup script:

```bash
# From the travel-chat-agent directory
cd ../travel-chat-agent
./setup.sh full-deploy
```

This deploys both the backend (travel-chat-agent) and frontend (chat-ui) to a local Kind cluster.

### Access URLs (After Port Forwarding)

After deployment, set up port forwarding:

```bash
# From travel-chat-agent directory
./setup.sh forward-all
```

| Service | URL | Description |
|---------|-----|-------------|
| **Chat UI** | **http://localhost:8096** | Web interface (open this in browser) |
| Travel Chat Agent | http://localhost:8095 | Backend API |

### Manual Port Forwarding

```bash
# Chat UI (frontend)
kubectl port-forward -n gomind-examples svc/chat-ui-service 8096:80

# Travel Chat Agent (backend)
kubectl port-forward -n gomind-examples svc/travel-chat-agent-service 8095:80
```

### Configuration for Kubernetes

When running in Kubernetes with port forwarding:
1. Open http://localhost:8096 in your browser
2. Click the gear icon (⚙️) in the header
3. Set Backend URL to `http://localhost:8095`
4. Click Save

## Files

```
chat-ui/
├── index.html        # Full application (HTML + CSS + JS)
├── mock.html         # Mock UI for design preview
├── Dockerfile        # Container image for nginx
├── k8-deployment.yaml # Kubernetes deployment
└── README.md         # This file
```

## Development

The UI is a single-file application with no build steps required. To modify:

1. Edit `index.html`
2. Refresh the browser

### Adding New Features

The JavaScript is organized into sections:
- **Configuration**: Backend URL, session state
- **Connection**: Health check, session creation
- **UI Helpers**: Message rendering, progress updates
- **SSE Handling**: Stream parsing, event dispatch
- **Form Handlers**: User input, suggestions

## Mock Mode

For design work without a backend, use `mock.html`:

```bash
open mock.html
```

This shows simulated responses for Tokyo, Paris, and Switzerland queries.

## Styling

The UI uses a teal color scheme with CSS custom properties. Key colors:
- Primary: `#0f766e` / `#0d9488` (teal gradient)
- Background: `#1a1a2e` / `#16213e` (dark gradient)
- Success: `#4ade80` (green)
- Error: `#dc2626` (red)
- Warning: `#fbbf24` (yellow)

## Debugging with Jaeger

The chat UI displays a request ID in the message footer (e.g., `orch-1767830612199846805`). You can use this ID to find the corresponding distributed trace in Jaeger.

### Finding a Trace by Request ID

1. Open Jaeger UI: http://localhost:16686
2. Select **Service**: `travel-chat-agent`
3. In the **Tags** field, enter:
   ```
   request_id=orch-1767830612199846805
   ```
4. Set an appropriate time range (last 15 minutes, 1 hour, etc.)
5. Click **Find Traces**

### Understanding Request ID vs Trace ID

| ID Type | Format | Purpose |
|---------|--------|---------|
| Request ID | `orch-...` (shown in UI) | Application-level identifier |
| Trace ID | Hex string (e.g., `bd0fbd7c...`) | OpenTelemetry/Jaeger identifier |

The request ID is attached as a **tag** to spans, so use the Tags field in Jaeger rather than the Trace ID field.

### Jaeger Access

After deploying with `./setup.sh full-deploy`, access Jaeger:

```bash
# From travel-chat-agent directory
./setup.sh forward-all
```

Then open http://localhost:16686

## Troubleshooting

### "Disconnected" Status

1. Check if travel-chat-agent is running: `curl http://localhost:8095/health`
2. Check browser console for CORS errors
3. Verify backend URL in settings

### No Response Streaming

1. Check browser console for SSE errors
2. Verify backend is returning proper SSE format
3. Try refreshing the page

### CORS Issues

If running from `file://`, some browsers block SSE. Use an HTTP server:

```bash
python -m http.server 3000
```

## Related Examples

- [travel-chat-agent](../travel-chat-agent/) - Backend for this UI
- [agent-with-orchestration](../agent-with-orchestration/) - Basic orchestration
