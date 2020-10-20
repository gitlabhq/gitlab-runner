package vault

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/vault/api"
)

type unwrappedAPIResponseError struct {
	statusCode int
	apiErrors  string
}

func newUnwrappedAPIResponseError(statusCode int, errors []string) *unwrappedAPIResponseError {
	return &unwrappedAPIResponseError{
		statusCode: statusCode,
		apiErrors:  strings.Join(errors, ", "),
	}
}

func (e *unwrappedAPIResponseError) Error() string {
	return fmt.Sprintf("api error: status code %d: %s", e.statusCode, e.apiErrors)
}

func (e *unwrappedAPIResponseError) Is(err error) bool {
	eerr, ok := err.(*unwrappedAPIResponseError)
	if !ok {
		return false
	}

	return eerr.statusCode == e.statusCode && eerr.apiErrors == e.apiErrors
}

func unwrapAPIResponseError(err error) error {
	if err == nil {
		return nil
	}

	apiErr := new(api.ResponseError)
	if !errors.As(err, &apiErr) {
		return err
	}

	return newUnwrappedAPIResponseError(apiErr.StatusCode, apiErr.Errors)
}
