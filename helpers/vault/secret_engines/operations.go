package secret_engines

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

type OperationType string

const (
	getOperation    OperationType = "get"
	putOperation    OperationType = "put"
	deleteOperation OperationType = "delete"
)

type OperationNotSupportedError struct {
	secretEngineName string
	operationType    OperationType
}

func NewUnsupportedGetOperationErr(engine vault.SecretEngine) *OperationNotSupportedError {
	return newErrOperationNotSupported(engine, getOperation)
}

func NewUnsupportedPutOperationErr(engine vault.SecretEngine) *OperationNotSupportedError {
	return newErrOperationNotSupported(engine, putOperation)
}

func NewUnsupportedDeleteOperationErr(engine vault.SecretEngine) *OperationNotSupportedError {
	return newErrOperationNotSupported(engine, deleteOperation)
}

func newErrOperationNotSupported(engine vault.SecretEngine, operationType OperationType) *OperationNotSupportedError {
	return &OperationNotSupportedError{
		secretEngineName: engine.EngineName(),
		operationType:    operationType,
	}
}

func (e *OperationNotSupportedError) Error() string {
	return fmt.Sprintf("operation %q for secret engine %q is not supported", e.operationType, e.secretEngineName)
}

func (e *OperationNotSupportedError) Is(err error) bool {
	eerr, ok := err.(*OperationNotSupportedError)
	if !ok {
		return false
	}

	return eerr.secretEngineName == e.secretEngineName && eerr.operationType == e.operationType
}
