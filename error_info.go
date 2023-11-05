package kate

// ErrorInfo defines the error type
type ErrorInfo interface {
	error
	Code() int
}

// errSimple define a basic error type which implements the ErrorInfo interface
type errSimple struct {
	ErrCode    int    `json:"code"`
	ErrMessage string `json:"message"`
}

// NewError create an errSimple instance
func NewError(code int, message string) ErrorInfo {
	return &errSimple{code, message}
}

// Code implements the `ErrorInfo.Code()` method
func (e *errSimple) Code() int {
	return e.ErrCode
}

// Error implements the `ErrorInfo.Error()` method
func (e *errSimple) Error() string {
	return e.ErrMessage
}

// ErrorInfoWithData defines the error type with extra data
type ErrorInfoWithData interface {
	error
	Code() int
	Data() any
}

// errWithData defines a basic error type with extra data which implements ErrorInfoWithData interface
type errWithData struct {
	*errSimple
	ErrData any
}

// NewErrorWithData create a errWithData instance
func NewErrorWithData(code int, message string, data any) ErrorInfoWithData {
	return &errWithData{
		errSimple: &errSimple{code, message},
		ErrData:   data,
	}
}

// Data implements the `ErrorInfoWithData.Data()` method
func (e *errWithData) Data() any {
	return e.ErrData
}
