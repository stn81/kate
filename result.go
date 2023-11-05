package kate

// Result define the handle result for http request
type Result struct {
	ErrNO  int    `json:"errno"`
	ErrMsg string `json:"errmsg"`
	Data   any    `json:"data,omitempty"`
}
