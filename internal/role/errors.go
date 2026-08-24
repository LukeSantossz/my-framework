package role

import (
	"errors"

	"github.com/LukeSantossz/my-framework/internal/backend"
)

func asUnavailable(err error, target **backend.Unavailable) bool {
	return errors.As(err, target)
}
