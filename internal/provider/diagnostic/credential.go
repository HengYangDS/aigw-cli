package diagnostic

import "net/http"

// Credential carries the Provider-owned values that an Account diagnostic
// reads, independently of the Account's inference API Token.
type Credential struct {
	SystemToken string
	UserID      string
}

// Sender sends one request and returns its response without following a
// redirect, so that a Credential never reaches another origin.
type Sender interface {
	Send(request *http.Request) (*http.Response, error)
}
