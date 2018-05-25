// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package file_properties : This namespace contains helpers for property and
// template metadata endpoints.  These endpoints enable you to tag arbitrary
// key/value data to Dropbox files.  The most basic unit in this namespace is
// the `PropertyField`. These fields encapsulate the actual key/value data.
// Fields are added to a Dropbox file using a `PropertyGroup`. Property groups
// contain a reference to a Dropbox file and a `PropertyGroupTemplate`. Property
// groups are uniquely identified by the combination of their associated Dropbox
// file and template.  The `PropertyGroupTemplate` is a way of restricting the
// possible key names and value types of the data within a property group. The
// possible key names and value types are explicitly enumerated using
// `PropertyFieldTemplate` objects.  You can think of a property group template
// as a class definition for a particular key/value metadata object, and the
// property groups themselves as the instantiations of these objects.  Templates
// are owned either by a user/app pair or team/app pair. Templates and their
// associated properties can't be accessed by any app other than the app that
// created them, and even then, only when the app is linked with the owner of
// the template (either a user or team).  User-owned templates are accessed via
// the user-auth file_properties/templates/*_for_user endpoints, while
// team-owned templates are accessed via the team-auth
// file_properties/templates/*_for_team endpoints. Properties associated with
// either type of template can be accessed via the user-auth properties/*
// endpoints.  Finally, properties can be accessed from a number of endpoints
// that return metadata, including `files/get_metadata`, and
// `files/list_folder`. Properties can also be added during upload, using
// `files/upload`.
package file_properties

import (
	"encoding/json"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// AddPropertiesArg : has no documentation (yet)
type AddPropertiesArg struct {
	// Path : A unique identifier for the file or folder.
	Path string `json:"path"`
	// PropertyGroups : The property groups which are to be added to a Dropbox
	// file.
	PropertyGroups []*PropertyGroup `json:"property_groups"`
}

// NewAddPropertiesArg returns a new AddPropertiesArg instance
func NewAddPropertiesArg(Path string, PropertyGroups []*PropertyGroup) *AddPropertiesArg {
	s := new(AddPropertiesArg)
	s.Path = Path
	s.PropertyGroups = PropertyGroups
	return s
}

// TemplateError : has no documentation (yet)
type TemplateError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
}

// Valid tag values for TemplateError
const (
	TemplateErrorTemplateNotFound  = "template_not_found"
	TemplateErrorRestrictedContent = "restricted_content"
	TemplateErrorOther             = "other"
)

// UnmarshalJSON deserializes into a TemplateError instance
func (u *TemplateError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	}
	return nil
}

// PropertiesError : has no documentation (yet)
type PropertiesError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for PropertiesError
const (
	PropertiesErrorTemplateNotFound  = "template_not_found"
	PropertiesErrorRestrictedContent = "restricted_content"
	PropertiesErrorOther             = "other"
	PropertiesErrorPath              = "path"
	PropertiesErrorUnsupportedFolder = "unsupported_folder"
)

// UnmarshalJSON deserializes into a PropertiesError instance
func (u *PropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// InvalidPropertyGroupError : has no documentation (yet)
type InvalidPropertyGroupError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for InvalidPropertyGroupError
const (
	InvalidPropertyGroupErrorTemplateNotFound      = "template_not_found"
	InvalidPropertyGroupErrorRestrictedContent     = "restricted_content"
	InvalidPropertyGroupErrorOther                 = "other"
	InvalidPropertyGroupErrorPath                  = "path"
	InvalidPropertyGroupErrorUnsupportedFolder     = "unsupported_folder"
	InvalidPropertyGroupErrorPropertyFieldTooLarge = "property_field_too_large"
	InvalidPropertyGroupErrorDoesNotFitTemplate    = "does_not_fit_template"
)

// UnmarshalJSON deserializes into a InvalidPropertyGroupError instance
func (u *InvalidPropertyGroupError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// AddPropertiesError : has no documentation (yet)
type AddPropertiesError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for AddPropertiesError
const (
	AddPropertiesErrorTemplateNotFound           = "template_not_found"
	AddPropertiesErrorRestrictedContent          = "restricted_content"
	AddPropertiesErrorOther                      = "other"
	AddPropertiesErrorPath                       = "path"
	AddPropertiesErrorUnsupportedFolder          = "unsupported_folder"
	AddPropertiesErrorPropertyFieldTooLarge      = "property_field_too_large"
	AddPropertiesErrorDoesNotFitTemplate         = "does_not_fit_template"
	AddPropertiesErrorPropertyGroupAlreadyExists = "property_group_already_exists"
)

// UnmarshalJSON deserializes into a AddPropertiesError instance
func (u *AddPropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// PropertyGroupTemplate : Defines how a property group may be structured.
type PropertyGroupTemplate struct {
	// Name : Display name for the template. Template names can be up to 256
	// bytes.
	Name string `json:"name"`
	// Description : Description for the template. Template descriptions can be
	// up to 1024 bytes.
	Description string `json:"description"`
	// Fields : Definitions of the property fields associated with this
	// template. There can be up to 32 properties in a single template.
	Fields []*PropertyFieldTemplate `json:"fields"`
}

// NewPropertyGroupTemplate returns a new PropertyGroupTemplate instance
func NewPropertyGroupTemplate(Name string, Description string, Fields []*PropertyFieldTemplate) *PropertyGroupTemplate {
	s := new(PropertyGroupTemplate)
	s.Name = Name
	s.Description = Description
	s.Fields = Fields
	return s
}

// AddTemplateArg : has no documentation (yet)
type AddTemplateArg struct {
	PropertyGroupTemplate
}

// NewAddTemplateArg returns a new AddTemplateArg instance
func NewAddTemplateArg(Name string, Description string, Fields []*PropertyFieldTemplate) *AddTemplateArg {
	s := new(AddTemplateArg)
	s.Name = Name
	s.Description = Description
	s.Fields = Fields
	return s
}

// AddTemplateResult : has no documentation (yet)
type AddTemplateResult struct {
	// TemplateId : An identifier for template added by  See
	// `templatesAddForUser` or `templatesAddForTeam`.
	TemplateId string `json:"template_id"`
}

// NewAddTemplateResult returns a new AddTemplateResult instance
func NewAddTemplateResult(TemplateId string) *AddTemplateResult {
	s := new(AddTemplateResult)
	s.TemplateId = TemplateId
	return s
}

// GetTemplateArg : has no documentation (yet)
type GetTemplateArg struct {
	// TemplateId : An identifier for template added by route  See
	// `templatesAddForUser` or `templatesAddForTeam`.
	TemplateId string `json:"template_id"`
}

// NewGetTemplateArg returns a new GetTemplateArg instance
func NewGetTemplateArg(TemplateId string) *GetTemplateArg {
	s := new(GetTemplateArg)
	s.TemplateId = TemplateId
	return s
}

// GetTemplateResult : has no documentation (yet)
type GetTemplateResult struct {
	PropertyGroupTemplate
}

// NewGetTemplateResult returns a new GetTemplateResult instance
func NewGetTemplateResult(Name string, Description string, Fields []*PropertyFieldTemplate) *GetTemplateResult {
	s := new(GetTemplateResult)
	s.Name = Name
	s.Description = Description
	s.Fields = Fields
	return s
}

// ListTemplateResult : has no documentation (yet)
type ListTemplateResult struct {
	// TemplateIds : List of identifiers for templates added by  See
	// `templatesAddForUser` or `templatesAddForTeam`.
	TemplateIds []string `json:"template_ids"`
}

// NewListTemplateResult returns a new ListTemplateResult instance
func NewListTemplateResult(TemplateIds []string) *ListTemplateResult {
	s := new(ListTemplateResult)
	s.TemplateIds = TemplateIds
	return s
}

// LogicalOperator : Logical operator to join search queries together.
type LogicalOperator struct {
	dropbox.Tagged
}

// Valid tag values for LogicalOperator
const (
	LogicalOperatorOrOperator = "or_operator"
	LogicalOperatorOther      = "other"
)

// LookUpPropertiesError : has no documentation (yet)
type LookUpPropertiesError struct {
	dropbox.Tagged
}

// Valid tag values for LookUpPropertiesError
const (
	LookUpPropertiesErrorPropertyGroupNotFound = "property_group_not_found"
	LookUpPropertiesErrorOther                 = "other"
)

// LookupError : has no documentation (yet)
type LookupError struct {
	dropbox.Tagged
	// MalformedPath : has no documentation (yet)
	MalformedPath string `json:"malformed_path,omitempty"`
}

// Valid tag values for LookupError
const (
	LookupErrorMalformedPath     = "malformed_path"
	LookupErrorNotFound          = "not_found"
	LookupErrorNotFile           = "not_file"
	LookupErrorNotFolder         = "not_folder"
	LookupErrorRestrictedContent = "restricted_content"
	LookupErrorOther             = "other"
)

// UnmarshalJSON deserializes into a LookupError instance
func (u *LookupError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "malformed_path":
		err = json.Unmarshal(body, &u.MalformedPath)

		if err != nil {
			return err
		}
	}
	return nil
}

// ModifyTemplateError : has no documentation (yet)
type ModifyTemplateError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
}

// Valid tag values for ModifyTemplateError
const (
	ModifyTemplateErrorTemplateNotFound          = "template_not_found"
	ModifyTemplateErrorRestrictedContent         = "restricted_content"
	ModifyTemplateErrorOther                     = "other"
	ModifyTemplateErrorConflictingPropertyNames  = "conflicting_property_names"
	ModifyTemplateErrorTooManyProperties         = "too_many_properties"
	ModifyTemplateErrorTooManyTemplates          = "too_many_templates"
	ModifyTemplateErrorTemplateAttributeTooLarge = "template_attribute_too_large"
)

// UnmarshalJSON deserializes into a ModifyTemplateError instance
func (u *ModifyTemplateError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	}
	return nil
}

// OverwritePropertyGroupArg : has no documentation (yet)
type OverwritePropertyGroupArg struct {
	// Path : A unique identifier for the file or folder.
	Path string `json:"path"`
	// PropertyGroups : The property groups "snapshot" updates to force apply.
	PropertyGroups []*PropertyGroup `json:"property_groups"`
}

// NewOverwritePropertyGroupArg returns a new OverwritePropertyGroupArg instance
func NewOverwritePropertyGroupArg(Path string, PropertyGroups []*PropertyGroup) *OverwritePropertyGroupArg {
	s := new(OverwritePropertyGroupArg)
	s.Path = Path
	s.PropertyGroups = PropertyGroups
	return s
}

// PropertiesSearchArg : has no documentation (yet)
type PropertiesSearchArg struct {
	// Queries : Queries to search.
	Queries []*PropertiesSearchQuery `json:"queries"`
	// TemplateFilter : Filter results to contain only properties associated
	// with these template IDs.
	TemplateFilter *TemplateFilter `json:"template_filter"`
}

// NewPropertiesSearchArg returns a new PropertiesSearchArg instance
func NewPropertiesSearchArg(Queries []*PropertiesSearchQuery) *PropertiesSearchArg {
	s := new(PropertiesSearchArg)
	s.Queries = Queries
	s.TemplateFilter = &TemplateFilter{Tagged: dropbox.Tagged{"filter_none"}}
	return s
}

// PropertiesSearchContinueArg : has no documentation (yet)
type PropertiesSearchContinueArg struct {
	// Cursor : The cursor returned by your last call to `propertiesSearch` or
	// `propertiesSearchContinue`.
	Cursor string `json:"cursor"`
}

// NewPropertiesSearchContinueArg returns a new PropertiesSearchContinueArg instance
func NewPropertiesSearchContinueArg(Cursor string) *PropertiesSearchContinueArg {
	s := new(PropertiesSearchContinueArg)
	s.Cursor = Cursor
	return s
}

// PropertiesSearchContinueError : has no documentation (yet)
type PropertiesSearchContinueError struct {
	dropbox.Tagged
}

// Valid tag values for PropertiesSearchContinueError
const (
	PropertiesSearchContinueErrorReset = "reset"
	PropertiesSearchContinueErrorOther = "other"
)

// PropertiesSearchError : has no documentation (yet)
type PropertiesSearchError struct {
	dropbox.Tagged
	// PropertyGroupLookup : has no documentation (yet)
	PropertyGroupLookup *LookUpPropertiesError `json:"property_group_lookup,omitempty"`
}

// Valid tag values for PropertiesSearchError
const (
	PropertiesSearchErrorPropertyGroupLookup = "property_group_lookup"
	PropertiesSearchErrorOther               = "other"
)

// UnmarshalJSON deserializes into a PropertiesSearchError instance
func (u *PropertiesSearchError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PropertyGroupLookup : has no documentation (yet)
		PropertyGroupLookup json.RawMessage `json:"property_group_lookup,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "property_group_lookup":
		err = json.Unmarshal(w.PropertyGroupLookup, &u.PropertyGroupLookup)

		if err != nil {
			return err
		}
	}
	return nil
}

// PropertiesSearchMatch : has no documentation (yet)
type PropertiesSearchMatch struct {
	// Id : The ID for the matched file or folder.
	Id string `json:"id"`
	// Path : The path for the matched file or folder.
	Path string `json:"path"`
	// IsDeleted : Whether the file or folder is deleted.
	IsDeleted bool `json:"is_deleted"`
	// PropertyGroups : List of custom property groups associated with the file.
	PropertyGroups []*PropertyGroup `json:"property_groups"`
}

// NewPropertiesSearchMatch returns a new PropertiesSearchMatch instance
func NewPropertiesSearchMatch(Id string, Path string, IsDeleted bool, PropertyGroups []*PropertyGroup) *PropertiesSearchMatch {
	s := new(PropertiesSearchMatch)
	s.Id = Id
	s.Path = Path
	s.IsDeleted = IsDeleted
	s.PropertyGroups = PropertyGroups
	return s
}

// PropertiesSearchMode : has no documentation (yet)
type PropertiesSearchMode struct {
	dropbox.Tagged
	// FieldName : Search for a value associated with this field name.
	FieldName string `json:"field_name,omitempty"`
}

// Valid tag values for PropertiesSearchMode
const (
	PropertiesSearchModeFieldName = "field_name"
	PropertiesSearchModeOther     = "other"
)

// UnmarshalJSON deserializes into a PropertiesSearchMode instance
func (u *PropertiesSearchMode) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "field_name":
		err = json.Unmarshal(body, &u.FieldName)

		if err != nil {
			return err
		}
	}
	return nil
}

// PropertiesSearchQuery : has no documentation (yet)
type PropertiesSearchQuery struct {
	// Query : The property field value for which to search across templates.
	Query string `json:"query"`
	// Mode : The mode with which to perform the search.
	Mode *PropertiesSearchMode `json:"mode"`
	// LogicalOperator : The logical operator with which to append the query.
	LogicalOperator *LogicalOperator `json:"logical_operator"`
}

// NewPropertiesSearchQuery returns a new PropertiesSearchQuery instance
func NewPropertiesSearchQuery(Query string, Mode *PropertiesSearchMode) *PropertiesSearchQuery {
	s := new(PropertiesSearchQuery)
	s.Query = Query
	s.Mode = Mode
	s.LogicalOperator = &LogicalOperator{Tagged: dropbox.Tagged{"or_operator"}}
	return s
}

// PropertiesSearchResult : has no documentation (yet)
type PropertiesSearchResult struct {
	// Matches : A list (possibly empty) of matches for the query.
	Matches []*PropertiesSearchMatch `json:"matches"`
	// Cursor : Pass the cursor into `propertiesSearchContinue` to continue to
	// receive search results. Cursor will be null when there are no more
	// results.
	Cursor string `json:"cursor,omitempty"`
}

// NewPropertiesSearchResult returns a new PropertiesSearchResult instance
func NewPropertiesSearchResult(Matches []*PropertiesSearchMatch) *PropertiesSearchResult {
	s := new(PropertiesSearchResult)
	s.Matches = Matches
	return s
}

// PropertyField : Raw key/value data to be associated with a Dropbox file.
// Property fields are added to Dropbox files as a `PropertyGroup`.
type PropertyField struct {
	// Name : Key of the property field associated with a file and template.
	// Keys can be up to 256 bytes.
	Name string `json:"name"`
	// Value : Value of the property field associated with a file and template.
	// Values can be up to 1024 bytes.
	Value string `json:"value"`
}

// NewPropertyField returns a new PropertyField instance
func NewPropertyField(Name string, Value string) *PropertyField {
	s := new(PropertyField)
	s.Name = Name
	s.Value = Value
	return s
}

// PropertyFieldTemplate : Defines how a single property field may be
// structured. Used exclusively by `PropertyGroupTemplate`.
type PropertyFieldTemplate struct {
	// Name : Key of the property field being described. Property field keys can
	// be up to 256 bytes.
	Name string `json:"name"`
	// Description : Description of the property field. Property field
	// descriptions can be up to 1024 bytes.
	Description string `json:"description"`
	// Type : Data type of the value of this property field. This type will be
	// enforced upon property creation and modifications.
	Type *PropertyType `json:"type"`
}

// NewPropertyFieldTemplate returns a new PropertyFieldTemplate instance
func NewPropertyFieldTemplate(Name string, Description string, Type *PropertyType) *PropertyFieldTemplate {
	s := new(PropertyFieldTemplate)
	s.Name = Name
	s.Description = Description
	s.Type = Type
	return s
}

// PropertyGroup : A subset of the property fields described by the
// corresponding `PropertyGroupTemplate`. Properties are always added to a
// Dropbox file as a `PropertyGroup`. The possible key names and value types in
// this group are defined by the corresponding `PropertyGroupTemplate`.
type PropertyGroup struct {
	// TemplateId : A unique identifier for the associated template.
	TemplateId string `json:"template_id"`
	// Fields : The actual properties associated with the template. There can be
	// up to 32 property types per template.
	Fields []*PropertyField `json:"fields"`
}

// NewPropertyGroup returns a new PropertyGroup instance
func NewPropertyGroup(TemplateId string, Fields []*PropertyField) *PropertyGroup {
	s := new(PropertyGroup)
	s.TemplateId = TemplateId
	s.Fields = Fields
	return s
}

// PropertyGroupUpdate : has no documentation (yet)
type PropertyGroupUpdate struct {
	// TemplateId : A unique identifier for a property template.
	TemplateId string `json:"template_id"`
	// AddOrUpdateFields : Property fields to update. If the property field
	// already exists, it is updated. If the property field doesn't exist, the
	// property group is added.
	AddOrUpdateFields []*PropertyField `json:"add_or_update_fields,omitempty"`
	// RemoveFields : Property fields to remove (by name), provided they exist.
	RemoveFields []string `json:"remove_fields,omitempty"`
}

// NewPropertyGroupUpdate returns a new PropertyGroupUpdate instance
func NewPropertyGroupUpdate(TemplateId string) *PropertyGroupUpdate {
	s := new(PropertyGroupUpdate)
	s.TemplateId = TemplateId
	return s
}

// PropertyType : Data type of the given property field added.
type PropertyType struct {
	dropbox.Tagged
}

// Valid tag values for PropertyType
const (
	PropertyTypeString = "string"
	PropertyTypeOther  = "other"
)

// RemovePropertiesArg : has no documentation (yet)
type RemovePropertiesArg struct {
	// Path : A unique identifier for the file or folder.
	Path string `json:"path"`
	// PropertyTemplateIds : A list of identifiers for a template created by
	// `templatesAddForUser` or `templatesAddForTeam`.
	PropertyTemplateIds []string `json:"property_template_ids"`
}

// NewRemovePropertiesArg returns a new RemovePropertiesArg instance
func NewRemovePropertiesArg(Path string, PropertyTemplateIds []string) *RemovePropertiesArg {
	s := new(RemovePropertiesArg)
	s.Path = Path
	s.PropertyTemplateIds = PropertyTemplateIds
	return s
}

// RemovePropertiesError : has no documentation (yet)
type RemovePropertiesError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
	// PropertyGroupLookup : has no documentation (yet)
	PropertyGroupLookup *LookUpPropertiesError `json:"property_group_lookup,omitempty"`
}

// Valid tag values for RemovePropertiesError
const (
	RemovePropertiesErrorTemplateNotFound    = "template_not_found"
	RemovePropertiesErrorRestrictedContent   = "restricted_content"
	RemovePropertiesErrorOther               = "other"
	RemovePropertiesErrorPath                = "path"
	RemovePropertiesErrorUnsupportedFolder   = "unsupported_folder"
	RemovePropertiesErrorPropertyGroupLookup = "property_group_lookup"
)

// UnmarshalJSON deserializes into a RemovePropertiesError instance
func (u *RemovePropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
		// PropertyGroupLookup : has no documentation (yet)
		PropertyGroupLookup json.RawMessage `json:"property_group_lookup,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	case "property_group_lookup":
		err = json.Unmarshal(w.PropertyGroupLookup, &u.PropertyGroupLookup)

		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveTemplateArg : has no documentation (yet)
type RemoveTemplateArg struct {
	// TemplateId : An identifier for a template created by
	// `templatesAddForUser` or `templatesAddForTeam`.
	TemplateId string `json:"template_id"`
}

// NewRemoveTemplateArg returns a new RemoveTemplateArg instance
func NewRemoveTemplateArg(TemplateId string) *RemoveTemplateArg {
	s := new(RemoveTemplateArg)
	s.TemplateId = TemplateId
	return s
}

// TemplateFilterBase : has no documentation (yet)
type TemplateFilterBase struct {
	dropbox.Tagged
	// FilterSome : Only templates with an ID in the supplied list will be
	// returned (a subset of templates will be returned).
	FilterSome []string `json:"filter_some,omitempty"`
}

// Valid tag values for TemplateFilterBase
const (
	TemplateFilterBaseFilterSome = "filter_some"
	TemplateFilterBaseOther      = "other"
)

// UnmarshalJSON deserializes into a TemplateFilterBase instance
func (u *TemplateFilterBase) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// FilterSome : Only templates with an ID in the supplied list will be
		// returned (a subset of templates will be returned).
		FilterSome json.RawMessage `json:"filter_some,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "filter_some":
		err = json.Unmarshal(body, &u.FilterSome)

		if err != nil {
			return err
		}
	}
	return nil
}

// TemplateFilter : has no documentation (yet)
type TemplateFilter struct {
	dropbox.Tagged
	// FilterSome : Only templates with an ID in the supplied list will be
	// returned (a subset of templates will be returned).
	FilterSome []string `json:"filter_some,omitempty"`
}

// Valid tag values for TemplateFilter
const (
	TemplateFilterFilterSome = "filter_some"
	TemplateFilterOther      = "other"
	TemplateFilterFilterNone = "filter_none"
)

// UnmarshalJSON deserializes into a TemplateFilter instance
func (u *TemplateFilter) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// FilterSome : Only templates with an ID in the supplied list will be
		// returned (a subset of templates will be returned).
		FilterSome json.RawMessage `json:"filter_some,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "filter_some":
		err = json.Unmarshal(body, &u.FilterSome)

		if err != nil {
			return err
		}
	}
	return nil
}

// TemplateOwnerType : has no documentation (yet)
type TemplateOwnerType struct {
	dropbox.Tagged
}

// Valid tag values for TemplateOwnerType
const (
	TemplateOwnerTypeUser  = "user"
	TemplateOwnerTypeTeam  = "team"
	TemplateOwnerTypeOther = "other"
)

// UpdatePropertiesArg : has no documentation (yet)
type UpdatePropertiesArg struct {
	// Path : A unique identifier for the file or folder.
	Path string `json:"path"`
	// UpdatePropertyGroups : The property groups "delta" updates to apply.
	UpdatePropertyGroups []*PropertyGroupUpdate `json:"update_property_groups"`
}

// NewUpdatePropertiesArg returns a new UpdatePropertiesArg instance
func NewUpdatePropertiesArg(Path string, UpdatePropertyGroups []*PropertyGroupUpdate) *UpdatePropertiesArg {
	s := new(UpdatePropertiesArg)
	s.Path = Path
	s.UpdatePropertyGroups = UpdatePropertyGroups
	return s
}

// UpdatePropertiesError : has no documentation (yet)
type UpdatePropertiesError struct {
	dropbox.Tagged
	// TemplateNotFound : Template does not exist for the given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
	// PropertyGroupLookup : has no documentation (yet)
	PropertyGroupLookup *LookUpPropertiesError `json:"property_group_lookup,omitempty"`
}

// Valid tag values for UpdatePropertiesError
const (
	UpdatePropertiesErrorTemplateNotFound      = "template_not_found"
	UpdatePropertiesErrorRestrictedContent     = "restricted_content"
	UpdatePropertiesErrorOther                 = "other"
	UpdatePropertiesErrorPath                  = "path"
	UpdatePropertiesErrorUnsupportedFolder     = "unsupported_folder"
	UpdatePropertiesErrorPropertyFieldTooLarge = "property_field_too_large"
	UpdatePropertiesErrorDoesNotFitTemplate    = "does_not_fit_template"
	UpdatePropertiesErrorPropertyGroupLookup   = "property_group_lookup"
)

// UnmarshalJSON deserializes into a UpdatePropertiesError instance
func (u *UpdatePropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
		// PropertyGroupLookup : has no documentation (yet)
		PropertyGroupLookup json.RawMessage `json:"property_group_lookup,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "template_not_found":
		err = json.Unmarshal(body, &u.TemplateNotFound)

		if err != nil {
			return err
		}
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	case "property_group_lookup":
		err = json.Unmarshal(w.PropertyGroupLookup, &u.PropertyGroupLookup)

		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateTemplateArg : has no documentation (yet)
type UpdateTemplateArg struct {
	// TemplateId : An identifier for template added by  See
	// `templatesAddForUser` or `templatesAddForTeam`.
	TemplateId string `json:"template_id"`
	// Name : A display name for the template. template names can be up to 256
	// bytes.
	Name string `json:"name,omitempty"`
	// Description : Description for the new template. Template descriptions can
	// be up to 1024 bytes.
	Description string `json:"description,omitempty"`
	// AddFields : Property field templates to be added to the group template.
	// There can be up to 32 properties in a single template.
	AddFields []*PropertyFieldTemplate `json:"add_fields,omitempty"`
}

// NewUpdateTemplateArg returns a new UpdateTemplateArg instance
func NewUpdateTemplateArg(TemplateId string) *UpdateTemplateArg {
	s := new(UpdateTemplateArg)
	s.TemplateId = TemplateId
	return s
}

// UpdateTemplateResult : has no documentation (yet)
type UpdateTemplateResult struct {
	// TemplateId : An identifier for template added by route  See
	// `templatesAddForUser` or `templatesAddForTeam`.
	TemplateId string `json:"template_id"`
}

// NewUpdateTemplateResult returns a new UpdateTemplateResult instance
func NewUpdateTemplateResult(TemplateId string) *UpdateTemplateResult {
	s := new(UpdateTemplateResult)
	s.TemplateId = TemplateId
	return s
}
