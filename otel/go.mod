module github.com/adnvilla/logger-go/otel

go 1.22

require (
	github.com/adnvilla/logger-go v1.2.0
	go.opentelemetry.io/otel/trace v1.28.0
)

require go.opentelemetry.io/otel v1.28.0 // indirect

// Local development uses the root module from this repository. Consumers
// ignore replace directives and get the root version required above, which
// the release pipeline pins to the matching release (see docs/RELEASING.md).
replace github.com/adnvilla/logger-go => ../
