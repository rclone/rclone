package server

import (
	"fmt"

	"github.com/google/uuid"
)

func newMessageLiteral(from, to string) []byte {
	return []byte(fmt.Sprintf("From: %v\r\nReceiver: %v\r\nSubject: %v\r\n\r\nHello World!", from, to, uuid.New()))
}

func newMessageLiteralWithSubject(from, to, subject string) []byte {
	return []byte(fmt.Sprintf("From: %v\r\nReceiver: %v\r\nSubject: %v\r\n\r\nHello World!", from, to, subject))
}
