package backend

import (
	"github.com/bradenaw/juniper/xslices"
	"github.com/rclone/go-proton-api"
)

func merge(updates []update) []update {
	if len(updates) < 2 {
		return updates
	}

	if merged := merge(updates[1:]); xslices.IndexFunc(merged, func(other update) bool {
		return other.replaces(updates[0])
	}) < 0 {
		return append([]update{updates[0]}, merged...)
	} else {
		return merged
	}
}

type update interface {
	replaces(other update) bool

	_isUpdate()
}

type baseUpdate struct{}

func (baseUpdate) replaces(update) bool {
	return false
}

func (baseUpdate) _isUpdate() {}

type userRefreshed struct {
	baseUpdate

	refresh proton.RefreshFlag
}

type messageCreated struct {
	baseUpdate
	messageID string
}

type messageUpdated struct {
	baseUpdate
	messageID string
}

func (update *messageUpdated) replaces(other update) bool {
	switch other := other.(type) {
	case *messageUpdated:
		return update.messageID == other.messageID

	default:
		return false
	}
}

type messageDeleted struct {
	baseUpdate
	messageID string
}

func (update *messageDeleted) replaces(other update) bool {
	switch other := other.(type) {
	case *messageCreated:
		return update.messageID == other.messageID

	case *messageUpdated:
		return update.messageID == other.messageID

	case *messageDeleted:
		if update.messageID != other.messageID {
			return false
		}

		panic("message deleted twice")

	default:
		return false
	}
}

type labelCreated struct {
	baseUpdate
	labelID string
}

type labelUpdated struct {
	baseUpdate
	labelID string
}

func (update *labelUpdated) replaces(other update) bool {
	switch other := other.(type) {
	case *labelUpdated:
		return update.labelID == other.labelID

	default:
		return false
	}
}

type labelDeleted struct {
	baseUpdate
	labelID string
}

func (update *labelDeleted) replaces(other update) bool {
	switch other := other.(type) {
	case *labelCreated:
		return update.labelID == other.labelID

	case *labelUpdated:
		return update.labelID == other.labelID

	case *labelDeleted:
		if update.labelID != other.labelID {
			return false
		}

		panic("label deleted twice")

	default:
		return false
	}
}

type addressCreated struct {
	baseUpdate
	addressID string
}

type addressUpdated struct {
	baseUpdate
	addressID string
}

func (update *addressUpdated) replaces(other update) bool {
	switch other := other.(type) {
	case *addressUpdated:
		return update.addressID == other.addressID

	default:
		return false
	}
}

type addressDeleted struct {
	baseUpdate
	addressID string
}

func (update *addressDeleted) replaces(other update) bool {
	switch other := other.(type) {
	case *addressCreated:
		return update.addressID == other.addressID

	case *addressUpdated:
		return update.addressID == other.addressID

	case *addressDeleted:
		if update.addressID != other.addressID {
			return false
		}

		panic("address deleted twice")

	default:
		return false
	}
}

type userSettingsUpdate struct {
	baseUpdate
	settings proton.UserSettings
}

func (update *userSettingsUpdate) replaces(other update) bool {
	switch other.(type) {
	case *userSettingsUpdate:
		return true
	default:
		return false
	}
}
