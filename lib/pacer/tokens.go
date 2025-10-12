// Tokens for controlling concurrency

package pacer

// TokenDispenser is for controlling concurrency
type TokenDispenser struct {
	tokens chan struct{}
}

// NewTokenDispenser makes a pool of n tokens
func NewTokenDispenser(n int) *TokenDispenser {
	td := &TokenDispenser{
		tokens: make(chan struct{}, n),
	}
	// Fill up the upload tokens
	for range n {
		td.tokens <- struct{}{}
	}
	return td
}

// Get gets a token from the pool - don't forget to return it with Put
func (td *TokenDispenser) Get() {
	<-td.tokens
}

// Put returns a token
func (td *TokenDispenser) Put() {
	td.tokens <- struct{}{}
}
