package jsonrpc

import "encoding/json"

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type ErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	ErrCodeParse      = -32700
	ErrCodeInvalidReq = -32600
	ErrCodeNotFound   = -32601
	ErrCodeInternal   = -32603
	ErrCodeLocked     = -32000
	ErrCodeAuthFailed = -32001
)

func successResponse(id json.RawMessage, result any) *Response {
	return &Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

func errorResponse(id json.RawMessage, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error:   &ErrorObject{Code: code, Message: message},
		ID:      id,
	}
}
