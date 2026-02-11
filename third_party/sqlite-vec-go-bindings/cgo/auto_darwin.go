//go:build darwin

package vec

// Auto is intentionally a no-op on Darwin because process-global auto
// extension APIs are deprecated by Apple.
//
// Use InitDBHandle via a per-connection hook instead.
func Auto() {}

// Cancel is a no-op on Darwin for the same reason as Auto.
func Cancel() {}
