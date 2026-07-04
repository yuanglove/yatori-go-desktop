package mobilecore

import "encoding/json"

const (
	CodeOK              = ""
	CodeNotInitialized  = "CORE_NOT_INITIALIZED"
	CodeInvalidArgument = "INVALID_ARGUMENT"
	CodeUnsupported     = "UNSUPPORTED"
	CodeInternalError   = "INTERNAL_ERROR"
)

type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
	Code  string      `json:"code"`
}

func ok(data interface{}) string {
	return encodeResponse(response{OK: true, Data: data, Error: "", Code: CodeOK})
}

func fail(code, message string) string {
	if code == "" {
		code = CodeInternalError
	}
	return encodeResponse(response{OK: false, Data: nil, Error: message, Code: code})
}

func encodeResponse(resp response) string {
	b, err := json.Marshal(resp)
	if err != nil {
		fallback := response{OK: false, Data: nil, Error: err.Error(), Code: CodeInternalError}
		b, _ = json.Marshal(fallback)
	}
	return string(b)
}
