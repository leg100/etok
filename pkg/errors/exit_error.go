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
