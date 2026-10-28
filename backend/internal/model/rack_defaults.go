package model

const (
	// Rack defaults are shared by rack instances, template versions and the
	// built-in STANDARD-42U template. Keep the frontend rackDefaults.ts aligned.
	DefaultRackStatus        = RackStatusAvailable
	DefaultRackType          = "STANDARD"
	DefaultRackUHeight       = 42
	DefaultRackWidthMm       = 600
	DefaultRackDepthMm       = 1200
	DefaultRackHeightMm      = 2000
	DefaultRackRotation      = 0
	DefaultRackDualPower     = false
	DefaultRackInputCircuits = 0
	DefaultRackPDUCount      = 0
	DefaultRackSortOrder     = 0
	DefaultRackTemplateCode  = "STANDARD-42U"
)
