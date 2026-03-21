package codeindex

import "errors"

type setupRequiredError struct {
	message string
}

func (e *setupRequiredError) Error() string {
	return e.message
}

func newSetupRequiredError(message string) error {
	return &setupRequiredError{message: message}
}

func IsSetupRequired(err error) bool {
	var target *setupRequiredError
	return errors.As(err, &target)
}
