module github.com/itsneelabh/gomind/workflow

go 1.23

replace (
	github.com/itsneelabh/gomind/core => ../core
	github.com/itsneelabh/gomind/ai => ../ai
)

require (
	github.com/itsneelabh/gomind/core v0.0.0-00010101000000-000000000000
	github.com/itsneelabh/gomind/ai v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v3 v3.0.1
)