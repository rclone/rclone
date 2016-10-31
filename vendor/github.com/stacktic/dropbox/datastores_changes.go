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
)

type value struct {
	values []interface{}
	isList bool
}

type fieldOp struct {
	Op     string
	Index  int
	Index2 int
	Data   value
}

type opDict map[string]fieldOp

type change struct {
	Op       string
	TID      string
	RecordID string
	Ops      opDict
	Data     Fields
	Revert   *change
}
type listOfChanges []*change

type changeWork struct {
	c   *change
	out chan error
}

const (
	recordDelete = "D"
	recordInsert = "I"
	recordUpdate = "U"
	fieldDelete  = "D"
	fieldPut     = "P"
	listCreate   = "LC"
	listDelete   = "LD"
	listInsert   = "LI"
	listMove     = "LM"
	listPut      = "LP"
)

func newValueFromInterface(i interface{}) *value {
	if a, ok := i.([]byte); ok {
		return &value{
			values: []interface{}{a},
			isList: false,
		}
	}
	if reflect.TypeOf(i).Kind() == reflect.Slice || reflect.TypeOf(i).Kind() == reflect.Array {
		val := reflect.ValueOf(i)
		v := &value{
			values: make([]interface{}, val.Len()),
			isList: true,
		}
		for i := range v.values {
			v.values[i] = val.Index(i).Interface()
		}
		return v
	}
	return &value{
		values: []interface{}{i},
		isList: false,
	}
}

func newValue(v *value) *value {
	var nv *value

	nv = &value{
		values: make([]interface{}, len(v.values)),
		isList: v.isList,
	}
	copy(nv.values, v.values)
	return nv
}

func newFields(f Fields) Fields {
	var n Fields

	n = make(Fields)
	for k, v := range f {
		n[k] = *newValue(&v)
	}
	return n
}

func (ds *Datastore) deleteRecord(table, record string) error {
	return ds.handleChange(&change{
		Op:       recordDelete,
		TID:      table,
		RecordID: record,
	})
}

func (ds *Datastore) insertRecord(table, record string, values Fields) error {
	return ds.handleChange(&change{
		Op:       recordInsert,
		TID:      table,
		RecordID: record,
		Data:     newFields(values),
	})
}

func (ds *Datastore) updateFields(table, record string, values map[string]interface{}) error {
	var dsval opDict

	dsval = make(opDict)
	for k, v := range values {
		dsval[k] = fieldOp{
			Op:   fieldPut,
			Data: *newValueFromInterface(v),
		}
	}
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops:      dsval,
	})
}

func (ds *Datastore) updateField(table, record, field string, i interface{}) error {
	return ds.updateFields(table, record, map[string]interface{}{field: i})
}

func (ds *Datastore) deleteField(table, record, field string) error {
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops: opDict{
			field: fieldOp{
				Op: fieldDelete,
			},
		},
	})
}

func (ds *Datastore) listCreate(table, record, field string) error {
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops: opDict{
			field: fieldOp{
				Op: listCreate,
			},
		},
	})
}

func (ds *Datastore) listDelete(table, record, field string, pos int) error {
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops: opDict{
			field: fieldOp{
				Op:    listDelete,
				Index: pos,
			},
		},
	})
}

func (ds *Datastore) listInsert(table, record, field string, pos int, i interface{}) error {
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops: opDict{
			field: fieldOp{
				Op:    listInsert,
				Index: pos,
				Data:  *newValueFromInterface(i),
			},
		},
	})
}

func (ds *Datastore) listMove(table, record, field string, from, to int) error {
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops: opDict{
			field: fieldOp{
				Op:     listMove,
				Index:  from,
				Index2: to,
			},
		},
	})
}

func (ds *Datastore) listPut(table, record, field string, pos int, i interface{}) error {
	return ds.handleChange(&change{
		Op:       recordUpdate,
		TID:      table,
		RecordID: record,
		Ops: opDict{
			field: fieldOp{
				Op:    listPut,
				Index: pos,
				Data:  *newValueFromInterface(i),
			},
		},
	})
}

func (ds *Datastore) handleChange(c *change) error {
	var out chan error

	if ds.changesQueue == nil {
		return fmt.Errorf("datastore is closed")
	}
	out = make(chan error)
	ds.changesQueue <- changeWork{
		c:   c,
		out: out,
	}
	return <-out
}

func (ds *Datastore) doHandleChange() {
	var err error
	var c *change

	q := ds.changesQueue
	for cw := range q {
		c = cw.c

		if err = ds.validateChange(c); err != nil {
			cw.out <- err
			continue
		}
		if c.Revert, err = ds.inverseChange(c); err != nil {
			cw.out <- err
			continue
		}

		if err = ds.applyChange(c); err != nil {
			cw.out <- err
			continue
		}

		ds.changes = append(ds.changes, c)

		if ds.autoCommit {
			if err = ds.Commit(); err != nil {
				cw.out <- err
			}
		}
		close(cw.out)
	}
}

func (ds *Datastore) validateChange(c *change) error {
	var t *Table
	var r *Record
	var ok bool

	if t, ok = ds.tables[c.TID]; !ok {
		t = &Table{
			datastore: ds,
			tableID:   c.TID,
			records:   make(map[string]*Record),
		}
	}

	r = t.records[c.RecordID]

	switch c.Op {
	case recordInsert, recordDelete:
		return nil
	case recordUpdate:
		if r == nil {
			return fmt.Errorf("no such record: %s", c.RecordID)
		}
		for field, op := range c.Ops {
			if op.Op == fieldPut || op.Op == fieldDelete {
				continue
			}
			v, ok := r.fields[field]
			if op.Op == listCreate {
				if ok {
					return fmt.Errorf("field %s already exists", field)
				}
				continue
			}
			if !ok {
				return fmt.Errorf("no such field: %s", field)
			}
			if !v.isList {
				return fmt.Errorf("field %s is not a list", field)
			}
			maxIndex := len(v.values) - 1
			if op.Op == listInsert {
				maxIndex++
			}
			if op.Index > maxIndex {
				return fmt.Errorf("out of bound access index %d on [0:%d]", op.Index, maxIndex)
			}
			if op.Index2 > maxIndex {
				return fmt.Errorf("out of bound access index %d on [0:%d]", op.Index, maxIndex)
			}
		}
	}
	return nil
}

func (ds *Datastore) applyChange(c *change) error {
	var t *Table
	var r *Record
	var ok bool

	if t, ok = ds.tables[c.TID]; !ok {
		t = &Table{
			datastore: ds,
			tableID:   c.TID,
			records:   make(map[string]*Record),
		}
		ds.tables[c.TID] = t
	}

	r = t.records[c.RecordID]

	switch c.Op {
	case recordInsert:
		t.records[c.RecordID] = &Record{
			table:    t,
			recordID: c.RecordID,
			fields:   newFields(c.Data),
		}
	case recordDelete:
		if r == nil {
			return nil
		}
		r.isDeleted = true
		delete(t.records, c.RecordID)
	case recordUpdate:
		for field, op := range c.Ops {
			v, ok := r.fields[field]
			switch op.Op {
			case fieldPut:
				r.fields[field] = *newValue(&op.Data)
			case fieldDelete:
				if ok {
					delete(r.fields, field)
				}
			case listCreate:
				if !ok {
					r.fields[field] = value{isList: true}
				}
			case listDelete:
				copy(v.values[op.Index:], v.values[op.Index+1:])
				v.values = v.values[:len(v.values)-1]
				r.fields[field] = v
			case listInsert:
				v.values = append(v.values, op.Data)
				copy(v.values[op.Index+1:], v.values[op.Index:len(v.values)-1])
				v.values[op.Index] = op.Data.values[0]
				r.fields[field] = v
			case listMove:
				val := v.values[op.Index]
				if op.Index < op.Index2 {
					copy(v.values[op.Index:op.Index2], v.values[op.Index+1:op.Index2+1])
				} else {
					copy(v.values[op.Index2+1:op.Index+1], v.values[op.Index2:op.Index])
				}
				v.values[op.Index2] = val
				r.fields[field] = v
			case listPut:
				r.fields[field].values[op.Index] = op.Data.values[0]
			}
		}
	}
	return nil
}

func (ds *Datastore) inverseChange(c *change) (*change, error) {
	var t *Table
	var r *Record
	var ok bool
	var rev *change

	if t, ok = ds.tables[c.TID]; !ok {
		t = &Table{
			datastore: ds,
			tableID:   c.TID,
			records:   make(map[string]*Record),
		}
		ds.tables[c.TID] = t
	}

	r = t.records[c.RecordID]

	switch c.Op {
	case recordInsert:
		return &change{
			Op:       recordDelete,
			TID:      c.TID,
			RecordID: c.RecordID,
		}, nil
	case recordDelete:
		if r == nil {
			return nil, nil
		}
		return &change{
			Op:       recordInsert,
			TID:      c.TID,
			RecordID: c.RecordID,
			Data:     newFields(r.fields),
		}, nil
	case recordUpdate:
		rev = &change{
			Op:       recordUpdate,
			TID:      c.TID,
			RecordID: c.RecordID,
			Ops:      make(opDict),
		}
		for field, op := range c.Ops {
			switch op.Op {
			case fieldPut:
				if v, ok := r.fields[field]; ok {
					rev.Ops[field] = fieldOp{
						Op:   fieldPut,
						Data: *newValue(&v),
					}
				} else {
					rev.Ops[field] = fieldOp{
						Op: fieldDelete,
					}
				}
			case fieldDelete:
				if v, ok := r.fields[field]; ok {
					rev.Ops[field] = fieldOp{
						Op:   fieldPut,
						Data: *newValue(&v),
					}
				}
			case listCreate:
				if _, ok := r.fields[field]; !ok {
					rev.Ops[field] = fieldOp{
						Op: fieldDelete,
					}
				}
			case listDelete:
				v := r.fields[field]
				rev.Ops[field] = fieldOp{
					Op:    listInsert,
					Index: op.Index,
					Data: value{
						values: []interface{}{v.values[op.Index]},
						isList: false,
					},
				}
			case listInsert:
				rev.Ops[field] = fieldOp{
					Op:    listDelete,
					Index: op.Index,
				}
			case listMove:
				rev.Ops[field] = fieldOp{
					Op:     listMove,
					Index:  op.Index2,
					Index2: op.Index,
				}
			case listPut:
				v := r.fields[field]
				rev.Ops[field] = fieldOp{
					Op:    listPut,
					Index: op.Index,
					Data: value{
						values: []interface{}{v.values[op.Index]},
						isList: false,
					},
				}
			}
		}
	}
	return rev, nil
}
