package cli

import (
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
)

const dateFlagLayout = "2006-01-02"

func validateDateFlag(name, value string) *errors.Error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(dateFlagLayout, value); err != nil {
		return errors.New(errors.InvalidArguments, "--"+name+" must use YYYY-MM-DD", errors.CatValidation, false, err)
	}
	return nil
}

func validateRequiredDateFlag(name, value string) *errors.Error {
	if value == "" {
		return errors.New(errors.InvalidArguments, "--"+name+" is required", errors.CatValidation, false, nil)
	}
	return validateDateFlag(name, value)
}

// Both operands are zero-padded ISO-8601 once parsed, so lexical order is chronological order.
func validateDateRange(from, to string) *errors.Error {
	if err := validateDateFlag("from", from); err != nil {
		return err
	}
	if err := validateDateFlag("to", to); err != nil {
		return err
	}
	if from != "" && to != "" && from > to {
		return errors.New(errors.InvalidArguments, "--from must not be after --to", errors.CatValidation, false, nil)
	}
	return nil
}

func validatePositiveInt(name string, value int) *errors.Error {
	if value <= 0 {
		return errors.New(errors.InvalidArguments, "--"+name+" must be greater than zero", errors.CatValidation, false, nil)
	}
	return nil
}

func validateNonNegativeInt(name string, value int) *errors.Error {
	if value < 0 {
		return errors.New(errors.InvalidArguments, "--"+name+" must not be negative", errors.CatValidation, false, nil)
	}
	return nil
}
