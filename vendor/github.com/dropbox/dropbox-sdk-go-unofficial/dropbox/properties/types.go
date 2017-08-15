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

// Package properties : This namespace contains helper entities for property and
// property/template endpoints.
package properties

import (
	"encoding/json"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// GetPropertyTemplateArg : has no documentation (yet)
type GetPropertyTemplateArg struct {
	// TemplateId : An identifier for property template added by route
	// properties/template/add.
	TemplateId string `json:"template_id"`
}

// NewGetPropertyTemplateArg returns a new GetPropertyTemplateArg instance
func NewGetPropertyTemplateArg(TemplateId string) *GetPropertyTemplateArg {
	s := new(GetPropertyTemplateArg)
	s.TemplateId = TemplateId
	return s
}

// PropertyGroupTemplate : Describes property templates that can be filled and
// associated with a file.
type PropertyGroupTemplate struct {
	// Name : A display name for the property template. Property template names
	// can be up to 256 bytes.
	Name string `json:"name"`
	// Description : Description for new property template. Property template
	// descriptions can be up to 1024 bytes.
	Description string `json:"description"`
	// Fields : This is a list of custom properties associated with a property
	// template. There can be up to 64 properties in a single property template.
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

// GetPropertyTemplateResult : The Property template for the specified template.
type GetPropertyTemplateResult struct {
	PropertyGroupTemplate
}

// NewGetPropertyTemplateResult returns a new GetPropertyTemplateResult instance
func NewGetPropertyTemplateResult(Name string, Description string, Fields []*PropertyFieldTemplate) *GetPropertyTemplateResult {
	s := new(GetPropertyTemplateResult)
	s.Name = Name
	s.Description = Description
	s.Fields = Fields
	return s
}

// ListPropertyTemplateIds : has no documentation (yet)
type ListPropertyTemplateIds struct {
	// TemplateIds : List of identifiers for templates added by route
	// properties/template/add.
	TemplateIds []string `json:"template_ids"`
}

// NewListPropertyTemplateIds returns a new ListPropertyTemplateIds instance
func NewListPropertyTemplateIds(TemplateIds []string) *ListPropertyTemplateIds {
	s := new(ListPropertyTemplateIds)
	s.TemplateIds = TemplateIds
	return s
}

// PropertyTemplateError : has no documentation (yet)
type PropertyTemplateError struct {
	dropbox.Tagged
	// TemplateNotFound : Property template does not exist for given identifier.
	TemplateNotFound string `json:"template_not_found,omitempty"`
}

// Valid tag values for PropertyTemplateError
const (
	PropertyTemplateErrorTemplateNotFound  = "template_not_found"
	PropertyTemplateErrorRestrictedContent = "restricted_content"
	PropertyTemplateErrorOther             = "other"
)

// UnmarshalJSON deserializes into a PropertyTemplateError instance
func (u *PropertyTemplateError) UnmarshalJSON(body []byte) error {
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

// ModifyPropertyTemplateError : has no documentation (yet)
type ModifyPropertyTemplateError struct {
	dropbox.Tagged
}

// Valid tag values for ModifyPropertyTemplateError
const (
	ModifyPropertyTemplateErrorConflictingPropertyNames  = "conflicting_property_names"
	ModifyPropertyTemplateErrorTooManyProperties         = "too_many_properties"
	ModifyPropertyTemplateErrorTooManyTemplates          = "too_many_templates"
	ModifyPropertyTemplateErrorTemplateAttributeTooLarge = "template_attribute_too_large"
)

// PropertyField : has no documentation (yet)
type PropertyField struct {
	// Name : This is the name or key of a custom property in a property
	// template. File property names can be up to 256 bytes.
	Name string `json:"name"`
	// Value : Value of a custom property attached to a file. Values can be up
	// to 1024 bytes.
	Value string `json:"value"`
}

// NewPropertyField returns a new PropertyField instance
func NewPropertyField(Name string, Value string) *PropertyField {
	s := new(PropertyField)
	s.Name = Name
	s.Value = Value
	return s
}

// PropertyFieldTemplate : Describe a single property field type which that can
// be part of a property template.
type PropertyFieldTemplate struct {
	// Name : This is the name or key of a custom property in a property
	// template. File property names can be up to 256 bytes.
	Name string `json:"name"`
	// Description : This is the description for a custom property in a property
	// template. File property description can be up to 1024 bytes.
	Description string `json:"description"`
	// Type : This is the data type of the value of this property. This type
	// will be enforced upon property creation and modifications.
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

// PropertyGroup : Collection of custom properties in filled property templates.
type PropertyGroup struct {
	// TemplateId : A unique identifier for a property template type.
	TemplateId string `json:"template_id"`
	// Fields : This is a list of custom properties associated with a file.
	// There can be up to 32 properties for a template.
	Fields []*PropertyField `json:"fields"`
}

// NewPropertyGroup returns a new PropertyGroup instance
func NewPropertyGroup(TemplateId string, Fields []*PropertyField) *PropertyGroup {
	s := new(PropertyGroup)
	s.TemplateId = TemplateId
	s.Fields = Fields
	return s
}

// PropertyType : Data type of the given property added. This endpoint is in
// beta and  only properties of type strings is supported.
type PropertyType struct {
	dropbox.Tagged
}

// Valid tag values for PropertyType
const (
	PropertyTypeString = "string"
	PropertyTypeOther  = "other"
)
