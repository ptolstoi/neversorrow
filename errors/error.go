package errors

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

type Error interface {
	StatusCode() int
	Error() string
	Stacktrace() []string
}

type internalError struct {
	cause      string
	code       int
	stacktrace []string
}

func New(cause string) Error {
	return &internalError{
		cause:      cause,
		code:       http.StatusInternalServerError,
		stacktrace: GetStacktrace(),
	}
}

func NewWithCode(cause string, code int) Error {
	return &internalError{
		cause:      cause,
		code:       code,
		stacktrace: GetStacktrace(),
	}
}

func (err *internalError) StatusCode() int {
	return err.code
}

func (err *internalError) Error() string {
	return err.cause
}

func (err *internalError) Stacktrace() []string {
	return err.stacktrace
}

func GetStacktrace() []string {
	var stacktrace []string
	pcStacktrace := make([]uintptr, 10)

	if size := runtime.Callers(3, pcStacktrace); size > 0 {
		frames := runtime.CallersFrames(pcStacktrace)
		for {
			var frame runtime.Frame
			var found bool
			if frame, found = frames.Next(); !found {
				break
			}

			fnName := frame.Function
			file := frame.File
			line := frame.Line

			parts := strings.Split(fnName, "/")
			function := fnName

			if len(parts) > 2 && strings.Contains(parts[0], ".") {
				function = strings.Join(parts[2:], "/")
			}

			stacktrace = append(stacktrace,
				fmt.Sprintf("%s(%s:%d)", function, file, line))
		}
	}

	return stacktrace
}
