package namecheap

// deref returns the dereferenced value of a non-nil pointer as any, or nil for a nil pointer.
// Useful for fmt.Sprintf(%v) calls that need nil-safe pointer formatting.
func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
