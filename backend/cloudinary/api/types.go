// Package api has type definitions for cloudinary
package api

import (
	"fmt"
)

// CloudinaryEncoder extends the built-in encoder
type CloudinaryEncoder interface {
	// FromStandardPath takes a / separated path in Standard encoding
	// and converts it to a / separated path in this encoding.
	FromStandardPath(string) string
	// FromStandardName takes name in Standard encoding and converts
	// it in this encoding.
	FromStandardName(string) string
	// ToStandardPath takes a / separated path in this encoding
	// and converts it to a / separated path in Standard encoding.
	ToStandardPath(string) string
	// ToStandardName takes name in this encoding and converts
	// it in Standard encoding.
	ToStandardName(string, string) string
	// Encoded root of the remote (as passed into NewFs)
	FromStandardFullPath(string) string
}

// UpdateOptions was created to pass options from Update to Put
type UpdateOptions struct {
	PublicID     string
	ResourceType string
	DeliveryType string
	AssetFolder  string
	DisplayName  string
}

// Header formats the option as a string
func (o *UpdateOptions) Header() (string, string) {
	return "UpdateOption", fmt.Sprintf("%s/%s/%s", o.ResourceType, o.DeliveryType, o.PublicID)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *UpdateOptions) Mandatory() bool {
	return false
}

// String formats the option into human-readable form
func (o *UpdateOptions) String() string {
	return fmt.Sprintf("Fully qualified Public ID: %s/%s/%s", o.ResourceType, o.DeliveryType, o.PublicID)
}
