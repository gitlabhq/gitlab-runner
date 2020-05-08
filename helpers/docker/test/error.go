package test

// NotFoundError implements the interface that docker client checks for
// `IsErrNotFound`
// https://github.com/moby/moby/blob/f6a5ccf492e8eab969ffad8404117806b4a15a35/client/errors.go#L36-L49
type NotFoundError struct {
}

func (e *NotFoundError) NotFound() bool {
	return true
}

func (e *NotFoundError) Error() string {
	return "not found"
}
