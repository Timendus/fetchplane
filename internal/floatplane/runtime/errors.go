package runtime

import "fmt"

type StatusError struct {
	Status Status
	Msg    string
}

func (e *StatusError) Error() string {
	return e.Msg
}

func Skipf(format string, args ...any) error {
	return &StatusError{
		Status: StatusSkipped,
		Msg:    fmt.Sprintf(format, args...),
	}
}
