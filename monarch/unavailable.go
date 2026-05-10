package monarch

import "github.com/thedavidweng/monarchmoney-cli/monarch/errors"

func featureUnavailable(message string) error {
	return errors.New(errors.FEATURE_UNAVAILABLE, message, errors.CatAPI, false, nil)
}
