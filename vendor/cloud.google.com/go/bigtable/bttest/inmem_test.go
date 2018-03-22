// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bttest

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/context"
	btapb "google.golang.org/genproto/googleapis/bigtable/admin/v2"
	btpb "google.golang.org/genproto/googleapis/bigtable/v2"
	"google.golang.org/grpc"
)

func TestConcurrentMutationsReadModifyAndGC(t *testing.T) {
	s := &server{
		tables: make(map[string]*table),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.CreateTable(
		ctx,
		&btapb.CreateTableRequest{Parent: "cluster", TableId: "t"}); err != nil {
		t.Fatal(err)
	}
	const name = `cluster/tables/t`
	tbl := s.tables[name]
	req := &btapb.ModifyColumnFamiliesRequest{
		Name: name,
		Modifications: []*btapb.ModifyColumnFamiliesRequest_Modification{{
			Id:  "cf",
			Mod: &btapb.ModifyColumnFamiliesRequest_Modification_Create{Create: &btapb.ColumnFamily{}},
		}},
	}
	_, err := s.ModifyColumnFamilies(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	req = &btapb.ModifyColumnFamiliesRequest{
		Name: name,
		Modifications: []*btapb.ModifyColumnFamiliesRequest_Modification{{
			Id: "cf",
			Mod: &btapb.ModifyColumnFamiliesRequest_Modification_Update{Update: &btapb.ColumnFamily{
				GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 1}},
			}},
		}},
	}
	if _, err := s.ModifyColumnFamilies(ctx, req); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var ts int64
	ms := func() []*btpb.Mutation {
		return []*btpb.Mutation{{
			Mutation: &btpb.Mutation_SetCell_{SetCell: &btpb.Mutation_SetCell{
				FamilyName:      "cf",
				ColumnQualifier: []byte(`col`),
				TimestampMicros: atomic.AddInt64(&ts, 1000),
			}},
		}}
	}

	rmw := func() *btpb.ReadModifyWriteRowRequest {
		return &btpb.ReadModifyWriteRowRequest{
			TableName: name,
			RowKey:    []byte(fmt.Sprint(rand.Intn(100))),
			Rules: []*btpb.ReadModifyWriteRule{{
				FamilyName:      "cf",
				ColumnQualifier: []byte("col"),
				Rule:            &btpb.ReadModifyWriteRule_IncrementAmount{IncrementAmount: 1},
			}},
		}
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				req := &btpb.MutateRowRequest{
					TableName: name,
					RowKey:    []byte(fmt.Sprint(rand.Intn(100))),
					Mutations: ms(),
				}
				s.MutateRow(ctx, req)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				_, _ = s.ReadModifyWriteRow(ctx, rmw())
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			tbl.gc()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("Concurrent mutations and GCs haven't completed after 1s")
	}
}

func TestCreateTableWithFamily(t *testing.T) {
	// The Go client currently doesn't support creating a table with column families
	// in one operation but it is allowed by the API. This must still be supported by the
	// fake server so this test lives here instead of in the main bigtable
	// integration test.
	s := &server{
		tables: make(map[string]*table),
	}
	ctx := context.Background()
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf1": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 123}}},
			"cf2": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 456}}},
		},
	}
	cTbl, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		t.Fatalf("Creating table: %v", err)
	}
	tbl, err := s.GetTable(ctx, &btapb.GetTableRequest{Name: cTbl.Name})
	if err != nil {
		t.Fatalf("Getting table: %v", err)
	}
	cf := tbl.ColumnFamilies["cf1"]
	if cf == nil {
		t.Fatalf("Missing col family cf1")
	}
	if got, want := cf.GcRule.GetMaxNumVersions(), int32(123); got != want {
		t.Errorf("Invalid MaxNumVersions: wanted:%d, got:%d", want, got)
	}
	cf = tbl.ColumnFamilies["cf2"]
	if cf == nil {
		t.Fatalf("Missing col family cf2")
	}
	if got, want := cf.GcRule.GetMaxNumVersions(), int32(456); got != want {
		t.Errorf("Invalid MaxNumVersions: wanted:%d, got:%d", want, got)
	}
}

type MockSampleRowKeysServer struct {
	responses []*btpb.SampleRowKeysResponse
	grpc.ServerStream
}

func (s *MockSampleRowKeysServer) Send(resp *btpb.SampleRowKeysResponse) error {
	s.responses = append(s.responses, resp)
	return nil
}

func TestSampleRowKeys(t *testing.T) {
	s := &server{
		tables: make(map[string]*table),
	}
	ctx := context.Background()
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 1}}},
		},
	}
	tbl, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		t.Fatalf("Creating table: %v", err)
	}

	// Populate the table
	val := []byte("value")
	rowCount := 1000
	for i := 0; i < rowCount; i++ {
		req := &btpb.MutateRowRequest{
			TableName: tbl.Name,
			RowKey:    []byte("row-" + strconv.Itoa(i)),
			Mutations: []*btpb.Mutation{{
				Mutation: &btpb.Mutation_SetCell_{SetCell: &btpb.Mutation_SetCell{
					FamilyName:      "cf",
					ColumnQualifier: []byte("col"),
					TimestampMicros: 0,
					Value:           val,
				}},
			}},
		}
		if _, err := s.MutateRow(ctx, req); err != nil {
			t.Fatalf("Populating table: %v", err)
		}
	}

	mock := &MockSampleRowKeysServer{}
	if err := s.SampleRowKeys(&btpb.SampleRowKeysRequest{TableName: tbl.Name}, mock); err != nil {
		t.Errorf("SampleRowKeys error: %v", err)
	}
	if len(mock.responses) == 0 {
		t.Fatal("Response count: got 0, want > 0")
	}
	// Make sure the offset of the final response is the offset of the final row
	got := mock.responses[len(mock.responses)-1].OffsetBytes
	want := int64((rowCount - 1) * len(val))
	if got != want {
		t.Errorf("Invalid offset: got %d, want %d", got, want)
	}
}

func TestDropRowRange(t *testing.T) {
	s := &server{
		tables: make(map[string]*table),
	}
	ctx := context.Background()
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 1}}},
		},
	}
	tblInfo, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		t.Fatalf("Creating table: %v", err)
	}

	tbl := s.tables[tblInfo.Name]

	// Populate the table
	prefixes := []string{"AAA", "BBB", "CCC", "DDD"}
	count := 3
	doWrite := func() {
		for _, prefix := range prefixes {
			for i := 0; i < count; i++ {
				req := &btpb.MutateRowRequest{
					TableName: tblInfo.Name,
					RowKey:    []byte(prefix + strconv.Itoa(i)),
					Mutations: []*btpb.Mutation{{
						Mutation: &btpb.Mutation_SetCell_{SetCell: &btpb.Mutation_SetCell{
							FamilyName:      "cf",
							ColumnQualifier: []byte("col"),
							TimestampMicros: 0,
							Value:           []byte{},
						}},
					}},
				}
				if _, err := s.MutateRow(ctx, req); err != nil {
					t.Fatalf("Populating table: %v", err)
				}
			}
		}
	}

	doWrite()
	tblSize := tbl.rows.Len()
	req := &btapb.DropRowRangeRequest{
		Name:   tblInfo.Name,
		Target: &btapb.DropRowRangeRequest_RowKeyPrefix{RowKeyPrefix: []byte("AAA")},
	}
	if _, err = s.DropRowRange(ctx, req); err != nil {
		t.Fatalf("Dropping first range: %v", err)
	}
	got, want := tbl.rows.Len(), tblSize-count
	if got != want {
		t.Errorf("Row count after first drop: got %d (%v), want %d", got, tbl.rows, want)
	}

	req = &btapb.DropRowRangeRequest{
		Name:   tblInfo.Name,
		Target: &btapb.DropRowRangeRequest_RowKeyPrefix{RowKeyPrefix: []byte("DDD")},
	}
	if _, err = s.DropRowRange(ctx, req); err != nil {
		t.Fatalf("Dropping second range: %v", err)
	}
	got, want = tbl.rows.Len(), tblSize-(2*count)
	if got != want {
		t.Errorf("Row count after second drop: got %d (%v), want %d", got, tbl.rows, want)
	}

	req = &btapb.DropRowRangeRequest{
		Name:   tblInfo.Name,
		Target: &btapb.DropRowRangeRequest_RowKeyPrefix{RowKeyPrefix: []byte("XXX")},
	}
	if _, err = s.DropRowRange(ctx, req); err != nil {
		t.Fatalf("Dropping invalid range: %v", err)
	}
	got, want = tbl.rows.Len(), tblSize-(2*count)
	if got != want {
		t.Errorf("Row count after invalid drop: got %d (%v), want %d", got, tbl.rows, want)
	}

	req = &btapb.DropRowRangeRequest{
		Name:   tblInfo.Name,
		Target: &btapb.DropRowRangeRequest_DeleteAllDataFromTable{DeleteAllDataFromTable: true},
	}
	if _, err = s.DropRowRange(ctx, req); err != nil {
		t.Fatalf("Dropping all data: %v", err)
	}
	got, want = tbl.rows.Len(), 0
	if got != want {
		t.Errorf("Row count after drop all: got %d, want %d", got, want)
	}

	// Test that we can write rows, delete some and then write them again.
	count = 1
	doWrite()

	req = &btapb.DropRowRangeRequest{
		Name:   tblInfo.Name,
		Target: &btapb.DropRowRangeRequest_DeleteAllDataFromTable{DeleteAllDataFromTable: true},
	}
	if _, err = s.DropRowRange(ctx, req); err != nil {
		t.Fatalf("Dropping all data: %v", err)
	}
	got, want = tbl.rows.Len(), 0
	if got != want {
		t.Errorf("Row count after drop all: got %d, want %d", got, want)
	}

	doWrite()
	got, want = tbl.rows.Len(), len(prefixes)
	if got != want {
		t.Errorf("Row count after rewrite: got %d, want %d", got, want)
	}

	req = &btapb.DropRowRangeRequest{
		Name:   tblInfo.Name,
		Target: &btapb.DropRowRangeRequest_RowKeyPrefix{RowKeyPrefix: []byte("BBB")},
	}
	if _, err = s.DropRowRange(ctx, req); err != nil {
		t.Fatalf("Dropping range: %v", err)
	}
	doWrite()
	got, want = tbl.rows.Len(), len(prefixes)
	if got != want {
		t.Errorf("Row count after drop range: got %d, want %d", got, want)
	}
}

type MockReadRowsServer struct {
	responses []*btpb.ReadRowsResponse
	grpc.ServerStream
}

func (s *MockReadRowsServer) Send(resp *btpb.ReadRowsResponse) error {
	s.responses = append(s.responses, resp)
	return nil
}

func TestReadRows(t *testing.T) {
	ctx := context.Background()
	s := &server{
		tables: make(map[string]*table),
	}
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf0": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 1}}},
		},
	}
	tblInfo, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		t.Fatalf("Creating table: %v", err)
	}
	mreq := &btpb.MutateRowRequest{
		TableName: tblInfo.Name,
		RowKey:    []byte("row"),
		Mutations: []*btpb.Mutation{{
			Mutation: &btpb.Mutation_SetCell_{SetCell: &btpb.Mutation_SetCell{
				FamilyName:      "cf0",
				ColumnQualifier: []byte("col"),
				TimestampMicros: 1000,
				Value:           []byte{},
			}},
		}},
	}
	if _, err := s.MutateRow(ctx, mreq); err != nil {
		t.Fatalf("Populating table: %v", err)
	}

	for _, rowset := range []*btpb.RowSet{
		{RowKeys: [][]byte{[]byte("row")}},
		{RowRanges: []*btpb.RowRange{{StartKey: &btpb.RowRange_StartKeyClosed{StartKeyClosed: []byte("")}}}},
		{RowRanges: []*btpb.RowRange{{StartKey: &btpb.RowRange_StartKeyClosed{StartKeyClosed: []byte("r")}}}},
		{RowRanges: []*btpb.RowRange{{
			StartKey: &btpb.RowRange_StartKeyClosed{StartKeyClosed: []byte("")},
			EndKey:   &btpb.RowRange_EndKeyOpen{EndKeyOpen: []byte("s")},
		}}},
	} {
		mock := &MockReadRowsServer{}
		req := &btpb.ReadRowsRequest{TableName: tblInfo.Name, Rows: rowset}
		if err = s.ReadRows(req, mock); err != nil {
			t.Fatalf("ReadRows error: %v", err)
		}
		if got, want := len(mock.responses), 1; got != want {
			t.Errorf("%+v: response count: got %d, want %d", rowset, got, want)
		}
	}
}

func TestReadRowsOrder(t *testing.T) {
	s := &server{
		tables: make(map[string]*table),
	}
	ctx := context.Background()
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf0": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 1}}},
		},
	}
	tblInfo, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		t.Fatalf("Creating table: %v", err)
	}
	count := 3
	mcf := func(i int) *btapb.ModifyColumnFamiliesRequest {
		return &btapb.ModifyColumnFamiliesRequest{
			Name: tblInfo.Name,
			Modifications: []*btapb.ModifyColumnFamiliesRequest_Modification{{
				Id:  "cf" + strconv.Itoa(i),
				Mod: &btapb.ModifyColumnFamiliesRequest_Modification_Create{Create: &btapb.ColumnFamily{}},
			}},
		}
	}
	for i := 1; i <= count; i++ {
		_, err = s.ModifyColumnFamilies(ctx, mcf(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	// Populate the table
	for fc := 0; fc < count; fc++ {
		for cc := count; cc > 0; cc-- {
			for tc := 0; tc < count; tc++ {
				req := &btpb.MutateRowRequest{
					TableName: tblInfo.Name,
					RowKey:    []byte("row"),
					Mutations: []*btpb.Mutation{{
						Mutation: &btpb.Mutation_SetCell_{SetCell: &btpb.Mutation_SetCell{
							FamilyName:      "cf" + strconv.Itoa(fc),
							ColumnQualifier: []byte("col" + strconv.Itoa(cc)),
							TimestampMicros: int64((tc + 1) * 1000),
							Value:           []byte{},
						}},
					}},
				}
				if _, err := s.MutateRow(ctx, req); err != nil {
					t.Fatalf("Populating table: %v", err)
				}
			}
		}
	}
	req := &btpb.ReadRowsRequest{
		TableName: tblInfo.Name,
		Rows:      &btpb.RowSet{RowKeys: [][]byte{[]byte("row")}},
	}
	mock := &MockReadRowsServer{}
	if err = s.ReadRows(req, mock); err != nil {
		t.Errorf("ReadRows error: %v", err)
	}
	if len(mock.responses) == 0 {
		t.Fatal("Response count: got 0, want > 0")
	}
	if len(mock.responses[0].Chunks) != 27 {
		t.Fatalf("Chunk count: got %d, want 27", len(mock.responses[0].Chunks))
	}
	testOrder := func(ms *MockReadRowsServer) {
		var prevFam, prevCol string
		var prevTime int64
		for _, cc := range ms.responses[0].Chunks {
			if prevFam == "" {
				prevFam = cc.FamilyName.Value
				prevCol = string(cc.Qualifier.Value)
				prevTime = cc.TimestampMicros
				continue
			}
			if cc.FamilyName.Value < prevFam {
				t.Errorf("Family order is not correct: got %s < %s", cc.FamilyName.Value, prevFam)
			} else if cc.FamilyName.Value == prevFam {
				if string(cc.Qualifier.Value) < prevCol {
					t.Errorf("Column order is not correct: got %s < %s", string(cc.Qualifier.Value), prevCol)
				} else if string(cc.Qualifier.Value) == prevCol {
					if cc.TimestampMicros > prevTime {
						t.Errorf("cell order is not correct: got %d > %d", cc.TimestampMicros, prevTime)
					}
				}
			}
			prevFam = cc.FamilyName.Value
			prevCol = string(cc.Qualifier.Value)
			prevTime = cc.TimestampMicros
		}
	}
	testOrder(mock)

	// Read with interleave filter
	inter := &btpb.RowFilter_Interleave{}
	fnr := &btpb.RowFilter{Filter: &btpb.RowFilter_FamilyNameRegexFilter{FamilyNameRegexFilter: "1"}}
	cqr := &btpb.RowFilter{Filter: &btpb.RowFilter_ColumnQualifierRegexFilter{ColumnQualifierRegexFilter: []byte("2")}}
	inter.Filters = append(inter.Filters, fnr, cqr)
	req = &btpb.ReadRowsRequest{
		TableName: tblInfo.Name,
		Rows:      &btpb.RowSet{RowKeys: [][]byte{[]byte("row")}},
		Filter: &btpb.RowFilter{
			Filter: &btpb.RowFilter_Interleave_{Interleave: inter},
		},
	}

	mock = &MockReadRowsServer{}
	if err = s.ReadRows(req, mock); err != nil {
		t.Errorf("ReadRows error: %v", err)
	}
	if len(mock.responses) == 0 {
		t.Fatal("Response count: got 0, want > 0")
	}
	if len(mock.responses[0].Chunks) != 18 {
		t.Fatalf("Chunk count: got %d, want 18", len(mock.responses[0].Chunks))
	}
	testOrder(mock)

	// Check order after ReadModifyWriteRow
	rmw := func(i int) *btpb.ReadModifyWriteRowRequest {
		return &btpb.ReadModifyWriteRowRequest{
			TableName: tblInfo.Name,
			RowKey:    []byte("row"),
			Rules: []*btpb.ReadModifyWriteRule{{
				FamilyName:      "cf3",
				ColumnQualifier: []byte("col" + strconv.Itoa(i)),
				Rule:            &btpb.ReadModifyWriteRule_IncrementAmount{IncrementAmount: 1},
			}},
		}
	}
	for i := count; i > 0; i-- {
		s.ReadModifyWriteRow(ctx, rmw(i))
	}
	req = &btpb.ReadRowsRequest{
		TableName: tblInfo.Name,
		Rows:      &btpb.RowSet{RowKeys: [][]byte{[]byte("row")}},
	}
	mock = &MockReadRowsServer{}
	if err = s.ReadRows(req, mock); err != nil {
		t.Errorf("ReadRows error: %v", err)
	}
	if len(mock.responses) == 0 {
		t.Fatal("Response count: got 0, want > 0")
	}
	if len(mock.responses[0].Chunks) != 30 {
		t.Fatalf("Chunk count: got %d, want 30", len(mock.responses[0].Chunks))
	}
	testOrder(mock)
}

func TestCheckAndMutateRowWithoutPredicate(t *testing.T) {
	s := &server{
		tables: make(map[string]*table),
	}
	ctx := context.Background()
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{MaxNumVersions: 1}}},
		},
	}
	tbl, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		t.Fatalf("Creating table: %v", err)
	}

	// Populate the table
	val := []byte("value")
	mrreq := &btpb.MutateRowRequest{
		TableName: tbl.Name,
		RowKey:    []byte("row-present"),
		Mutations: []*btpb.Mutation{{
			Mutation: &btpb.Mutation_SetCell_{SetCell: &btpb.Mutation_SetCell{
				FamilyName:      "cf",
				ColumnQualifier: []byte("col"),
				TimestampMicros: 0,
				Value:           val,
			}},
		}},
	}
	if _, err := s.MutateRow(ctx, mrreq); err != nil {
		t.Fatalf("Populating table: %v", err)
	}

	req := &btpb.CheckAndMutateRowRequest{
		TableName: tbl.Name,
		RowKey:    []byte("row-not-present"),
	}
	if res, err := s.CheckAndMutateRow(ctx, req); err != nil {
		t.Errorf("CheckAndMutateRow error: %v", err)
	} else if got, want := res.PredicateMatched, false; got != want {
		t.Errorf("Invalid PredicateMatched value: got %t, want %t", got, want)
	}

	req = &btpb.CheckAndMutateRowRequest{
		TableName: tbl.Name,
		RowKey:    []byte("row-present"),
	}
	if res, err := s.CheckAndMutateRow(ctx, req); err != nil {
		t.Errorf("CheckAndMutateRow error: %v", err)
	} else if got, want := res.PredicateMatched, true; got != want {
		t.Errorf("Invalid PredicateMatched value: got %t, want %t", got, want)
	}
}

// helper function to populate table data
func populateTable(ctx context.Context, s *server) (*btapb.Table, error) {
	newTbl := btapb.Table{
		ColumnFamilies: map[string]*btapb.ColumnFamily{
			"cf0": {GcRule: &btapb.GcRule{Rule: &btapb.GcRule_MaxNumVersions{1}}},
		},
	}
	tblInfo, err := s.CreateTable(ctx, &btapb.CreateTableRequest{Parent: "cluster", TableId: "t", Table: &newTbl})
	if err != nil {
		return nil, err
	}
	count := 3
	mcf := func(i int) *btapb.ModifyColumnFamiliesRequest {
		return &btapb.ModifyColumnFamiliesRequest{
			Name: tblInfo.Name,
			Modifications: []*btapb.ModifyColumnFamiliesRequest_Modification{{
				Id:  "cf" + strconv.Itoa(i),
				Mod: &btapb.ModifyColumnFamiliesRequest_Modification_Create{&btapb.ColumnFamily{}},
			}},
		}
	}
	for i := 1; i <= count; i++ {
		_, err = s.ModifyColumnFamilies(ctx, mcf(i))
		if err != nil {
			return nil, err
		}
	}
	// Populate the table
	for fc := 0; fc < count; fc++ {
		for cc := count; cc > 0; cc-- {
			for tc := 0; tc < count; tc++ {
				req := &btpb.MutateRowRequest{
					TableName: tblInfo.Name,
					RowKey:    []byte("row"),
					Mutations: []*btpb.Mutation{{
						Mutation: &btpb.Mutation_SetCell_{&btpb.Mutation_SetCell{
							FamilyName:      "cf" + strconv.Itoa(fc),
							ColumnQualifier: []byte("col" + strconv.Itoa(cc)),
							TimestampMicros: int64((tc + 1) * 1000),
							Value:           []byte{},
						}},
					}},
				}
				if _, err := s.MutateRow(ctx, req); err != nil {
					return nil, err
				}
			}
		}
	}

	return tblInfo, nil
}

func TestFilters(t *testing.T) {
	tests := []struct {
		in  *btpb.RowFilter
		out int
	}{
		{in: &btpb.RowFilter{Filter: &btpb.RowFilter_BlockAllFilter{true}}, out: 0},
		{in: &btpb.RowFilter{Filter: &btpb.RowFilter_BlockAllFilter{false}}, out: 1},
		{in: &btpb.RowFilter{Filter: &btpb.RowFilter_PassAllFilter{true}}, out: 1},
		{in: &btpb.RowFilter{Filter: &btpb.RowFilter_PassAllFilter{false}}, out: 0},
	}

	ctx := context.Background()

	s := &server{
		tables: make(map[string]*table),
	}

	tblInfo, err := populateTable(ctx, s)
	if err != nil {
		t.Fatal(err)
	}

	req := &btpb.ReadRowsRequest{
		TableName: tblInfo.Name,
		Rows:      &btpb.RowSet{RowKeys: [][]byte{[]byte("row")}},
	}

	for _, tc := range tests {
		req.Filter = tc.in

		mock := &MockReadRowsServer{}
		if err = s.ReadRows(req, mock); err != nil {
			t.Errorf("ReadRows error: %v", err)
			continue
		}

		if len(mock.responses) != tc.out {
			t.Errorf("Response count: got %d, want %d", len(mock.responses), tc.out)
			continue
		}
	}
}
