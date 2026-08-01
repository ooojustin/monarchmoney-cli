package cli

import "github.com/thedavidweng/monarchmoney-cli/internal/errors"

func wrapError(err error, message string) *errors.Error {
	if cliErr, ok := err.(*errors.Error); ok {
		return cliErr
	}
	return errors.New(errors.APIError, message, errors.CatAPI, false, err)
}
