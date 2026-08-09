package cli

import (
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
)

const dateFlagLayout = "2006-01-02"

// The window follows the caller's calendar day. A UTC clock ends it on tomorrow's
// date for anyone west of Greenwich during their evening.
func resolveDateRange(startDate, endDate string, now time.Time) (resolvedStartDate, resolvedEndDate string) {
	if startDate == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format(dateFlagLayout)
	}
	if endDate == "" {
		endDate = now.Format(dateFlagLayout)
	}
	return startDate, endDate
}

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
		return errors.New(errors.InvalidArguments, "start date must not be after end date", errors.CatValidation, false, nil)
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
