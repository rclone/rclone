// Package vcard implements the vCard format, defined in RFC 6350.
package vcard

import (
	"strconv"
	"strings"
	"time"
)

// MIME type and file extension for VCard, defined in RFC 6350 section 10.1.
const (
	MIMEType  = "text/vcard"
	Extension = "vcf"
)

const timestampLayout = "20060102T150405Z"

// Card property parameters.
const (
	ParamLanguage      = "LANGUAGE"
	ParamValue         = "VALUE"
	ParamPreferred     = "PREF"
	ParamAltID         = "ALTID"
	ParamPID           = "PID"
	ParamType          = "TYPE"
	ParamMediaType     = "MEDIATYPE"
	ParamCalendarScale = "CALSCALE"
	ParamSortAs        = "SORT-AS"
	ParamGeolocation   = "GEO"
	ParamTimezone      = "TZ"
)

// Card properties.
const (
	// General Properties
	FieldSource = "SOURCE"
	FieldKind   = "KIND"
	FieldXML    = "XML"

	// Identification Properties
	FieldFormattedName = "FN"
	FieldName          = "N"
	FieldNickname      = "NICKNAME"
	FieldPhoto         = "PHOTO"
	FieldBirthday      = "BDAY"
	FieldAnniversary   = "ANNIVERSARY"
	FieldGender        = "GENDER"

	// Delivery Addressing Properties
	FieldAddress = "ADR"

	// Communications Properties
	FieldTelephone = "TEL"
	FieldEmail     = "EMAIL"
	FieldIMPP      = "IMPP" // Instant Messaging and Presence Protocol
	FieldLanguage  = "LANG"

	// Geographical Properties
	FieldTimezone    = "TZ"
	FieldGeolocation = "GEO"

	// Organizational Properties
	FieldTitle        = "TITLE"
	FieldRole         = "ROLE"
	FieldLogo         = "LOGO"
	FieldOrganization = "ORG"
	FieldMember       = "MEMBER"
	FieldRelated      = "RELATED"

	// Explanatory Properties
	FieldCategories   = "CATEGORIES"
	FieldNote         = "NOTE"
	FieldProductID    = "PRODID"
	FieldRevision     = "REV"
	FieldSound        = "SOUND"
	FieldUID          = "UID"
	FieldClientPIDMap = "CLIENTPIDMAP"
	FieldURL          = "URL"
	FieldVersion      = "VERSION"

	// Security Properties
	FieldKey = "KEY"

	// Calendar Properties
	FieldFreeOrBusyURL      = "FBURL"
	FieldCalendarAddressURI = "CALADRURI"
	FieldCalendarURI        = "CALURI"
)

func maybeGet(l []string, i int) string {
	if i < len(l) {
		return l[i]
	}
	return ""
}

// A Card is an address book entry.
type Card map[string][]*Field

// Get returns the first field of the card for the given property. If there is
// no such field, it returns nil.
func (c Card) Get(k string) *Field {
	fields := c[k]
	if len(fields) == 0 {
		return nil
	}
	return fields[0]
}

// Add adds the k, f pair to the list of fields. It appends to any existing
// fields.
func (c Card) Add(k string, f *Field) {
	c[k] = append(c[k], f)
}

// Set sets the key k to the single field f. It replaces any existing field.
func (c Card) Set(k string, f *Field) {
	c[k] = []*Field{f}
}

// Preferred returns the preferred field of the card for the given property.
func (c Card) Preferred(k string) *Field {
	fields := c[k]
	if len(fields) == 0 {
		return nil
	}

	field := fields[0]
	min := 100
	for _, f := range fields {
		n := 100
		if pref := f.Params.Get(ParamPreferred); pref != "" {
			n, _ = strconv.Atoi(pref)
		} else if f.Params.HasType("pref") {
			// Apple Contacts adds "pref" to the TYPE param
			n = 1
		}

		if n < min {
			min = n
			field = f
		}
	}
	return field
}

// Value returns the first field value of the card for the given property. If
// there is no such field, it returns an empty string.
func (c Card) Value(k string) string {
	f := c.Get(k)
	if f == nil {
		return ""
	}
	return f.Value
}

// AddValue adds the k, v pair to the list of field values. It appends to any
// existing values.
func (c Card) AddValue(k, v string) {
	c.Add(k, &Field{Value: v})
}

// SetValue sets the field k to the single value v. It replaces any existing
// value.
func (c Card) SetValue(k, v string) {
	c.Set(k, &Field{Value: v})
}

// PreferredValue returns the preferred field value of the card.
func (c Card) PreferredValue(k string) string {
	f := c.Preferred(k)
	if f == nil {
		return ""
	}
	return f.Value
}

// Values returns a list of values for a given property.
func (c Card) Values(k string) []string {
	fields := c[k]
	if fields == nil {
		return nil
	}

	values := make([]string, len(fields))
	for i, f := range fields {
		values[i] = f.Value
	}
	return values
}

// Kind returns the kind of the object represented by this card. If it isn't
// specified, it returns the default: KindIndividual.
func (c Card) Kind() Kind {
	kind := strings.ToLower(c.Value(FieldKind))
	if kind == "" {
		return KindIndividual
	}
	return Kind(kind)
}

// SetKind sets the kind of the object represented by this card.
func (c Card) SetKind(kind Kind) {
	c.SetValue(FieldKind, string(kind))
}

// FormattedNames returns formatted names of the card. The length of the result
// is always greater or equal to 1.
func (c Card) FormattedNames() []*Field {
	fns := c[FieldFormattedName]
	if len(fns) == 0 {
		return []*Field{{Value: ""}}
	}
	return fns
}

// Names returns names of the card.
func (c Card) Names() []*Name {
	ns := c[FieldName]
	if ns == nil {
		return nil
	}

	names := make([]*Name, len(ns))
	for i, n := range ns {
		names[i] = newName(n)
	}
	return names
}

// Name returns the preferred name of the card. If it isn't specified, it
// returns nil.
func (c Card) Name() *Name {
	n := c.Preferred(FieldName)
	if n == nil {
		return nil
	}
	return newName(n)
}

// AddName adds the specified name to the list of names.
func (c Card) AddName(name *Name) {
	c.Add(FieldName, name.field())
}

// SetName replaces the list of names with the single specified name.
func (c Card) SetName(name *Name) {
	c.Set(FieldName, name.field())
}

// Gender returns this card's gender.
func (c Card) Gender() (sex Sex, identity string) {
	v := c.Value(FieldGender)
	parts := strings.SplitN(v, ";", 2)
	return Sex(strings.ToUpper(parts[0])), maybeGet(parts, 1)
}

// SetGender sets this card's gender.
func (c Card) SetGender(sex Sex, identity string) {
	v := string(sex)
	if identity != "" {
		v += ";" + identity
	}
	c.SetValue(FieldGender, v)
}

// Addresses returns addresses of the card.
func (c Card) Addresses() []*Address {
	adrs := c[FieldAddress]
	if adrs == nil {
		return nil
	}

	addresses := make([]*Address, len(adrs))
	for i, adr := range adrs {
		addresses[i] = newAddress(adr)
	}
	return addresses
}

// Address returns the preferred address of the card. If it isn't specified, it
// returns nil.
func (c Card) Address() *Address {
	adr := c.Preferred(FieldAddress)
	if adr == nil {
		return nil
	}
	return newAddress(adr)
}

// AddAddress adds an address to the list of addresses.
func (c Card) AddAddress(address *Address) {
	c.Add(FieldAddress, address.field())
}

// SetAddress replaces the list of addresses with the single specified address.
func (c Card) SetAddress(address *Address) {
	c.Set(FieldAddress, address.field())
}

// Categories returns category information about the card, also known as "tags".
func (c Card) Categories() []string {
	return strings.Split(c.PreferredValue(FieldCategories), ",")
}

// SetCategories sets category information about the card.
func (c Card) SetCategories(categories []string) {
	c.SetValue(FieldCategories, strings.Join(categories, ","))
}

// Revision returns revision information about the current card.
func (c Card) Revision() (time.Time, error) {
	rev := c.Value(FieldRevision)
	if rev == "" {
		return time.Time{}, nil
	}
	return time.Parse(timestampLayout, rev)
}

// SetRevision sets revision information about the current card.
func (c Card) SetRevision(t time.Time) {
	c.SetValue(FieldRevision, t.Format(timestampLayout))
}

// A field contains a value and some parameters.
type Field struct {
	Value  string
	Params Params
	Group  string
}

// Params is a set of field parameters.
type Params map[string][]string

// Get returns the first value with the key k. It returns an empty string if
// there is no such value.
func (p Params) Get(k string) string {
	values := p[k]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Add adds the k, v pair to the list of parameters. It appends to any existing
// values.
func (p Params) Add(k, v string) {
	p[k] = append(p[k], v)
}

// Set sets the parameter k to the single value v. It replaces any existing
// value.
func (p Params) Set(k, v string) {
	p[k] = []string{v}
}

// Types returns the field types.
func (p Params) Types() []string {
	types := p[ParamType]
	list := make([]string, len(types))
	for i, t := range types {
		list[i] = strings.ToLower(t)
	}
	return list
}

// HasType returns true if and only if the field have the provided type.
func (p Params) HasType(t string) bool {
	for _, tt := range p[ParamType] {
		if strings.EqualFold(t, tt) {
			return true
		}
	}
	return false
}

// Kind is an object's kind.
type Kind string

// Values for FieldKind.
const (
	KindIndividual   Kind = "individual"
	KindGroup        Kind = "group"
	KindOrganization Kind = "org"
	KindLocation     Kind = "location"
)

// Values for ParamType.
const (
	// Generic
	TypeHome = "home"
	TypeWork = "work"

	// For FieldTelephone
	TypeText      = "text"
	TypeVoice     = "voice" // Default
	TypeFax       = "fax"
	TypeCell      = "cell"
	TypeVideo     = "video"
	TypePager     = "pager"
	TypeTextPhone = "textphone"

	// For FieldRelated
	TypeContact      = "contact"
	TypeAcquaintance = "acquaintance"
	TypeFriend       = "friend"
	TypeMet          = "met"
	TypeCoWorker     = "co-worker"
	TypeColleague    = "colleague"
	TypeCoResident   = "co-resident"
	TypeNeighbor     = "neighbor"
	TypeChild        = "child"
	TypeParent       = "parent"
	TypeSibling      = "sibling"
	TypeSpouse       = "spouse"
	TypeKin          = "kin"
	TypeMuse         = "muse"
	TypeCrush        = "crush"
	TypeDate         = "date"
	TypeSweetheart   = "sweetheart"
	TypeMe           = "me"
	TypeAgent        = "agent"
	TypeEmergency    = "emergency"
)

// Name contains an object's name components.
type Name struct {
	*Field

	FamilyName      string
	GivenName       string
	AdditionalName  string
	HonorificPrefix string
	HonorificSuffix string
}

func newName(field *Field) *Name {
	components := strings.Split(field.Value, ";")
	return &Name{
		field,
		maybeGet(components, 0),
		maybeGet(components, 1),
		maybeGet(components, 2),
		maybeGet(components, 3),
		maybeGet(components, 4),
	}
}

func (n *Name) field() *Field {
	if n.Field == nil {
		n.Field = new(Field)
	}
	n.Field.Value = strings.Join([]string{
		n.FamilyName,
		n.GivenName,
		n.AdditionalName,
		n.HonorificPrefix,
		n.HonorificSuffix,
	}, ";")
	return n.Field
}

// Sex is an object's biological sex.
type Sex string

const (
	SexUnspecified Sex = ""
	SexFemale      Sex = "F"
	SexMale        Sex = "M"
	SexOther       Sex = "O"
	SexNone        Sex = "N"
	SexUnknown     Sex = "U"
)

// An Address is a delivery address.
type Address struct {
	*Field

	PostOfficeBox   string
	ExtendedAddress string // e.g., apartment or suite number
	StreetAddress   string
	Locality        string // e.g., city
	Region          string // e.g., state or province
	PostalCode      string
	Country         string
}

func newAddress(field *Field) *Address {
	components := strings.Split(field.Value, ";")
	return &Address{
		field,
		maybeGet(components, 0),
		maybeGet(components, 1),
		maybeGet(components, 2),
		maybeGet(components, 3),
		maybeGet(components, 4),
		maybeGet(components, 5),
		maybeGet(components, 6),
	}
}

func (a *Address) field() *Field {
	if a.Field == nil {
		a.Field = new(Field)
	}
	a.Field.Value = strings.Join([]string{
		a.PostOfficeBox,
		a.ExtendedAddress,
		a.StreetAddress,
		a.Locality,
		a.Region,
		a.PostalCode,
		a.Country,
	}, ";")
	return a.Field
}
