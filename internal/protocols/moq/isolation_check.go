// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package moq

import (
	"sync/atomic"
)

// Global flag to track if MoQ is enabled
// This provides a fast check without needing config access
var moqEnabled atomic.Bool

// SetEnabled updates the global MoQ enabled state
// Should only be called by configuration manager
func SetEnabled(enabled bool) {
	moqEnabled.Store(enabled)
}

// IsEnabled returns whether MoQ is currently enabled
func IsEnabled() bool {
	return moqEnabled.Load()
}

// IsolationGuard provides a safety check for all MoQ entry points
// Usage: if !IsolationGuard() { return }
func IsolationGuard() bool {
	if !IsEnabled() {
		// MoQ is disabled, prevent any processing
		return false
	}
	return true
}

// MustBeEnabled panics if MoQ is not enabled
// Used in initialization paths that should never be called when disabled
func MustBeEnabled() {
	if !IsEnabled() {
		panic("MoQ code called when disabled - this is a bug!")
	}
}

// SafeExecute runs a function only if MoQ is enabled
func SafeExecute(fn func() error) error {
	if !IsolationGuard() {
		// silently skip execution when disabled
		return nil
	}
	return fn()
}

// Examples of how to use isolation guards:
//
// In any MoQ handler function:
//   func HandleMoQRequest() {
//       if !IsolationGuard() {
//           return // MoQ disabled, do nothing
//       }
//       // ... rest of handler code
//   }
//
// In initialization code:
//   func InitializeMoQServer() error {
//       MustBeEnabled() // will panic if called when disabled
//       // ... initialization code
//   }
//
// For optional operations:
//   err := SafeExecute(func() error {
//       // This code only runs if MoQ is enabled
//       return processMoQData()
//   })