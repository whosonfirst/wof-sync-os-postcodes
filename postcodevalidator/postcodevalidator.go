package postcodevalidator

import (
	"regexp"
)

var postcodeRegexp = regexp.MustCompile(`^[A-Z]{1,2}\d[A-Z\d]? ?\d[A-Z]{2}$`)

// Validate returns a boolean depending on whether the postcode is a full valid UK postcode
func Validate(postcode string) bool {
	return postcodeRegexp.MatchString(postcode)
}
