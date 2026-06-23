package usage_log

// NewLogger returns a Storage that fans out to the given writers.
// Returns nil if no writers are provided. If only one writer is
// given, it is returned directly without wrapping.
func NewLogger(writers ...Storage) Storage {
	switch len(writers) {
	case 0:
		return nil
	case 1:
		return writers[0]
	default:
		return NewMultiWriter(writers...)
	}
}
