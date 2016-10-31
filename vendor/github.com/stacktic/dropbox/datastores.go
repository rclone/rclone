/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

package dropbox

import (
	"fmt"
	"reflect"
	"regexp"
	"time"
)

// List represents a value of type list.
type List struct {
	record *Record
	field  string
	values []interface{}
}

// Fields represents a record.
type Fields map[string]value

// Record represents an entry in a table.
type Record struct {
	table     *Table
	recordID  string
	fields    Fields
	isDeleted bool
}

// Table represents a list of records.
type Table struct {
	datastore *Datastore
	tableID   string
	records   map[string]*Record
}

// DatastoreInfo represents the information about a datastore.
type DatastoreInfo struct {
	ID       string
	handle   string
	revision int
	title    string
	mtime    time.Time
}

type datastoreDelta struct {
	Revision int           `json:"rev"`
	Changes  listOfChanges `json:"changes"`
	Nonce    *string       `json:"nonce"`
}

type listOfDelta []datastoreDelta

// Datastore represents a datastore.
type Datastore struct {
	manager      *DatastoreManager
	info         DatastoreInfo
	changes      listOfChanges
	tables       map[string]*Table
	isDeleted    bool
	autoCommit   bool
	changesQueue chan changeWork
}

// DatastoreManager represents all datastores linked to the current account.
type DatastoreManager struct {
	dropbox    *Dropbox
	datastores []*Datastore
	token      string
}

const (
	defaultDatastoreID = "default"
	maxGlobalIDLength  = 63
	maxIDLength        = 64

	localIDPattern         = `[a-z0-9_-]([a-z0-9._-]{0,62}[a-z0-9_-])?`
	globalIDPattern        = `.[A-Za-z0-9_-]{1,63}`
	fieldsIDPattern        = `[A-Za-z0-9._+/=-]{1,64}`
	fieldsSpecialIDPattern = `:[A-Za-z0-9._+/=-]{1,63}`
)

var (
	localIDRegexp         *regexp.Regexp
	globalIDRegexp        *regexp.Regexp
	fieldsIDRegexp        *regexp.Regexp
	fieldsSpecialIDRegexp *regexp.Regexp
)

func init() {
	var err error
	if localIDRegexp, err = regexp.Compile(localIDPattern); err != nil {
		fmt.Println(err)
	}
	if globalIDRegexp, err = regexp.Compile(globalIDPattern); err != nil {
		fmt.Println(err)
	}
	if fieldsIDRegexp, err = regexp.Compile(fieldsIDPattern); err != nil {
		fmt.Println(err)
	}
	if fieldsSpecialIDRegexp, err = regexp.Compile(fieldsSpecialIDPattern); err != nil {
		fmt.Println(err)
	}
}

func isValidDatastoreID(ID string) bool {
	if ID[0] == '.' {
		return globalIDRegexp.MatchString(ID)
	}
	return localIDRegexp.MatchString(ID)
}

func isValidID(ID string) bool {
	if ID[0] == ':' {
		return fieldsSpecialIDRegexp.MatchString(ID)
	}
	return fieldsIDRegexp.MatchString(ID)
}

const (
	// TypeBoolean is the returned type when the value is a bool
	TypeBoolean AtomType = iota
	// TypeInteger is the returned type when the value is an int
	TypeInteger
	// TypeDouble is the returned type when the value is a float
	TypeDouble
	// TypeString is the returned type when the value is a string
	TypeString
	// TypeBytes is the returned type when the value is a []byte
	TypeBytes
	// TypeDate is the returned type when the value is a Date
	TypeDate
	// TypeList is the returned type when the value is a List
	TypeList
)

// AtomType represents the type of the value.
type AtomType int

// NewDatastoreManager returns a new DatastoreManager linked to the current account.
func (db *Dropbox) NewDatastoreManager() *DatastoreManager {
	return &DatastoreManager{
		dropbox: db,
	}
}

// OpenDatastore opens or creates a datastore.
func (dmgr *DatastoreManager) OpenDatastore(dsID string) (*Datastore, error) {
	rev, handle, _, err := dmgr.dropbox.openOrCreateDatastore(dsID)
	if err != nil {
		return nil, err
	}
	rv := &Datastore{
		manager: dmgr,
		info: DatastoreInfo{
			ID:       dsID,
			handle:   handle,
			revision: rev,
		},
		tables:       make(map[string]*Table),
		changesQueue: make(chan changeWork),
	}
	if rev > 0 {
		err = rv.LoadSnapshot()
	}
	go rv.doHandleChange()
	return rv, err
}

// OpenDefaultDatastore opens the default datastore.
func (dmgr *DatastoreManager) OpenDefaultDatastore() (*Datastore, error) {
	return dmgr.OpenDatastore(defaultDatastoreID)
}

// ListDatastores lists all datastores.
func (dmgr *DatastoreManager) ListDatastores() ([]DatastoreInfo, error) {
	info, _, err := dmgr.dropbox.listDatastores()
	return info, err
}

// DeleteDatastore deletes a datastore.
func (dmgr *DatastoreManager) DeleteDatastore(dsID string) error {
	_, err := dmgr.dropbox.deleteDatastore(dsID)
	return err
}

// CreateDatastore creates a global datastore with a unique ID, empty string for a random key.
func (dmgr *DatastoreManager) CreateDatastore(dsID string) (*Datastore, error) {
	rev, handle, _, err := dmgr.dropbox.createDatastore(dsID)
	if err != nil {
		return nil, err
	}
	return &Datastore{
		manager: dmgr,
		info: DatastoreInfo{
			ID:       dsID,
			handle:   handle,
			revision: rev,
		},
		tables:       make(map[string]*Table),
		changesQueue: make(chan changeWork),
	}, nil
}

// AwaitDeltas awaits for deltas and applies them.
func (ds *Datastore) AwaitDeltas() error {
	if len(ds.changes) != 0 {
		return fmt.Errorf("changes already pending")
	}
	_, _, deltas, err := ds.manager.dropbox.await([]*Datastore{ds}, "")
	if err != nil {
		return err
	}
	changes, ok := deltas[ds.info.handle]
	if !ok || len(changes) == 0 {
		return nil
	}
	return ds.applyDelta(changes)
}

func (ds *Datastore) applyDelta(dds []datastoreDelta) error {
	if len(ds.changes) != 0 {
		return fmt.Errorf("changes already pending")
	}
	for _, d := range dds {
		if d.Revision < ds.info.revision {
			continue
		}
		for _, c := range d.Changes {
			ds.applyChange(c)
		}
	}
	return nil
}

// Close closes the datastore.
func (ds *Datastore) Close() {
	close(ds.changesQueue)
}

// Delete deletes the datastore.
func (ds *Datastore) Delete() error {
	return ds.manager.DeleteDatastore(ds.info.ID)
}

// SetTitle sets the datastore title to the given string.
func (ds *Datastore) SetTitle(t string) error {
	if len(ds.info.title) == 0 {
		return ds.insertRecord(":info", "info", Fields{
			"title": value{
				values: []interface{}{t},
			},
		})
	}
	return ds.updateField(":info", "info", "title", t)
}

// SetMTime sets the datastore mtime to the given time.
func (ds *Datastore) SetMTime(t time.Time) error {
	if time.Time(ds.info.mtime).IsZero() {
		return ds.insertRecord(":info", "info", Fields{
			"mtime": value{
				values: []interface{}{t},
			},
		})
	}
	return ds.updateField(":info", "info", "mtime", t)
}

// Rollback reverts all local changes and discards them.
func (ds *Datastore) Rollback() error {
	if len(ds.changes) == 0 {
		return nil
	}
	for i := len(ds.changes) - 1; i >= 0; i-- {
		ds.applyChange(ds.changes[i].Revert)
	}
	ds.changes = ds.changes[:0]
	return nil
}

// GetTable returns the requested table.
func (ds *Datastore) GetTable(tableID string) (*Table, error) {
	if !isValidID(tableID) {
		return nil, fmt.Errorf("invalid table ID %s", tableID)
	}
	t, ok := ds.tables[tableID]
	if ok {
		return t, nil
	}
	t = &Table{
		datastore: ds,
		tableID:   tableID,
		records:   make(map[string]*Record),
	}
	ds.tables[tableID] = t
	return t, nil
}

// Commit commits the changes registered by sending them to the server.
func (ds *Datastore) Commit() error {
	rev, err := ds.manager.dropbox.putDelta(ds.info.handle, ds.info.revision, ds.changes)
	if err != nil {
		return err
	}
	ds.changes = ds.changes[:0]
	ds.info.revision = rev
	return nil
}

// LoadSnapshot updates the state of the datastore from the server.
func (ds *Datastore) LoadSnapshot() error {
	if len(ds.changes) != 0 {
		return fmt.Errorf("could not load snapshot when there are pending changes")
	}
	rows, rev, err := ds.manager.dropbox.getSnapshot(ds.info.handle)
	if err != nil {
		return err
	}

	ds.tables = make(map[string]*Table)
	for _, r := range rows {
		if _, ok := ds.tables[r.TID]; !ok {
			ds.tables[r.TID] = &Table{
				datastore: ds,
				tableID:   r.TID,
				records:   make(map[string]*Record),
			}
		}
		ds.tables[r.TID].records[r.RowID] = &Record{
			table:    ds.tables[r.TID],
			recordID: r.RowID,
			fields:   r.Data,
		}
	}
	ds.info.revision = rev
	return nil
}

// GetDatastore returns the datastore associated with this table.
func (t *Table) GetDatastore() *Datastore {
	return t.datastore
}

// GetID returns the ID of this table.
func (t *Table) GetID() string {
	return t.tableID
}

// Get returns the record with this ID.
func (t *Table) Get(recordID string) (*Record, error) {
	if !isValidID(recordID) {
		return nil, fmt.Errorf("invalid record ID %s", recordID)
	}
	return t.records[recordID], nil
}

// GetOrInsert gets the requested record.
func (t *Table) GetOrInsert(recordID string) (*Record, error) {
	if !isValidID(recordID) {
		return nil, fmt.Errorf("invalid record ID %s", recordID)
	}
	return t.GetOrInsertWithFields(recordID, nil)
}

// GetOrInsertWithFields gets the requested table.
func (t *Table) GetOrInsertWithFields(recordID string, fields Fields) (*Record, error) {
	if !isValidID(recordID) {
		return nil, fmt.Errorf("invalid record ID %s", recordID)
	}
	if r, ok := t.records[recordID]; ok {
		return r, nil
	}
	if fields == nil {
		fields = make(Fields)
	}
	if err := t.datastore.insertRecord(t.tableID, recordID, fields); err != nil {
		return nil, err
	}
	return t.records[recordID], nil
}

// Query returns a list of records matching all the given fields.
func (t *Table) Query(fields Fields) ([]*Record, error) {
	var records []*Record

next:
	for _, record := range t.records {
		for qf, qv := range fields {
			if rv, ok := record.fields[qf]; !ok || !reflect.DeepEqual(qv, rv) {
				continue next
			}
		}
		records = append(records, record)
	}
	return records, nil
}

// GetTable returns the table associated with this record.
func (r *Record) GetTable() *Table {
	return r.table
}

// GetID returns the ID of this record.
func (r *Record) GetID() string {
	return r.recordID
}

// IsDeleted returns whether this record was deleted.
func (r *Record) IsDeleted() bool {
	return r.isDeleted
}

// DeleteRecord deletes this record.
func (r *Record) DeleteRecord() {
	r.table.datastore.deleteRecord(r.table.tableID, r.recordID)
}

// HasField returns whether this field exists.
func (r *Record) HasField(field string) (bool, error) {
	if !isValidID(field) {
		return false, fmt.Errorf("invalid field %s", field)
	}
	_, ok := r.fields[field]
	return ok, nil
}

// Get gets the current value of this field.
func (r *Record) Get(field string) (interface{}, bool, error) {
	if !isValidID(field) {
		return nil, false, fmt.Errorf("invalid field %s", field)
	}
	v, ok := r.fields[field]
	if !ok {
		return nil, false, nil
	}
	if v.isList {
		return &List{
			record: r,
			field:  field,
			values: v.values,
		}, true, nil
	}
	return v.values[0], true, nil
}

// GetOrCreateList gets the current value of this field.
func (r *Record) GetOrCreateList(field string) (*List, error) {
	if !isValidID(field) {
		return nil, fmt.Errorf("invalid field %s", field)
	}
	v, ok := r.fields[field]
	if ok && !v.isList {
		return nil, fmt.Errorf("not a list")
	}
	if !ok {
		if err := r.table.datastore.listCreate(r.table.tableID, r.recordID, field); err != nil {
			return nil, err
		}
		v = r.fields[field]
	}
	return &List{
		record: r,
		field:  field,
		values: v.values,
	}, nil
}

func getType(i interface{}) (AtomType, error) {
	switch i.(type) {
	case bool:
		return TypeBoolean, nil
	case int, int32, int64:
		return TypeInteger, nil
	case float32, float64:
		return TypeDouble, nil
	case string:
		return TypeString, nil
	case []byte:
		return TypeBytes, nil
	case time.Time:
		return TypeDate, nil
	}
	return 0, fmt.Errorf("type %s not supported", reflect.TypeOf(i).Name())
}

// GetFieldType returns the type of the given field.
func (r *Record) GetFieldType(field string) (AtomType, error) {
	if !isValidID(field) {
		return 0, fmt.Errorf("invalid field %s", field)
	}
	v, ok := r.fields[field]
	if !ok {
		return 0, fmt.Errorf("no such field: %s", field)
	}
	if v.isList {
		return TypeList, nil
	}
	return getType(v.values[0])
}

// Set sets the value of a field.
func (r *Record) Set(field string, value interface{}) error {
	if !isValidID(field) {
		return fmt.Errorf("invalid field %s", field)
	}
	return r.table.datastore.updateField(r.table.tableID, r.recordID, field, value)
}

// DeleteField deletes the given field from this record.
func (r *Record) DeleteField(field string) error {
	if !isValidID(field) {
		return fmt.Errorf("invalid field %s", field)
	}
	return r.table.datastore.deleteField(r.table.tableID, r.recordID, field)
}

// FieldNames returns a list of fields names.
func (r *Record) FieldNames() []string {
	var rv []string

	rv = make([]string, 0, len(r.fields))
	for k := range r.fields {
		rv = append(rv, k)
	}
	return rv
}

// IsEmpty returns whether the list contains an element.
func (l *List) IsEmpty() bool {
	return len(l.values) == 0
}

// Size returns the number of elements in the list.
func (l *List) Size() int {
	return len(l.values)
}

// GetType gets the type of the n-th element in the list.
func (l *List) GetType(n int) (AtomType, error) {
	if n >= len(l.values) {
		return 0, fmt.Errorf("out of bound index")
	}
	return getType(l.values[n])
}

// Get gets the n-th element in the list.
func (l *List) Get(n int) (interface{}, error) {
	if n >= len(l.values) {
		return 0, fmt.Errorf("out of bound index")
	}
	return l.values[n], nil
}

// AddAtPos inserts the item at the n-th position in the list.
func (l *List) AddAtPos(n int, i interface{}) error {
	if n > len(l.values) {
		return fmt.Errorf("out of bound index")
	}
	err := l.record.table.datastore.listInsert(l.record.table.tableID, l.record.recordID, l.field, n, i)
	if err != nil {
		return err
	}
	l.values = l.record.fields[l.field].values
	return nil
}

// Add adds the item at the end of the list.
func (l *List) Add(i interface{}) error {
	return l.AddAtPos(len(l.values), i)
}

// Set sets the value of the n-th element of the list.
func (l *List) Set(n int, i interface{}) error {
	if n >= len(l.values) {
		return fmt.Errorf("out of bound index")
	}
	return l.record.table.datastore.listPut(l.record.table.tableID, l.record.recordID, l.field, n, i)
}

// Remove removes the n-th element of the list.
func (l *List) Remove(n int) error {
	if n >= len(l.values) {
		return fmt.Errorf("out of bound index")
	}
	err := l.record.table.datastore.listDelete(l.record.table.tableID, l.record.recordID, l.field, n)
	l.values = l.record.fields[l.field].values
	return err
}

// Move moves the element from the from-th position to the to-th.
func (l *List) Move(from, to int) error {
	if from >= len(l.values) || to >= len(l.values) {
		return fmt.Errorf("out of bound index")
	}
	return l.record.table.datastore.listMove(l.record.table.tableID, l.record.recordID, l.field, from, to)
}
