// +-------------------------------------------------------------------------
// | Copyright (C) 2016 Yunify, Inc.
// +-------------------------------------------------------------------------
// | Licensed under the Apache License, Version 2.0 (the "License");
// | you may not use this work except in compliance with the License.
// | You may obtain a copy of the License in the LICENSE file, or at:
// |
// | http://www.apache.org/licenses/LICENSE-2.0
// |
// | Unless required by applicable law or agreed to in writing, software
// | distributed under the License is distributed on an "AS IS" BASIS,
// | WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// | See the License for the specific language governing permissions and
// | limitations under the License.
// +-------------------------------------------------------------------------

package service

import (
	"fmt"
	"time"

	"github.com/yunify/qingstor-sdk-go/v3/request/errors"
)

// Properties presents the service properties.
type Properties struct {
	// Bucket name
	BucketName *string `json:"bucket-name" name:"bucket-name"` // Required
	// Object key
	ObjectKey *string `json:"object-key" name:"object-key"` // Required
	// QingCloud Zone ID
	Zone *string `json:"zone" name:"zone"`
}

// AbortIncompleteMultipartUploadType presents AbortIncompleteMultipartUpload.
type AbortIncompleteMultipartUploadType struct {
	// days after initiation
	DaysAfterInitiation *int `json:"days_after_initiation" name:"days_after_initiation"` // Required

}

// Validate validates the AbortIncompleteMultipartUpload.
func (v *AbortIncompleteMultipartUploadType) Validate() error {

	if v.DaysAfterInitiation == nil {
		return errors.ParameterRequiredError{
			ParameterName: "DaysAfterInitiation",
			ParentName:    "AbortIncompleteMultipartUpload",
		}
	}

	return nil
}

// ACLType presents ACL.
type ACLType struct {
	Grantee *GranteeType `json:"grantee" name:"grantee"` // Required
	// Permission for this grantee
	// Permission's available values: READ, WRITE, FULL_CONTROL
	Permission *string `json:"permission" name:"permission"` // Required

}

// Validate validates the ACL.
func (v *ACLType) Validate() error {

	if v.Grantee != nil {
		if err := v.Grantee.Validate(); err != nil {
			return err
		}
	}

	if v.Grantee == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Grantee",
			ParentName:    "ACL",
		}
	}

	if v.Permission == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Permission",
			ParentName:    "ACL",
		}
	}

	if v.Permission != nil {
		permissionValidValues := []string{"READ", "WRITE", "FULL_CONTROL"}
		permissionParameterValue := fmt.Sprint(*v.Permission)

		permissionIsValid := false
		for _, value := range permissionValidValues {
			if value == permissionParameterValue {
				permissionIsValid = true
			}
		}

		if !permissionIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "Permission",
				ParameterValue: permissionParameterValue,
				AllowedValues:  permissionValidValues,
			}
		}
	}

	return nil
}

// BucketType presents Bucket.
type BucketType struct {
	// Created time of the bucket
	Created *time.Time `json:"created,omitempty" name:"created" format:"ISO 8601"`
	// QingCloud Zone ID
	Location *string `json:"location,omitempty" name:"location"`
	// Bucket name
	Name *string `json:"name,omitempty" name:"name"`
	// URL to access the bucket
	URL *string `json:"url,omitempty" name:"url"`
}

// Validate validates the Bucket.
func (v *BucketType) Validate() error {

	return nil
}

// CloudfuncArgsType presents CloudfuncArgs.
type CloudfuncArgsType struct {
	Action     *string `json:"action" name:"action"` // Required
	KeyPrefix  *string `json:"key_prefix,omitempty" name:"key_prefix" default:"gen"`
	KeySeprate *string `json:"key_seprate,omitempty" name:"key_seprate" default:"_"`
	SaveBucket *string `json:"save_bucket,omitempty" name:"save_bucket"`
}

// Validate validates the CloudfuncArgs.
func (v *CloudfuncArgsType) Validate() error {

	if v.Action == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Action",
			ParentName:    "CloudfuncArgs",
		}
	}

	return nil
}

// ConditionType presents Condition.
type ConditionType struct {
	IPAddress     *IPAddressType     `json:"ip_address,omitempty" name:"ip_address"`
	IsNull        *IsNullType        `json:"is_null,omitempty" name:"is_null"`
	NotIPAddress  *NotIPAddressType  `json:"not_ip_address,omitempty" name:"not_ip_address"`
	StringLike    *StringLikeType    `json:"string_like,omitempty" name:"string_like"`
	StringNotLike *StringNotLikeType `json:"string_not_like,omitempty" name:"string_not_like"`
}

// Validate validates the Condition.
func (v *ConditionType) Validate() error {

	if v.IPAddress != nil {
		if err := v.IPAddress.Validate(); err != nil {
			return err
		}
	}

	if v.IsNull != nil {
		if err := v.IsNull.Validate(); err != nil {
			return err
		}
	}

	if v.NotIPAddress != nil {
		if err := v.NotIPAddress.Validate(); err != nil {
			return err
		}
	}

	if v.StringLike != nil {
		if err := v.StringLike.Validate(); err != nil {
			return err
		}
	}

	if v.StringNotLike != nil {
		if err := v.StringNotLike.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// CORSRuleType presents CORSRule.
type CORSRuleType struct {
	// Allowed headers
	AllowedHeaders []*string `json:"allowed_headers,omitempty" name:"allowed_headers"`
	// Allowed methods
	AllowedMethods []*string `json:"allowed_methods" name:"allowed_methods"` // Required
	// Allowed origin
	AllowedOrigin *string `json:"allowed_origin" name:"allowed_origin"` // Required
	// Expose headers
	ExposeHeaders []*string `json:"expose_headers,omitempty" name:"expose_headers"`
	// Max age seconds
	MaxAgeSeconds *int `json:"max_age_seconds,omitempty" name:"max_age_seconds"`
}

// Validate validates the CORSRule.
func (v *CORSRuleType) Validate() error {

	if len(v.AllowedMethods) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "AllowedMethods",
			ParentName:    "CORSRule",
		}
	}

	if v.AllowedOrigin == nil {
		return errors.ParameterRequiredError{
			ParameterName: "AllowedOrigin",
			ParentName:    "CORSRule",
		}
	}

	return nil
}

// ExpirationType presents Expiration.
type ExpirationType struct {
	// days
	Days *int `json:"days,omitempty" name:"days"`
}

// Validate validates the Expiration.
func (v *ExpirationType) Validate() error {

	return nil
}

// FilterType presents Filter.
type FilterType struct {
	// Prefix matching
	Prefix *string `json:"prefix" name:"prefix"` // Required

}

// Validate validates the Filter.
func (v *FilterType) Validate() error {

	if v.Prefix == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Prefix",
			ParentName:    "Filter",
		}
	}

	return nil
}

// GranteeType presents Grantee.
type GranteeType struct {
	// Grantee user ID
	ID *string `json:"id,omitempty" name:"id"`
	// Grantee group name
	Name *string `json:"name,omitempty" name:"name"`
	// Grantee type
	// Type's available values: user, group
	Type *string `json:"type" name:"type"` // Required

}

// Validate validates the Grantee.
func (v *GranteeType) Validate() error {

	if v.Type == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Type",
			ParentName:    "Grantee",
		}
	}

	if v.Type != nil {
		typeValidValues := []string{"user", "group"}
		typeParameterValue := fmt.Sprint(*v.Type)

		typeIsValid := false
		for _, value := range typeValidValues {
			if value == typeParameterValue {
				typeIsValid = true
			}
		}

		if !typeIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "Type",
				ParameterValue: typeParameterValue,
				AllowedValues:  typeValidValues,
			}
		}
	}

	return nil
}

// IPAddressType presents IPAddress.
type IPAddressType struct {
	// Source IP
	SourceIP []*string `json:"source_ip,omitempty" name:"source_ip"`
}

// Validate validates the IPAddress.
func (v *IPAddressType) Validate() error {

	return nil
}

// IsNullType presents IsNull.
type IsNullType struct {
	// Refer url
	Referer *bool `json:"Referer,omitempty" name:"Referer"`
}

// Validate validates the IsNull.
func (v *IsNullType) Validate() error {

	return nil
}

// KeyType presents Key.
type KeyType struct {
	// Object created time
	Created *time.Time `json:"created,omitempty" name:"created" format:"ISO 8601"`
	// Whether this key is encrypted
	Encrypted *bool `json:"encrypted,omitempty" name:"encrypted"`
	// MD5sum of the object
	Etag *string `json:"etag,omitempty" name:"etag"`
	// Object key
	Key *string `json:"key,omitempty" name:"key"`
	// MIME type of the object
	MimeType *string `json:"mime_type,omitempty" name:"mime_type"`
	// Last modified time in unix time format
	Modified *int `json:"modified,omitempty" name:"modified"`
	// Object content size
	Size *int64 `json:"size,omitempty" name:"size"`
	// Object storage class
	StorageClass *string `json:"storage_class,omitempty" name:"storage_class"`
}

// Validate validates the Key.
func (v *KeyType) Validate() error {

	return nil
}

// KeyDeleteErrorType presents KeyDeleteError.
type KeyDeleteErrorType struct {
	// Error code
	Code *string `json:"code,omitempty" name:"code"`
	// Object key
	Key *string `json:"key,omitempty" name:"key"`
	// Error message
	Message *string `json:"message,omitempty" name:"message"`
}

// Validate validates the KeyDeleteError.
func (v *KeyDeleteErrorType) Validate() error {

	return nil
}

// NotIPAddressType presents NotIPAddress.
type NotIPAddressType struct {
	// Source IP
	SourceIP []*string `json:"source_ip,omitempty" name:"source_ip"`
}

// Validate validates the NotIPAddress.
func (v *NotIPAddressType) Validate() error {

	return nil
}

// NotificationType presents Notification.
type NotificationType struct {
	// Event processing service
	// Cloudfunc's available values: tupu-porn, notifier, image
	Cloudfunc     *string            `json:"cloudfunc" name:"cloudfunc"` // Required
	CloudfuncArgs *CloudfuncArgsType `json:"cloudfunc_args,omitempty" name:"cloudfunc_args"`
	// event types
	EventTypes []*string `json:"event_types" name:"event_types"` // Required
	// notification id
	ID *string `json:"id" name:"id"` // Required
	// notify url
	NotifyURL *string `json:"notify_url,omitempty" name:"notify_url"`
	// Object name matching rule
	ObjectFilters []*string `json:"object_filters,omitempty" name:"object_filters"`
}

// Validate validates the Notification.
func (v *NotificationType) Validate() error {

	if v.Cloudfunc == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Cloudfunc",
			ParentName:    "Notification",
		}
	}

	if v.Cloudfunc != nil {
		cloudfuncValidValues := []string{"tupu-porn", "notifier", "image"}
		cloudfuncParameterValue := fmt.Sprint(*v.Cloudfunc)

		cloudfuncIsValid := false
		for _, value := range cloudfuncValidValues {
			if value == cloudfuncParameterValue {
				cloudfuncIsValid = true
			}
		}

		if !cloudfuncIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "Cloudfunc",
				ParameterValue: cloudfuncParameterValue,
				AllowedValues:  cloudfuncValidValues,
			}
		}
	}

	if v.CloudfuncArgs != nil {
		if err := v.CloudfuncArgs.Validate(); err != nil {
			return err
		}
	}

	if len(v.EventTypes) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "EventTypes",
			ParentName:    "Notification",
		}
	}

	if v.ID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "ID",
			ParentName:    "Notification",
		}
	}

	return nil
}

// ObjectPartType presents ObjectPart.
type ObjectPartType struct {
	// Object part created time
	Created *time.Time `json:"created,omitempty" name:"created" format:"ISO 8601"`
	// MD5sum of the object part
	Etag *string `json:"etag,omitempty" name:"etag"`
	// Object part number
	PartNumber *int `json:"part_number" name:"part_number" default:"0"` // Required
	// Object part size
	Size *int64 `json:"size,omitempty" name:"size"`
}

// Validate validates the ObjectPart.
func (v *ObjectPartType) Validate() error {

	if v.PartNumber == nil {
		return errors.ParameterRequiredError{
			ParameterName: "PartNumber",
			ParentName:    "ObjectPart",
		}
	}

	return nil
}

// OwnerType presents Owner.
type OwnerType struct {
	// User ID
	ID *string `json:"id,omitempty" name:"id"`
	// Username
	Name *string `json:"name,omitempty" name:"name"`
}

// Validate validates the Owner.
func (v *OwnerType) Validate() error {

	return nil
}

// RuleType presents Rule.
type RuleType struct {
	AbortIncompleteMultipartUpload *AbortIncompleteMultipartUploadType `json:"abort_incomplete_multipart_upload,omitempty" name:"abort_incomplete_multipart_upload"`
	Expiration                     *ExpirationType                     `json:"expiration,omitempty" name:"expiration"`
	Filter                         *FilterType                         `json:"filter" name:"filter"` // Required
	// rule id
	ID *string `json:"id" name:"id"` // Required
	// rule status
	// Status's available values: enabled, disabled
	Status     *string         `json:"status" name:"status"` // Required
	Transition *TransitionType `json:"transition,omitempty" name:"transition"`
}

// Validate validates the Rule.
func (v *RuleType) Validate() error {

	if v.AbortIncompleteMultipartUpload != nil {
		if err := v.AbortIncompleteMultipartUpload.Validate(); err != nil {
			return err
		}
	}

	if v.Expiration != nil {
		if err := v.Expiration.Validate(); err != nil {
			return err
		}
	}

	if v.Filter != nil {
		if err := v.Filter.Validate(); err != nil {
			return err
		}
	}

	if v.Filter == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Filter",
			ParentName:    "Rule",
		}
	}

	if v.ID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "ID",
			ParentName:    "Rule",
		}
	}

	if v.Status == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Status",
			ParentName:    "Rule",
		}
	}

	if v.Status != nil {
		statusValidValues := []string{"enabled", "disabled"}
		statusParameterValue := fmt.Sprint(*v.Status)

		statusIsValid := false
		for _, value := range statusValidValues {
			if value == statusParameterValue {
				statusIsValid = true
			}
		}

		if !statusIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "Status",
				ParameterValue: statusParameterValue,
				AllowedValues:  statusValidValues,
			}
		}
	}

	if v.Transition != nil {
		if err := v.Transition.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// StatementType presents Statement.
type StatementType struct {
	// QingStor API methods
	Action    []*string      `json:"action" name:"action"` // Required
	Condition *ConditionType `json:"condition,omitempty" name:"condition"`
	// Statement effect
	// Effect's available values: allow, deny
	Effect *string `json:"effect" name:"effect"` // Required
	// Bucket policy id, must be unique
	ID *string `json:"id" name:"id"` // Required
	// The resources to apply bucket policy
	Resource []*string `json:"resource,omitempty" name:"resource"`
	// The user to apply bucket policy
	User []*string `json:"user" name:"user"` // Required

}

// Validate validates the Statement.
func (v *StatementType) Validate() error {

	if len(v.Action) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "Action",
			ParentName:    "Statement",
		}
	}

	if v.Condition != nil {
		if err := v.Condition.Validate(); err != nil {
			return err
		}
	}

	if v.Effect == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Effect",
			ParentName:    "Statement",
		}
	}

	if v.Effect != nil {
		effectValidValues := []string{"allow", "deny"}
		effectParameterValue := fmt.Sprint(*v.Effect)

		effectIsValid := false
		for _, value := range effectValidValues {
			if value == effectParameterValue {
				effectIsValid = true
			}
		}

		if !effectIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "Effect",
				ParameterValue: effectParameterValue,
				AllowedValues:  effectValidValues,
			}
		}
	}

	if v.ID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "ID",
			ParentName:    "Statement",
		}
	}

	if len(v.User) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "User",
			ParentName:    "Statement",
		}
	}

	return nil
}

// StringLikeType presents StringLike.
type StringLikeType struct {
	// Refer url
	Referer []*string `json:"Referer,omitempty" name:"Referer"`
}

// Validate validates the StringLike.
func (v *StringLikeType) Validate() error {

	return nil
}

// StringNotLikeType presents StringNotLike.
type StringNotLikeType struct {
	// Refer url
	Referer []*string `json:"Referer,omitempty" name:"Referer"`
}

// Validate validates the StringNotLike.
func (v *StringNotLikeType) Validate() error {

	return nil
}

// TransitionType presents Transition.
type TransitionType struct {
	// days
	Days *int `json:"days,omitempty" name:"days"`
	// storage class
	StorageClass *int `json:"storage_class" name:"storage_class"` // Required

}

// Validate validates the Transition.
func (v *TransitionType) Validate() error {

	if v.StorageClass == nil {
		return errors.ParameterRequiredError{
			ParameterName: "StorageClass",
			ParentName:    "Transition",
		}
	}

	return nil
}

// UploadsType presents Uploads.
type UploadsType struct {
	// Object part created time
	Created *time.Time `json:"created,omitempty" name:"created" format:"ISO 8601"`
	// Object key
	Key *string `json:"key,omitempty" name:"key"`
	// Object upload id
	UploadID *string `json:"upload_id,omitempty" name:"upload_id"`
}

// Validate validates the Uploads.
func (v *UploadsType) Validate() error {

	return nil
}
