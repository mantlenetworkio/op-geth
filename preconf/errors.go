package preconf

import "github.com/ethereum/go-ethereum/rpc"

type PreConfAPIError struct {
	code int
	msg  string
	err  error
}

func (e *PreConfAPIError) ErrorCode() int {
	return e.code
}

func (e *PreConfAPIError) Error() string {
	return e.msg
}

func (e *PreConfAPIError) ErrorData() interface{} {
	if e.err == nil {
		return nil
	}
	return struct {
		Error string `json:"err"`
	}{e.err.Error()}
}

func (e *PreConfAPIError) With(err error) *PreConfAPIError {
	return &PreConfAPIError{
		code: e.code,
		msg:  e.msg,
		err:  err,
	}
}

var (
	_ rpc.Error     = new(PreConfAPIError)
	_ rpc.DataError = new(PreConfAPIError)
)

var (
	TIMEOUT = "TIMEOUT"
	REVERT  = "REVERT"
	SUCCESS = "SUCCESS"
	FAILED  = "FAILED"
	INVALID = "INVALID"
	WAITING = "WAITING"
	REORG   = "REORG"

	InvalidBlockNumber  = &PreConfAPIError{code: -48000, msg: "Invalid block number"}
	InvalidTimeOut      = &PreConfAPIError{code: -48001, msg: "Invalid time out"}
	InvalidGasLimitFlow = &PreConfAPIError{code: -48002, msg: "Invalid gas limit"}

	STATUS_TIMEOUT = Response{Status: TIMEOUT, Receipts: nil}
	STATUS_FAILED  = Response{Status: FAILED, Receipts: nil}
	STATUS_REVERT  = Response{Status: REVERT, Receipts: nil}
	STATUS_INVALID = Response{Status: INVALID, Receipts: nil}
	STATUS_REOGR   = Response{Status: REORG, Receipts: nil}
)
