package common

// NormalizeExitCode reinterprets an exit code that may have been stored as a
// Windows DWORD (uint32) as a signed int32 value.
//
// On Windows, exit codes are 32-bit unsigned integers. For example, exit -1
// produces 0xFFFFFFFF (4294967295) which must be reinterpreted as -1. For
// standard Unix exit codes in the range 0–255, this function is an identity
// operation, so it is safe to apply unconditionally regardless of the host or
// container OS.
//
// Values above math.MaxUint32 (0xFFFFFFFF) have their upper bits silently
// truncated to their lower 32 bits before sign-reinterpretation.
func NormalizeExitCode(code int) int {
	return int(int32(code))
}
