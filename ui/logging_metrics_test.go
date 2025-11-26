package ui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// TestLogger captures logs for verification in tests
type TestLogger struct {
	mu   sync.Mutex
	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (t *TestLogger) Info(msg string, fields map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs = append(t.logs, LogEntry{Level: "INFO", Message: msg, Fields: copyFields(fields)})
}

func (t *TestLogger) Error(msg string, fields map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs = append(t.logs, LogEntry{Level: "ERROR", Message: msg, Fields: copyFields(fields)})
}

func (t *TestLogger) Warn(msg string, fields map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs = append(t.logs, LogEntry{Level: "WARN", Message: msg, Fields: copyFields(fields)})
}

func (t *TestLogger) Debug(msg string, fields map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs = append(t.logs, LogEntry{Level: "DEBUG", Message: msg, Fields: copyFields(fields)})
}

// Context-aware logging methods
func (t *TestLogger) InfoWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	t.Info(msg, fields)
}

func (t *TestLogger) ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	t.Error(msg, fields)
}

func (t *TestLogger) WarnWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	t.Warn(msg, fields)
}

func (t *TestLogger) DebugWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	t.Debug(msg, fields)
}

func (t *TestLogger) GetLogs() []LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]LogEntry, len(t.logs))
	copy(result, t.logs)
	return result
}

func (t *TestLogger) GetLogsByOperation(operation string) []LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []LogEntry
	for _, log := range t.logs {
		if op, exists := log.Fields["operation"]; exists && op == operation {
			result = append(result, log)
		}
	}
	return result
}

func (t *TestLogger) GetLogsByLevel(level string) []LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []LogEntry
	for _, log := range t.logs {
		if log.Level == level {
			result = append(result, log)
		}
	}
	return result
}

func (t *TestLogger) HasLogWithMessage(message string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, log := range t.logs {
		if strings.Contains(log.Message, message) {
			return true
		}
	}
	return false
}

func (t *TestLogger) HasField(operation, field string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, log := range t.logs {
		if op, exists := log.Fields["operation"]; exists && op == operation {
			if _, fieldExists := log.Fields[field]; fieldExists {
				return true
			}
		}
	}
	return false
}

func (t *TestLogger) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs = nil
}

func copyFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}
	copied := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	return copied
}

// MockMetricsRegistry captures metrics emissions for testing
type MockMetricsRegistry struct {
	mu      sync.Mutex
	metrics []MetricEntry
}

type MetricEntry struct {
	Name   string
	Value  float64
	Labels map[string]string
	Ctx    context.Context
}

func (m *MockMetricsRegistry) Counter(name string, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, MetricEntry{
		Name:   name,
		Value:  1.0,
		Labels: parseLabels(labels...),
		Ctx:    context.Background(),
	})
}

func (m *MockMetricsRegistry) EmitWithContext(ctx context.Context, name string, value float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, MetricEntry{
		Name:   name,
		Value:  value,
		Labels: parseLabels(labels...),
		Ctx:    ctx,
	})
}

func (m *MockMetricsRegistry) GetBaggage(ctx context.Context) map[string]string {
	return make(map[string]string)
}

func (m *MockMetricsRegistry) Gauge(name string, value float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, MetricEntry{
		Name:   name,
		Value:  value,
		Labels: parseLabels(labels...),
		Ctx:    context.Background(),
	})
}

func (m *MockMetricsRegistry) Histogram(name string, value float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, MetricEntry{
		Name:   name,
		Value:  value,
		Labels: parseLabels(labels...),
		Ctx:    context.Background(),
	})
}

func (m *MockMetricsRegistry) GetMetrics() []MetricEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MetricEntry, len(m.metrics))
	copy(result, m.metrics)
	return result
}

func (m *MockMetricsRegistry) GetMetricsByName(name string) []MetricEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []MetricEntry
	for _, metric := range m.metrics {
		if metric.Name == name {
			result = append(result, metric)
		}
	}
	return result
}

func (m *MockMetricsRegistry) HasMetric(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, metric := range m.metrics {
		if metric.Name == name {
			return true
		}
	}
	return false
}

func (m *MockMetricsRegistry) HasLabel(name, labelKey, labelValue string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, metric := range m.metrics {
		if metric.Name == name {
			if val, exists := metric.Labels[labelKey]; exists && val == labelValue {
				return true
			}
		}
	}
	return false
}

func (m *MockMetricsRegistry) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = nil
}

func parseLabels(labels ...string) map[string]string {
	result := make(map[string]string)
	for i := 0; i < len(labels)-1; i += 2 {
		result[labels[i]] = labels[i+1]
	}
	return result
}

// Test P1-4: AutoConfigureTransports logging
func TestAutoConfigureTransportsLogging(t *testing.T) {
	testLogger := &TestLogger{}

	config := DefaultChatAgentConfig("test-agent")
	sessions := NewMockSessionManager(DefaultSessionConfig())

	agent := NewChatAgentWithOptions(
		config,
		sessions,
		WithLogger(testLogger),
	)

	// Auto-configure with empty transport list
	agent.AutoConfigureTransports()

	t.Run("logs operation start at INFO level", func(t *testing.T) {
		logs := testLogger.GetLogsByOperation("auto_configure_transports")
		if len(logs) == 0 {
			t.Fatal("No logs found for auto_configure_transports operation")
		}

		// Find the start log
		var startLog *LogEntry
		for _, log := range logs {
			if strings.Contains(log.Message, "Starting transport auto-configuration") {
				startLog = &log
				break
			}
		}

		if startLog == nil {
			t.Fatal("No start log found")
		}

		if startLog.Level != "INFO" {
			t.Errorf("Expected INFO level, got %s", startLog.Level)
		}
	})

	t.Run("includes transport names in start log", func(t *testing.T) {
		logs := testLogger.GetLogsByOperation("auto_configure_transports")
		var startLog *LogEntry
		for _, log := range logs {
			if strings.Contains(log.Message, "Starting") {
				startLog = &log
				break
			}
		}

		if startLog == nil {
			t.Fatal("No start log found")
		}

		if _, exists := startLog.Fields["transport_names"]; !exists {
			t.Error("Missing transport_names field")
		}
	})

	t.Run("logs completion summary at INFO level", func(t *testing.T) {
		logs := testLogger.GetLogsByOperation("auto_configure_transports")
		var summaryLog *LogEntry
		for _, log := range logs {
			if strings.Contains(log.Message, "completed") {
				summaryLog = &log
				break
			}
		}

		if summaryLog == nil {
			t.Fatal("No completion summary log found")
		}

		if summaryLog.Level != "INFO" {
			t.Errorf("Expected INFO level, got %s", summaryLog.Level)
		}
	})

	t.Run("includes success metrics in summary", func(t *testing.T) {
		logs := testLogger.GetLogsByOperation("auto_configure_transports")
		var summaryLog *LogEntry
		for _, log := range logs {
			if strings.Contains(log.Message, "completed") {
				summaryLog = &log
				break
			}
		}

		requiredFields := []string{"configured_count", "failed_count", "success_rate", "total_duration"}
		for _, field := range requiredFields {
			if _, exists := summaryLog.Fields[field]; !exists {
				t.Errorf("Missing required field: %s", field)
			}
		}
	})
}

// Test P1-5: TransportManager lifecycle logging
func TestTransportManagerLifecycleLogging(t *testing.T) {
	testLogger := &TestLogger{}
	registry := NewTransportRegistry()
	manager := NewTransportManagerWithLogger(registry, testLogger)

	// Register a mock transport
	mockTransport := &MockTransport{
		name:      "test-transport",
		priority:  1,
		available: true,
	}
	registry.Register(mockTransport)

	t.Run("InitializeTransport logs comprehensively", func(t *testing.T) {
		testLogger.Clear()

		config := TransportConfig{
			MaxConnections: 100,
			Timeout:        30 * time.Second,
			CORS: CORSConfig{
				Enabled: true,
			},
			RateLimit: RateLimitConfig{
				Enabled: true,
			},
		}

		err := manager.InitializeTransport("test-transport", config)
		if err != nil {
			t.Fatalf("InitializeTransport failed: %v", err)
		}

		logs := testLogger.GetLogsByOperation("transport_initialize")
		if len(logs) == 0 {
			t.Fatal("No initialization logs found")
		}

		// Verify INFO level logging
		infoLogs := testLogger.GetLogsByLevel("INFO")
		if len(infoLogs) == 0 {
			t.Error("No INFO level logs for initialization")
		}

		// Verify comprehensive configuration fields
		requiredFields := []string{
			"transport",
			"max_connections",
			"timeout",
			"cors_enabled",
			"rate_limit_enabled",
		}

		for _, field := range requiredFields {
			if !testLogger.HasField("transport_initialize", field) {
				t.Errorf("Missing required field: %s", field)
			}
		}
	})

	t.Run("StartTransport logs with performance timing", func(t *testing.T) {
		testLogger.Clear()

		err := manager.StartTransport(context.Background(), "test-transport")
		if err != nil {
			t.Fatalf("StartTransport failed: %v", err)
		}

		logs := testLogger.GetLogsByOperation("transport_start")
		if len(logs) == 0 {
			t.Fatal("No start logs found")
		}

		// Verify success log has duration
		var successLog *LogEntry
		for _, log := range logs {
			if strings.Contains(log.Message, "successfully") {
				successLog = &log
				break
			}
		}

		if successLog == nil {
			t.Fatal("No success log found")
		}

		if _, exists := successLog.Fields["startup_duration"]; !exists {
			t.Error("Missing startup_duration field")
		}
	})

	t.Run("StopTransport detects shutdown type", func(t *testing.T) {
		testLogger.Clear()

		// Test graceful shutdown (normal context)
		ctx := context.Background()
		err := manager.StopTransport(ctx, "test-transport")
		if err != nil {
			t.Fatalf("StopTransport failed: %v", err)
		}

		logs := testLogger.GetLogsByOperation("transport_stop")
		if len(logs) == 0 {
			t.Fatal("No stop logs found")
		}

		// Verify shutdown type is logged
		if !testLogger.HasField("transport_stop", "shutdown_type") {
			t.Error("Missing shutdown_type field")
		}
	})

	t.Run("HealthCheckTransport logs response time", func(t *testing.T) {
		testLogger.Clear()

		// Need to restart transport for health check
		manager.StartTransport(context.Background(), "test-transport")

		err := manager.HealthCheckTransport(context.Background(), "test-transport")
		if err != nil {
			t.Fatalf("HealthCheckTransport failed: %v", err)
		}

		logs := testLogger.GetLogsByOperation("transport_health_check")
		if len(logs) == 0 {
			t.Fatal("No health check logs found")
		}

		// Verify response time is logged
		var healthLog *LogEntry
		for _, log := range logs {
			if log.Level == "INFO" {
				healthLog = &log
				break
			}
		}

		if healthLog == nil {
			t.Fatal("No INFO level health check log found")
		}

		if _, exists := healthLog.Fields["response_time"]; !exists {
			t.Error("Missing response_time field")
		}
	})
}

// Test P2-6: Metrics integration
func TestMetricsIntegration(t *testing.T) {
	mockRegistry := &MockMetricsRegistry{}

	// Set global metrics registry
	originalRegistry := core.GetGlobalMetricsRegistry()
	core.SetMetricsRegistry(mockRegistry)
	defer func() {
		if originalRegistry != nil {
			core.SetMetricsRegistry(originalRegistry)
		}
	}()

	t.Run("ChatAgent CreateSession emits metrics", func(t *testing.T) {
		mockRegistry.Clear()

		config := DefaultChatAgentConfig("test-agent")
		sessions := NewMockSessionManager(DefaultSessionConfig())
		agent := NewChatAgentWithOptions(config, sessions)

		_, err := agent.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Verify operation metrics
		if !mockRegistry.HasMetric("gomind.ui.operations") {
			t.Error("Missing gomind.ui.operations metric")
		}

		// Verify session-specific metrics
		if !mockRegistry.HasMetric("gomind.ui.session.operations") {
			t.Error("Missing gomind.ui.session.operations metric")
		}

		if !mockRegistry.HasMetric("gomind.ui.session.duration") {
			t.Error("Missing gomind.ui.session.duration metric")
		}
	})

	t.Run("ChatAgent StreamResponse emits metrics", func(t *testing.T) {
		// Skip: StreamResponse requires AI client which is complex to mock for this test
		// The metrics emission code is structurally identical to CreateSession
		// and will be tested in integration tests with full AI client setup
		t.Skip("StreamResponse requires AI client - tested in integration tests")
	})

	t.Run("Metrics have correct labels", func(t *testing.T) {
		mockRegistry.Clear()

		config := DefaultChatAgentConfig("test-agent")
		sessions := NewMockSessionManager(DefaultSessionConfig())
		agent := NewChatAgentWithOptions(config, sessions)

		_, err := agent.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Check operation metrics has component label
		if !mockRegistry.HasLabel("gomind.ui.operations", "component", "ui") {
			t.Error("Missing component=ui label on gomind.ui.operations")
		}

		// Check session metrics has status label
		sessionMetrics := mockRegistry.GetMetricsByName("gomind.ui.session.operations")
		if len(sessionMetrics) == 0 {
			t.Fatal("No session operation metrics found")
		}

		hasStatus := false
		for _, m := range sessionMetrics {
			if _, exists := m.Labels["status"]; exists {
				hasStatus = true
				break
			}
		}

		if !hasStatus {
			t.Error("Missing status label on gomind.ui.session.operations")
		}
	})

	t.Run("TransportManager emits metrics", func(t *testing.T) {
		mockRegistry.Clear()

		testLogger := &TestLogger{}
		registry := NewTransportRegistry()
		manager := NewTransportManagerWithLogger(registry, testLogger)

		mockTransport := &MockTransport{
			name:      "test-transport",
			priority:  1,
			available: true,
		}
		registry.Register(mockTransport)

		config := TransportConfig{
			MaxConnections: 100,
			Timeout:        30 * time.Second,
		}

		err := manager.InitializeTransport("test-transport", config)
		if err != nil {
			t.Fatalf("InitializeTransport failed: %v", err)
		}

		// Verify transport metrics
		if !mockRegistry.HasMetric("gomind.ui.transport.operations") {
			t.Error("Missing gomind.ui.transport.operations metric")
		}

		if !mockRegistry.HasMetric("gomind.ui.transport.duration") {
			t.Error("Missing gomind.ui.transport.duration metric")
		}
	})

	t.Run("Graceful degradation when registry is nil", func(t *testing.T) {
		// Set registry to nil
		core.SetMetricsRegistry(nil)
		defer core.SetMetricsRegistry(mockRegistry)

		// Operations should not panic
		config := DefaultChatAgentConfig("test-agent")
		sessions := NewMockSessionManager(DefaultSessionConfig())
		agent := NewChatAgentWithOptions(config, sessions)

		// This should not panic
		_, err := agent.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("CreateSession failed with nil registry: %v", err)
		}
	})
}

// Test that metrics emission doesn't affect operation outcomes
func TestMetricsDoNotAffectOperations(t *testing.T) {
	// Create agent without metrics
	core.SetMetricsRegistry(nil)

	config1 := DefaultChatAgentConfig("agent1")
	sessions1 := NewMockSessionManager(DefaultSessionConfig())
	agent1 := NewChatAgentWithOptions(config1, sessions1)

	session1, err1 := agent1.CreateSession(context.Background())

	// Create agent with metrics
	mockRegistry := &MockMetricsRegistry{}
	core.SetMetricsRegistry(mockRegistry)
	defer core.SetMetricsRegistry(nil)

	config2 := DefaultChatAgentConfig("agent2")
	sessions2 := NewMockSessionManager(DefaultSessionConfig())
	agent2 := NewChatAgentWithOptions(config2, sessions2)

	session2, err2 := agent2.CreateSession(context.Background())

	// Both should succeed
	if err1 != nil || err2 != nil {
		t.Errorf("Session creation failed: err1=%v, err2=%v", err1, err2)
	}

	// Both should create valid sessions
	if session1 == nil || session2 == nil {
		t.Error("Session creation returned nil")
	}

	// Verify metrics were only emitted for agent2
	metrics := mockRegistry.GetMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be emitted for agent2")
	}
}