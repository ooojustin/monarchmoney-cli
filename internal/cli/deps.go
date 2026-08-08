package cli

import (
	stderrors "errors"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
)

func wrapError(err error, message string) *errors.Error {
	var cliErr *errors.Error
	if stderrors.As(err, &cliErr) {
		return cliErr
	}
	return errors.New(errors.APIError, message, errors.CatAPI, false, err)
}
