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
	"encoding/json"
	"testing"
)

func checkList(t *testing.T, l *List, e []interface{}) {
	var elt1 interface{}
	var err error

	if l.Size() != len(e) {
		t.Errorf("wrong size")
	}
	for i := range e {
		if elt1, err = l.Get(i); err != nil {
			t.Errorf("%s", err)
		}
		if elt1 != e[i] {
			t.Errorf("position %d mismatch got %#v, expected %#v", i, elt1, e[i])
		}
	}
}

func newDatastore(t *testing.T) *Datastore {
	var ds *Datastore

	ds = &Datastore{
		manager: newDropbox(t).NewDatastoreManager(),
		info: DatastoreInfo{
			ID:       "dummyID",
			handle:   "dummyHandle",
			title:    "dummyTitle",
			revision: 0,
		},
		tables:       make(map[string]*Table),
		changesQueue: make(chan changeWork),
	}
	go ds.doHandleChange()
	return ds
}

func TestList(t *testing.T) {
	var tbl *Table
	var r *Record
	var ds *Datastore
	var l *List
	var err error

	ds = newDatastore(t)

	if tbl, err = ds.GetTable("dummyTable"); err != nil {
		t.Errorf("%s", err)
	}
	if r, err = tbl.GetOrInsert("dummyRecord"); err != nil {
		t.Errorf("%s", err)
	}
	if l, err = r.GetOrCreateList("dummyList"); err != nil {
		t.Errorf("%s", err)
	}
	for i := 0; i < 10; i++ {
		if err = l.Add(i); err != nil {
			t.Errorf("%s", err)
		}
	}
	if ftype, err := r.GetFieldType("dummyList"); err != nil || ftype != TypeList {
		t.Errorf("wrong type")
	}

	ftype, err := l.GetType(0)
	if err != nil {
		t.Errorf("%s", err)
	}
	if ftype != TypeInteger {
		t.Errorf("wrong type")
	}

	checkList(t, l, []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	if err = l.Remove(5); err != nil {
		t.Errorf("could not remove element 5")
	}
	checkList(t, l, []interface{}{0, 1, 2, 3, 4, 6, 7, 8, 9})

	if err = l.Remove(0); err != nil {
		t.Errorf("could not remove element 0")
	}
	checkList(t, l, []interface{}{1, 2, 3, 4, 6, 7, 8, 9})

	if err = l.Remove(7); err != nil {
		t.Errorf("could not remove element 7")
	}
	checkList(t, l, []interface{}{1, 2, 3, 4, 6, 7, 8})

	if err = l.Remove(7); err == nil {
		t.Errorf("out of bound index must return an error")
	}
	checkList(t, l, []interface{}{1, 2, 3, 4, 6, 7, 8})

	if err = l.Move(3, 6); err != nil {
		t.Errorf("could not move element 3 to position 6")
	}
	checkList(t, l, []interface{}{1, 2, 3, 6, 7, 8, 4})

	if err = l.Move(3, 9); err == nil {
		t.Errorf("out of bound index must return an error")
	}
	checkList(t, l, []interface{}{1, 2, 3, 6, 7, 8, 4})

	if err = l.Move(6, 3); err != nil {
		t.Errorf("could not move element 6 to position 3")
	}
	checkList(t, l, []interface{}{1, 2, 3, 4, 6, 7, 8})

	if err = l.AddAtPos(0, 0); err != nil {
		t.Errorf("could not insert element at position 0")
	}
	checkList(t, l, []interface{}{0, 1, 2, 3, 4, 6, 7, 8})

	if err = l.Add(9); err != nil {
		t.Errorf("could not append element")
	}
	checkList(t, l, []interface{}{0, 1, 2, 3, 4, 6, 7, 8, 9})

	if err = l.AddAtPos(5, 5); err != nil {
		t.Errorf("could not insert element at position 5")
	}
	checkList(t, l, []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	if err = l.Set(0, 3); err != nil {
		t.Errorf("could not update element at position 0")
	}
	checkList(t, l, []interface{}{3, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	if err = l.Set(9, 2); err != nil {
		t.Errorf("could not update element at position 9")
	}
	checkList(t, l, []interface{}{3, 1, 2, 3, 4, 5, 6, 7, 8, 2})

	if err = l.Set(10, 11); err == nil {
		t.Errorf("out of bound index must return an error")
	}
	checkList(t, l, []interface{}{3, 1, 2, 3, 4, 5, 6, 7, 8, 2})
}

func TestGenerateID(t *testing.T) {
	f, err := generateDatastoreID()
	if err != nil {
		t.Errorf("%s", err)
	}
	if !isValidDatastoreID(f) {
		t.Errorf("generated ID is not correct")
	}
}

func TestUnmarshalAwait(t *testing.T) {
	type awaitResult struct {
		Deltas struct {
			Results map[string]struct {
				Deltas []datastoreDelta `json:"deltas"`
			} `json:"deltas"`
		} `json:"get_deltas"`
		Datastores struct {
			Info  []datastoreInfo `json:"datastores"`
			Token string          `json:"token"`
		} `json:"list_datastores"`
	}
	var r awaitResult
	var datastoreID string
	var res []datastoreDelta

	js := `{"get_deltas":{"deltas":{"12345678901234567890":{"deltas":[{"changes":[["I","dummyTable","dummyRecord",{}]],"nonce":"","rev":0},{"changes":[["U","dummyTable","dummyRecord",{"name":["P","dummy"]}],["U","dummyTable","dummyRecord",{"dummyList":["LC"]}]],"nonce":"","rev":1},{"changes":[["U","dummyTable","dummyRecord",{"dummyList":["LI",0,{"I":"0"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",1,{"I":"1"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",2,{"I":"2"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",3,{"I":"3"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",4,{"I":"4"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",5,{"I":"5"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",6,{"I":"6"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",7,{"I":"7"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",8,{"I":"8"}]}],["U","dummyTable","dummyRecord",{"dummyList":["LI",9,{"I":"9"}]}]],"nonce":"","rev":2},{"changes":[["D","dummyTable","dummyRecord"]],"nonce":"","rev":3}]}}}}`
	datastoreID = "12345678901234567890"

	expected := []datastoreDelta{
		datastoreDelta{
			Revision: 0,
			Changes: listOfChanges{
				&change{Op: recordInsert, TID: "dummyTable", RecordID: "dummyRecord", Data: Fields{}},
			},
		},
		datastoreDelta{
			Revision: 1,
			Changes: listOfChanges{
				&change{Op: "U", TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"name": fieldOp{Op: "P", Index: 0, Data: value{values: []interface{}{"dummy"}}}}},
				&change{Op: "U", TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: "LC"}}},
			},
		},
		datastoreDelta{
			Revision: 2,
			Changes: listOfChanges{
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 0, Data: value{values: []interface{}{int64(0)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 1, Data: value{values: []interface{}{int64(1)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 2, Data: value{values: []interface{}{int64(2)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 3, Data: value{values: []interface{}{int64(3)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 4, Data: value{values: []interface{}{int64(4)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 5, Data: value{values: []interface{}{int64(5)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 6, Data: value{values: []interface{}{int64(6)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 7, Data: value{values: []interface{}{int64(7)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 8, Data: value{values: []interface{}{int64(8)}}}}},
				&change{Op: recordUpdate, TID: "dummyTable", RecordID: "dummyRecord", Ops: opDict{"dummyList": fieldOp{Op: listInsert, Index: 9, Data: value{values: []interface{}{int64(9)}}}}},
			},
		},
		datastoreDelta{
			Revision: 3,
			Changes: listOfChanges{
				&change{Op: "D", TID: "dummyTable", RecordID: "dummyRecord"},
			},
		},
	}
	err := json.Unmarshal([]byte(js), &r)
	if err != nil {
		t.Errorf("%s", err)
	}
	if len(r.Deltas.Results) != 1 {
		t.Errorf("wrong number of datastoreDelta")
	}

	if tmp, ok := r.Deltas.Results[datastoreID]; !ok {
		t.Fatalf("wrong datastore ID")
	} else {
		res = tmp.Deltas
	}
	if len(res) != len(expected) {
		t.Fatalf("got %d results expected %d", len(res), len(expected))
	}
	for i, d := range res {
		ed := expected[i]
		if d.Revision != ed.Revision {
			t.Errorf("wrong revision got %d expected %d", d.Revision, expected[i].Revision)
		}
		for j, c := range d.Changes {
			if !c.equals(ed.Changes[j]) {
				t.Errorf("wrong change: got: %+v expected: %+v", *c, *ed.Changes[j])
			}
		}
	}
}
