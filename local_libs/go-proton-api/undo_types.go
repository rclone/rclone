package proton

import "errors"

var ErrUndoTokenExpired = errors.New("undo token expired")

type UndoToken struct {
	Token      string
	ValidUntil int64
}

type UndoRes struct {
	Messages []Message
}
