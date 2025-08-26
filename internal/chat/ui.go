package chat

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// ChatUITemplate provides a simple HTML template for conversational agents
const ChatUITemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.AgentName}} - Conversational Agent</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background-color: #f5f5f5;
            height: 100vh;
            display: flex;
            flex-direction: column;
        }
        
        .header {
            background-color: #007bff;
            color: white;
            padding: 1rem;
            text-align: center;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        
        .chat-container {
            flex: 1;
            display: flex;
            flex-direction: column;
            max-width: 800px;
            margin: 0 auto;
            width: 100%;
            background: white;
            box-shadow: 0 0 20px rgba(0,0,0,0.1);
        }
        
        .messages {
            flex: 1;
            padding: 1rem;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }
        
        .message {
            max-width: 80%;
            padding: 0.75rem 1rem;
            border-radius: 1rem;
            word-wrap: break-word;
        }
        
        .message.user {
            align-self: flex-end;
            background-color: #007bff;
            color: white;
        }
        
        .message.assistant {
            align-self: flex-start;
            background-color: #e9ecef;
            color: #333;
        }
        
        .message.assistant.streaming {
            position: relative;
        }
        
        .message.assistant.streaming::after {
            content: '▋';
            animation: blink 1s infinite;
            margin-left: 2px;
        }
        
        @keyframes blink {
            0%, 50% { opacity: 1; }
            51%, 100% { opacity: 0; }
        }
        
        .message.system {
            align-self: center;
            background-color: #ffc107;
            color: #856404;
            font-size: 0.9rem;
            max-width: 90%;
        }
        
        .input-container {
            padding: 1rem;
            border-top: 1px solid #dee2e6;
            display: flex;
            gap: 0.5rem;
        }
        
        .message-input {
            flex: 1;
            padding: 0.75rem;
            border: 1px solid #ced4da;
            border-radius: 0.5rem;
            font-size: 1rem;
            outline: none;
            transition: border-color 0.2s;
        }
        
        .message-input:focus {
            border-color: #007bff;
            box-shadow: 0 0 0 0.2rem rgba(0, 123, 255, 0.25);
        }
        
        .send-button {
            padding: 0.75rem 1.5rem;
            background-color: #007bff;
            color: white;
            border: none;
            border-radius: 0.5rem;
            font-size: 1rem;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        
        .send-button:hover:not(:disabled) {
            background-color: #0056b3;
        }
        
        .send-button:disabled {
            background-color: #6c757d;
            cursor: not-allowed;
        }
        
        .status {
            padding: 0.5rem 1rem;
            text-align: center;
            font-size: 0.9rem;
            color: #6c757d;
            border-top: 1px solid #dee2e6;
        }
        
        .typing-indicator {
            display: none;
            align-self: flex-start;
            padding: 0.75rem 1rem;
            background-color: #e9ecef;
            border-radius: 1rem;
            color: #6c757d;
            font-style: italic;
        }
        
        @media (max-width: 768px) {
            .chat-container {
                height: 100vh;
                max-width: none;
                margin: 0;
                border-radius: 0;
            }
            
            .message {
                max-width: 90%;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.AgentName}}</h1>
        <p>Conversational Agent Interface</p>
    </div>
    
    <div class="chat-container">
        <div class="messages" id="messages">
            <div class="message system">
                Welcome! Start a conversation with {{.AgentName}}. Your session ID: <span id="session-id">Loading...</span>
            </div>
        </div>
        
        <div class="typing-indicator" id="typing-indicator">
            Agent is typing...
        </div>
        
        <div class="input-container">
            <input 
                type="text" 
                id="message-input" 
                class="message-input" 
                placeholder="Type your message..." 
                autocomplete="off"
            >
            <button id="send-button" class="send-button">Send</button>
        </div>
        
        <div class="status" id="status">
            Ready to chat
        </div>
    </div>

    <script>
        class ChatInterface {
            constructor() {
                this.sessionId = null;
                this.messageInput = document.getElementById('message-input');
                this.sendButton = document.getElementById('send-button');
                this.messagesContainer = document.getElementById('messages');
                this.statusElement = document.getElementById('status');
                this.typingIndicator = document.getElementById('typing-indicator');
                this.sessionIdElement = document.getElementById('session-id');
                
                this.initializeEventListeners();
                this.createSession();
            }
            
            initializeEventListeners() {
                this.sendButton.addEventListener('click', () => this.sendMessage());
                this.messageInput.addEventListener('keypress', (e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        this.sendMessage();
                    }
                });
            }
            
            async createSession() {
                try {
                    const response = await fetch('/conversation/session', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        }
                    });
                    
                    if (!response.ok) {
                        throw new Error('Failed to create session');
                    }
                    
                    const data = await response.json();
                    this.sessionId = data.sessionId;
                    this.sessionIdElement.textContent = this.sessionId;
                    this.updateStatus('Connected');
                } catch (error) {
                    console.error('Error creating session:', error);
                    this.updateStatus('Failed to connect');
                }
            }
            
            async sendMessage() {
                const message = this.messageInput.value.trim();
                if (!message || !this.sessionId) return;
                
                // Add user message to UI
                this.addMessage('user', message);
                this.messageInput.value = '';
                this.setLoading(true);
                this.showTypingIndicator();
                
                // Create assistant message placeholder for streaming
                const assistantMessageElement = this.createStreamingMessage();
                
                try {
                    // Use the AI streaming endpoint for real-time responses
                    const response = await fetch('/ai/stream', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({
                            message: message,
                            system_prompt: "You are a helpful AI assistant. Provide clear, accurate, and engaging responses.",
                            temperature: 0.7,
                            max_tokens: 1000
                        })
                    });
                    
                    if (!response.ok) {
                        throw new Error('Failed to send message');
                    }
                    
                    // Handle streaming response
                    const reader = response.body.getReader();
                    const decoder = new TextDecoder();
                    let assistantResponse = '';
                    
                    while (true) {
                        const { done, value } = await reader.read();
                        if (done) break;
                        
                        const chunk = decoder.decode(value);
                        const lines = chunk.split('\n');
                        
                        for (const line of lines) {
                            if (line.startsWith('data: ')) {
                                try {
                                    const data = JSON.parse(line.slice(6));
                                    if (data.chunk_type === 'content' && data.content) {
                                        assistantResponse += data.content;
                                        this.updateStreamingMessage(assistantMessageElement, assistantResponse);
                                        
                                        // Remove streaming class when complete
                                        if (data.is_complete) {
                                            assistantMessageElement.classList.remove('streaming');
                                        }
                                    }
                                } catch (e) {
                                    // Ignore JSON parsing errors for incomplete chunks
                                }
                            }
                        }
                    }
                    
                    // Ensure streaming class is removed when done
                    assistantMessageElement.classList.remove('streaming');
                    
                    this.updateStatus('Ready to chat');
                } catch (error) {
                    console.error('Error sending message:', error);
                    this.addMessage('system', 'Error: Failed to send message. Please try again.');
                    this.updateStatus('Error occurred');
                } finally {
                    this.setLoading(false);
                    this.hideTypingIndicator();
                }
            }
            
            addMessage(type, content) {
                const messageDiv = document.createElement('div');
                messageDiv.className = 'message ' + type;
                messageDiv.textContent = content;
                
                this.messagesContainer.appendChild(messageDiv);
                this.scrollToBottom();
            }
            
            createStreamingMessage() {
                const messageDiv = document.createElement('div');
                messageDiv.className = 'message assistant streaming';
                messageDiv.textContent = '';
                
                this.messagesContainer.appendChild(messageDiv);
                this.scrollToBottom();
                return messageDiv;
            }
            
            updateStreamingMessage(messageElement, content) {
                messageElement.textContent = content;
                this.scrollToBottom();
            }
            
            setLoading(loading) {
                this.sendButton.disabled = loading;
                this.messageInput.disabled = loading;
                this.sendButton.textContent = loading ? 'Sending...' : 'Send';
            }
            
            showTypingIndicator() {
                this.typingIndicator.style.display = 'block';
                this.scrollToBottom();
            }
            
            hideTypingIndicator() {
                this.typingIndicator.style.display = 'none';
            }
            
            updateStatus(status) {
                this.statusElement.textContent = status;
            }
            
            scrollToBottom() {
                this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
            }
        }
        
        // Initialize chat interface when page loads
        document.addEventListener('DOMContentLoaded', () => {
            new ChatInterface();
        });
    </script>
</body>
</html>`

// ChatUIData represents data passed to the chat UI template
type ChatUIData struct {
	AgentName string
}

// ServeChatUI serves the chat UI for conversational agents
func ServeChatUI(agentName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tmpl, err := template.New("chatui").Parse(ChatUITemplate)
		if err != nil {
			http.Error(w, "Failed to parse template", http.StatusInternalServerError)
			return
		}

		data := ChatUIData{
			AgentName: agentName,
	}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

// GenerateChatUIHTML generates a standalone HTML file for the chat interface
func GenerateChatUIHTML(agentName string) (string, error) {
	tmpl, err := template.New("chatui").Parse(ChatUITemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := ChatUIData{
		AgentName: agentName,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
