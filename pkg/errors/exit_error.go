package errors

type ExitError interface {
	error
	ExitCode() int
}

type exitError struct {
	code int
}

func NewExitError(code int) ExitError {
	return &exitError{
		code: code,
	}
}

func (e *exitError) Error() string {
	return ""
}

func (e *exitError) ExitCode() int {
	return e.code
}

// Is allows exitError to be used in errors.Is, which'll return true if the
// error being checked is of exitError type and has the same exit code. Doesn't
// handle cases where an exitError is wrapped.
func (e *exitError) Is(err error) bool {
	exit, ok := err.(*exitError)
	return ok && exit.code == e.code
}
