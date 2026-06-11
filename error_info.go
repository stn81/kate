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

// HTTPStatusCarrier 是 ErrorInfo 的可选扩展：错误值显式携带 HTTP 状态码。
// 仅 RESTHandler 渲染时读取；BaseHandler（errno 风格，恒 200）会忽略它，
// 因此公共 service 层可以安全返回带状态的错误，由端点的 handler 类型决定渲染契约。
type HTTPStatusCarrier interface {
	HTTPStatus() int
}

// errWithHTTPStatus 在既有 ErrorInfo 外披一层 HTTP 状态；Unwrap 保留内层的
// Code/Error/Data 语义（errors.As 链可达）。
type errWithHTTPStatus struct {
	ErrorInfo
	httpStatus int
}

// HTTPStatus implements the `HTTPStatusCarrier.HTTPStatus()` method
func (e *errWithHTTPStatus) HTTPStatus() int {
	return e.httpStatus
}

// Unwrap exposes the inner ErrorInfo to errors.As/errors.Is
func (e *errWithHTTPStatus) Unwrap() error {
	return e.ErrorInfo
}

// WithHTTPStatus wraps an existing ErrorInfo with an explicit http status
func WithHTTPStatus(err ErrorInfo, httpStatus int) ErrorInfo {
	return &errWithHTTPStatus{ErrorInfo: err, httpStatus: httpStatus}
}

// NewHTTPError create an ErrorInfo carrying an explicit http status
func NewHTTPError(httpStatus, errno int, message string) ErrorInfo {
	return WithHTTPStatus(NewError(errno, message), httpStatus)
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
