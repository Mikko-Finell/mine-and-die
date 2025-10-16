package cli

import (
	"errors"
	"io"
)

var ErrNotImplemented = errors.New("effectsgen: generator pipeline not implemented yet")

func Execute(stdout io.Writer, stderr io.Writer) error {
	_ = stdout
	_ = stderr

	return ErrNotImplemented
}
