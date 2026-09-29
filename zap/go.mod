module github.com/adnvilla/logger-go/zap

go 1.22

require (
	github.com/adnvilla/logger-go v1.2.0
	go.uber.org/zap v1.27.0
	go.uber.org/zap/exp v0.3.0
)

require go.uber.org/multierr v1.11.0 // indirect

// Local development uses the root module from this repository. Consumers
// ignore replace directives and get the root version required above, which
// the release pipeline pins to the matching release (see docs/RELEASING.md).
replace github.com/adnvilla/logger-go => ../
