module github.com/itsneelabh/gomind/telemetry

go 1.23

replace github.com/itsneelabh/gomind/core => ../core

require (
	github.com/itsneelabh/gomind/core v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v1.37.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.37.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.37.0
	go.opentelemetry.io/otel/metric v1.37.0
	go.opentelemetry.io/otel/sdk v1.37.0
	go.opentelemetry.io/otel/trace v1.37.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0
)