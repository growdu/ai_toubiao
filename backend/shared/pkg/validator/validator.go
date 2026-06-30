// Package validator wraps go-playground/validator with project conventions.
//
// Custom validators registered:
//   - hex64   : string is exactly 64 lowercase hex chars (SHA256 hex)
//   - mime    : string matches MIME format (type/subtype, no spaces)
//   - uuidstr : string parses as a valid UUID
package validator

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

var (
	mimeRe   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9!#$&^_.+-]{0,126}/[a-zA-Z0-9][a-zA-Z0-9!#$&^_.+-]{0,126}$`)
	hex64Re  = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

var v = validator.New(validator.WithRequiredStructEnabled())

func init() {
	_ = v.RegisterValidation("hex64", func(fl validator.FieldLevel) bool {
		return hex64Re.MatchString(fl.Field().String())
	})
	_ = v.RegisterValidation("mime", func(fl validator.FieldLevel) bool {
		return mimeRe.MatchString(fl.Field().String())
	})
	_ = v.RegisterValidation("uuidstr", func(fl validator.FieldLevel) bool {
		_, err := uuid.Parse(fl.Field().String())
		return err == nil
	})
}

// Validate checks the given struct against its `validate:\"...\"` tags.
// Returns the first error wrapped with field context.
func Validate(s any) error {
	if err := v.Struct(s); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			first := verr[0]
			return fmt.Errorf("field %q failed validation: %s",
				first.Field(), first.Tag())
		}
		return err
	}
	return nil
}

// MustValidate panics if validation fails. Use at startup.
func MustValidate(s any) {
	if err := Validate(s); err != nil {
		panic(fmt.Sprintf("invalid %s: %v", reflect.TypeOf(s).Name(), err))
	}
}