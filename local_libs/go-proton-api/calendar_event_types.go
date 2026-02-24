package proton

import (
	"encoding/base64"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type CalendarEvent struct {
	ID            string
	UID           string
	CalendarID    string
	SharedEventID string

	CreateTime    int64
	LastEditTime  int64
	StartTime     int64
	StartTimezone string
	EndTime       int64
	EndTimezone   string
	FullDay       Bool

	Author      string
	Permissions CalendarPermissions
	Attendees   []CalendarAttendee

	SharedKeyPacket   string
	CalendarKeyPacket string

	SharedEvents    []CalendarEventPart
	CalendarEvents  []CalendarEventPart
	AttendeesEvents []CalendarEventPart
	PersonalEvents  []CalendarEventPart
}

// TODO: Only personal events have MemberID; should we have a different type for that?
type CalendarEventPart struct {
	MemberID string

	Type      CalendarEventType
	Data      string
	Signature string
	Author    string
}

func (part CalendarEventPart) Decode(calKR *crypto.KeyRing, addrKR *crypto.KeyRing, kp []byte) error {
	if part.Type&CalendarEventTypeEncrypted != 0 {
		var enc *crypto.PGPMessage

		if kp != nil {
			raw, err := base64.StdEncoding.DecodeString(part.Data)
			if err != nil {
				return err
			}

			enc = crypto.NewPGPSplitMessage(kp, raw).GetPGPMessage()
		} else {
			var err error

			if enc, err = crypto.NewPGPMessageFromArmored(part.Data); err != nil {
				return err
			}
		}

		dec, err := calKR.Decrypt(enc, nil, crypto.GetUnixTime())
		if err != nil {
			return err
		}

		part.Data = dec.GetString()
	}

	if part.Type&CalendarEventTypeSigned != 0 {
		sig, err := crypto.NewPGPSignatureFromArmored(part.Signature)
		if err != nil {
			return err
		}

		if err := addrKR.VerifyDetached(crypto.NewPlainMessageFromString(part.Data), sig, crypto.GetUnixTime()); err != nil {
			return err
		}
	}

	return nil
}

type CalendarEventType int

const (
	CalendarEventTypeClear CalendarEventType = iota
	CalendarEventTypeEncrypted
	CalendarEventTypeSigned
)

type CalendarAttendee struct {
	ID          string
	Token       string
	Status      CalendarAttendeeStatus
	Permissions CalendarPermissions
}

// TODO: What is this?
type CalendarAttendeeStatus int

const (
	CalendarAttendeeStatusPending CalendarAttendeeStatus = iota
	CalendarAttendeeStatusMaybe
	CalendarAttendeeStatusNo
	CalendarAttendeeStatusYes
)
