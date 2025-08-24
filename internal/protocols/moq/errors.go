package moq

import (
	"fmt"
)

// Error codes for MoQ protocol
// Based on moq-lite specification
const (
	// General errors (0x00-0x0F)
	ErrorCodeNoError              uint64 = 0x00
	ErrorCodeInternalError        uint64 = 0x01
	ErrorCodeUnauthorized         uint64 = 0x02
	ErrorCodeProtocolViolation    uint64 = 0x03
	ErrorCodeDuplicateTrackAlias  uint64 = 0x04
	ErrorCodeParameterLengthMismatch uint64 = 0x05
	ErrorCodeTooManySubscribes    uint64 = 0x06
	
	// Setup errors (0x10-0x1F)
	ErrorCodeVersionMismatch      uint64 = 0x10
	ErrorCodeInvalidRole          uint64 = 0x11
	ErrorCodeInvalidPath          uint64 = 0x12
	
	// Announce errors (0x20-0x2F)
	ErrorCodeAnnounceForbidden    uint64 = 0x20
	ErrorCodeAnnounceNotFound     uint64 = 0x21
	ErrorCodeAnnounceDuplicate    uint64 = 0x22
	ErrorCodeAnnounceInvalid      uint64 = 0x23
	
	// Subscribe errors (0x30-0x3F)
	ErrorCodeSubscribeForbidden   uint64 = 0x30
	ErrorCodeSubscribeNotFound    uint64 = 0x31
	ErrorCodeSubscribeInvalid     uint64 = 0x32
	ErrorCodeSubscribeExpired     uint64 = 0x33
	ErrorCodeSubscribeRateLimited uint64 = 0x34
	
	// Stream errors (0x40-0x4F)
	ErrorCodeStreamClosed         uint64 = 0x40
	ErrorCodeStreamReset          uint64 = 0x41
	ErrorCodeStreamInvalid        uint64 = 0x42
	ErrorCodeStreamBufferFull     uint64 = 0x43
	
	// QUIC/Transport errors (0x50-0x5F)
	ErrorCodeConnectionTimeout    uint64 = 0x50
	ErrorCodeConnectionRefused    uint64 = 0x51
	ErrorCodeTransportError       uint64 = 0x52
)

// ErrorCode represents a MoQ error code with description
type ErrorCode struct {
	Code   uint64
	Reason string
}

// Error implements the error interface
func (e ErrorCode) Error() string {
	return fmt.Sprintf("MoQ error %d: %s", e.Code, e.Reason)
}

// Common errors
var (
	ErrNoError = ErrorCode{
		Code:   ErrorCodeNoError,
		Reason: "no error",
	}
	
	ErrInternalError = ErrorCode{
		Code:   ErrorCodeInternalError,
		Reason: "internal error",
	}
	
	ErrUnauthorized = ErrorCode{
		Code:   ErrorCodeUnauthorized,
		Reason: "unauthorized",
	}
	
	ErrProtocolViolation = ErrorCode{
		Code:   ErrorCodeProtocolViolation,
		Reason: "protocol violation",
	}
	
	ErrVersionMismatch = ErrorCode{
		Code:   ErrorCodeVersionMismatch,
		Reason: "no compatible version",
	}
	
	ErrInvalidRole = ErrorCode{
		Code:   ErrorCodeInvalidRole,
		Reason: "invalid role",
	}
	
	ErrAnnounceForbidden = ErrorCode{
		Code:   ErrorCodeAnnounceForbidden,
		Reason: "announce forbidden",
	}
	
	ErrAnnounceNotFound = ErrorCode{
		Code:   ErrorCodeAnnounceNotFound,
		Reason: "namespace not found",
	}
	
	ErrSubscribeForbidden = ErrorCode{
		Code:   ErrorCodeSubscribeForbidden,
		Reason: "subscribe forbidden",
	}
	
	ErrSubscribeNotFound = ErrorCode{
		Code:   ErrorCodeSubscribeNotFound,
		Reason: "track not found",
	}
	
	ErrStreamClosed = ErrorCode{
		Code:   ErrorCodeStreamClosed,
		Reason: "stream closed",
	}
	
	ErrConnectionTimeout = ErrorCode{
		Code:   ErrorCodeConnectionTimeout,
		Reason: "connection timeout",
	}
)

// NewError creates a new error with custom reason
func NewError(code uint64, reason string) ErrorCode {
	return ErrorCode{
		Code:   code,
		Reason: reason,
	}
}

// GetErrorReason returns a default reason for an error code
func GetErrorReason(code uint64) string {
	switch code {
	case ErrorCodeNoError:
		return "no error"
	case ErrorCodeInternalError:
		return "internal error"
	case ErrorCodeUnauthorized:
		return "unauthorized"
	case ErrorCodeProtocolViolation:
		return "protocol violation"
	case ErrorCodeDuplicateTrackAlias:
		return "duplicate track alias"
	case ErrorCodeParameterLengthMismatch:
		return "parameter length mismatch"
	case ErrorCodeTooManySubscribes:
		return "too many subscribes"
	case ErrorCodeVersionMismatch:
		return "version mismatch"
	case ErrorCodeInvalidRole:
		return "invalid role"
	case ErrorCodeInvalidPath:
		return "invalid path"
	case ErrorCodeAnnounceForbidden:
		return "announce forbidden"
	case ErrorCodeAnnounceNotFound:
		return "announce not found"
	case ErrorCodeAnnounceDuplicate:
		return "duplicate announcement"
	case ErrorCodeAnnounceInvalid:
		return "invalid announcement"
	case ErrorCodeSubscribeForbidden:
		return "subscribe forbidden"
	case ErrorCodeSubscribeNotFound:
		return "subscribe not found"
	case ErrorCodeSubscribeInvalid:
		return "invalid subscription"
	case ErrorCodeSubscribeExpired:
		return "subscription expired"
	case ErrorCodeSubscribeRateLimited:
		return "subscription rate limited"
	case ErrorCodeStreamClosed:
		return "stream closed"
	case ErrorCodeStreamReset:
		return "stream reset"
	case ErrorCodeStreamInvalid:
		return "invalid stream"
	case ErrorCodeStreamBufferFull:
		return "stream buffer full"
	case ErrorCodeConnectionTimeout:
		return "connection timeout"
	case ErrorCodeConnectionRefused:
		return "connection refused"
	case ErrorCodeTransportError:
		return "transport error"
	default:
		return fmt.Sprintf("unknown error code %d", code)
	}
}

// IsRetryable returns true if the error is retryable
func IsRetryable(code uint64) bool {
	switch code {
	case ErrorCodeInternalError,
		ErrorCodeStreamBufferFull,
		ErrorCodeSubscribeRateLimited,
		ErrorCodeConnectionTimeout:
		return true
	default:
		return false
	}
}

// IsFatal returns true if the error is fatal and connection should be closed
func IsFatal(code uint64) bool {
	switch code {
	case ErrorCodeProtocolViolation,
		ErrorCodeVersionMismatch,
		ErrorCodeUnauthorized:
		return true
	default:
		return false
	}
}