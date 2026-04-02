package tlsfetch

import "fmt"

type ErrInvalidJA3 struct {
	Input  string
	Reason string
}

func (e *ErrInvalidJA3) Error() string {
	return fmt.Sprintf("tlsfetch: invalid JA3 string: %s", e.Reason)
}

type ErrInvalidProfile struct {
	Name   string
	Reason string
}

func (e *ErrInvalidProfile) Error() string {
	return fmt.Sprintf("tlsfetch: invalid profile %q: %s", e.Name, e.Reason)
}

type ErrProxyConnect struct {
	Addr  string
	Cause error
}

func (e *ErrProxyConnect) Error() string {
	return fmt.Sprintf("tlsfetch: proxy connect to %s failed: %v", e.Addr, e.Cause)
}

func (e *ErrProxyConnect) Unwrap() error { return e.Cause }
