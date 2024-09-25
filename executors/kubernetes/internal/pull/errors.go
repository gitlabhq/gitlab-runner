package pull

import "fmt"

// compile-time assertion to ensure ImagePullError always implements the
// error interface
var _ error = &ImagePullError{}

type ImagePullError struct {
	Message   string
	Image     string
	Container string
}

func (e *ImagePullError) Error() string {
	return fmt.Sprintf("pulling image %q for container %s: %s", e.Image, e.Container, e.Message)
}
