package contracts

import "context"

// Sidecar is an auxiliary process that runs alongside the main application.
// Used for cross-cutting concerns in production service mesh environments:
// health probes, metrics exposition, config watching, service discovery, secret rotation.
//
// Lifecycle:
//  1. App start → all Sidecar.Start() called concurrently
//  2. Start() MUST block until ctx is cancelled (similar to http.Server.ListenAndServe)
//  3. App shutdown → all Sidecar.Stop() called with a shutdown timeout context
//  4. Stop() MUST be graceful: flush pending ops, close connections
//
// Error handling:
//   - Start() returning an error means the sidecar crashed; App logs a warning but does NOT crash
//   - Stop() errors are logged but do not halt the shutdown sequence
type Sidecar interface {
	// Name returns the unique identifier of the sidecar, used in logs.
	Name() string

	// Start runs the sidecar. Blocks until ctx is cancelled.
	// nil return = clean shutdown, non-nil = unexpected failure.
	Start(ctx context.Context) error

	// Stop performs graceful shutdown. ctx carries a deadline.
	Stop(ctx context.Context) error
}
