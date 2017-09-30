// Copyright 2015 Google Inc. All Rights Reserved.
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

package bigquery

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	gax "github.com/googleapis/gax-go"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/internal"
	"cloud.google.com/go/internal/pretty"
	"cloud.google.com/go/internal/testutil"
	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	client  *Client
	dataset *Dataset
	schema  = Schema{
		{Name: "name", Type: StringFieldType},
		{Name: "nums", Type: IntegerFieldType, Repeated: true},
		{Name: "rec", Type: RecordFieldType, Schema: Schema{
			{Name: "bool", Type: BooleanFieldType},
		}},
	}
	testTableExpiration time.Time
	datasetIDs          = testutil.NewUIDSpace("dataset")
)

func TestMain(m *testing.M) {
	initIntegrationTest()
	os.Exit(m.Run())
}

func getClient(t *testing.T) *Client {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	return client
}

// If integration tests will be run, create a unique bucket for them.
func initIntegrationTest() {
	flag.Parse() // needed for testing.Short()
	if testing.Short() {
		return
	}
	ctx := context.Background()
	ts := testutil.TokenSource(ctx, Scope)
	if ts == nil {
		log.Println("Integration tests skipped. See CONTRIBUTING.md for details")
		return
	}
	projID := testutil.ProjID()
	var err error
	client, err = NewClient(ctx, projID, option.WithTokenSource(ts))
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	dataset = client.Dataset("bigquery_integration_test")
	if err := dataset.Create(ctx, nil); err != nil && !hasStatusCode(err, http.StatusConflict) { // AlreadyExists is 409
		log.Fatalf("creating dataset: %v", err)
	}
	testTableExpiration = time.Now().Add(10 * time.Minute).Round(time.Second)
}

func TestIntegration_TableCreate(t *testing.T) {
	// Check that creating a record field with an empty schema is an error.
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	table := dataset.Table("t_bad")
	schema := Schema{
		{Name: "rec", Type: RecordFieldType, Schema: Schema{}},
	}
	err := table.Create(context.Background(), &TableMetadata{
		Schema:         schema,
		ExpirationTime: time.Now().Add(5 * time.Minute),
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !hasStatusCode(err, http.StatusBadRequest) {
		t.Fatalf("want a 400 error, got %v", err)
	}
}

func TestIntegration_TableCreateView(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	// Test that standard SQL views work.
	view := dataset.Table("t_view_standardsql")
	query := fmt.Sprintf("SELECT APPROX_COUNT_DISTINCT(name) FROM `%s.%s.%s`",
		dataset.ProjectID, dataset.DatasetID, table.TableID)
	err := view.Create(context.Background(), &TableMetadata{
		ViewQuery:      query,
		UseStandardSQL: true,
	})
	if err != nil {
		t.Fatalf("table.create: Did not expect an error, got: %v", err)
	}
	view.Delete(ctx)
}

func TestIntegration_TableMetadata(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)
	// Check table metadata.
	md, err := table.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// TODO(jba): check md more thorougly.
	if got, want := md.FullID, fmt.Sprintf("%s:%s.%s", dataset.ProjectID, dataset.DatasetID, table.TableID); got != want {
		t.Errorf("metadata.FullID: got %q, want %q", got, want)
	}
	if got, want := md.Type, RegularTable; got != want {
		t.Errorf("metadata.Type: got %v, want %v", got, want)
	}
	if got, want := md.ExpirationTime, testTableExpiration; !got.Equal(want) {
		t.Errorf("metadata.Type: got %v, want %v", got, want)
	}

	// Check that timePartitioning is nil by default
	if md.TimePartitioning != nil {
		t.Errorf("metadata.TimePartitioning: got %v, want %v", md.TimePartitioning, nil)
	}

	// Create tables that have time partitioning
	partitionCases := []struct {
		timePartitioning   TimePartitioning
		expectedExpiration time.Duration
	}{
		{TimePartitioning{}, time.Duration(0)},
		{TimePartitioning{time.Second}, time.Second},
	}
	for i, c := range partitionCases {
		table := dataset.Table(fmt.Sprintf("t_metadata_partition_%v", i))
		err = table.Create(context.Background(), &TableMetadata{
			Schema:           schema,
			TimePartitioning: &c.timePartitioning,
			ExpirationTime:   time.Now().Add(5 * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
		defer table.Delete(ctx)
		md, err = table.Metadata(ctx)
		if err != nil {
			t.Fatal(err)
		}

		got := md.TimePartitioning
		want := &TimePartitioning{c.expectedExpiration}
		if !testutil.Equal(got, want) {
			t.Errorf("metadata.TimePartitioning: got %v, want %v", got, want)
		}
	}
}

func TestIntegration_DatasetCreate(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	uid := strings.Replace(datasetIDs.New(), "-", "_", -1)
	ds := client.Dataset(uid)
	wmd := &DatasetMetadata{Name: "name", Location: "EU"}
	err := ds.Create(ctx, wmd)
	if err != nil {
		t.Fatal(err)
	}
	gmd, err := ds.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := gmd.Name, wmd.Name; got != want {
		t.Errorf("name: got %q, want %q", got, want)
	}
	if got, want := gmd.Location, wmd.Location; got != want {
		t.Errorf("location: got %q, want %q", got, want)
	}
	if err := ds.Delete(ctx); err != nil {
		t.Fatalf("deleting dataset %s: %v", ds, err)
	}
}

func TestIntegration_DatasetMetadata(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	md, err := dataset.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := md.FullID, fmt.Sprintf("%s:%s", dataset.ProjectID, dataset.DatasetID); got != want {
		t.Errorf("FullID: got %q, want %q", got, want)
	}
	jan2016 := time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)
	if md.CreationTime.Before(jan2016) {
		t.Errorf("CreationTime: got %s, want > 2016-1-1", md.CreationTime)
	}
	if md.LastModifiedTime.Before(jan2016) {
		t.Errorf("LastModifiedTime: got %s, want > 2016-1-1", md.LastModifiedTime)
	}

	// Verify that we get a NotFound for a nonexistent dataset.
	_, err = client.Dataset("does_not_exist").Metadata(ctx)
	if err == nil || !hasStatusCode(err, http.StatusNotFound) {
		t.Errorf("got %v, want NotFound error", err)
	}
}

func TestIntegration_DatasetDelete(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	ds := client.Dataset("delete_test")
	if err := ds.Create(ctx, nil); err != nil && !hasStatusCode(err, http.StatusConflict) { // AlreadyExists is 409
		t.Fatalf("creating dataset %s: %v", ds, err)
	}
	if err := ds.Delete(ctx); err != nil {
		t.Fatalf("deleting dataset %s: %v", ds, err)
	}
}

func TestIntegration_DatasetUpdateETags(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}

	check := func(md *DatasetMetadata, wantDesc, wantName string) {
		if md.Description != wantDesc {
			t.Errorf("description: got %q, want %q", md.Description, wantDesc)
		}
		if md.Name != wantName {
			t.Errorf("name: got %q, want %q", md.Name, wantName)
		}
	}

	ctx := context.Background()
	md, err := dataset.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if md.ETag == "" {
		t.Fatal("empty ETag")
	}
	// Write without ETag succeeds.
	desc := md.Description + "d2"
	name := md.Name + "n2"
	md2, err := dataset.Update(ctx, DatasetMetadataToUpdate{Description: desc, Name: name}, "")
	if err != nil {
		t.Fatal(err)
	}
	check(md2, desc, name)

	// Write with original ETag fails because of intervening write.
	_, err = dataset.Update(ctx, DatasetMetadataToUpdate{Description: "d", Name: "n"}, md.ETag)
	if err == nil {
		t.Fatal("got nil, want error")
	}

	// Write with most recent ETag succeeds.
	md3, err := dataset.Update(ctx, DatasetMetadataToUpdate{Description: "", Name: ""}, md2.ETag)
	if err != nil {
		t.Fatal(err)
	}
	check(md3, "", "")
}

func TestIntegration_DatasetUpdateDefaultExpiration(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	md, err := dataset.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Set the default expiration time.
	md, err = dataset.Update(ctx, DatasetMetadataToUpdate{DefaultTableExpiration: time.Hour}, "")
	if err != nil {
		t.Fatal(err)
	}
	if md.DefaultTableExpiration != time.Hour {
		t.Fatalf("got %s, want 1h", md.DefaultTableExpiration)
	}
	// Omitting DefaultTableExpiration doesn't change it.
	md, err = dataset.Update(ctx, DatasetMetadataToUpdate{Name: "xyz"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if md.DefaultTableExpiration != time.Hour {
		t.Fatalf("got %s, want 1h", md.DefaultTableExpiration)
	}
	// Setting it to 0 deletes it (which looks like a 0 duration).
	md, err = dataset.Update(ctx, DatasetMetadataToUpdate{DefaultTableExpiration: time.Duration(0)}, "")
	if err != nil {
		t.Fatal(err)
	}
	if md.DefaultTableExpiration != 0 {
		t.Fatalf("got %s, want 0", md.DefaultTableExpiration)
	}
}

func TestIntegration_DatasetUpdateLabels(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	md, err := dataset.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// TODO(jba): use a separate dataset for each test run so
	// tests don't interfere with each other.
	var dm DatasetMetadataToUpdate
	dm.SetLabel("label", "value")
	md, err = dataset.Update(ctx, dm, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := md.Labels["label"], "value"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	dm = DatasetMetadataToUpdate{}
	dm.DeleteLabel("label")
	md, err = dataset.Update(ctx, dm, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := md.Labels["label"]; ok {
		t.Error("label still present after deletion")
	}
}

func TestIntegration_Tables(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)
	wantName := table.FullyQualifiedName()

	// This test is flaky due to eventual consistency.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := internal.Retry(ctx, gax.Backoff{}, func() (stop bool, err error) {
		// Iterate over tables in the dataset.
		it := dataset.Tables(ctx)
		var tableNames []string
		for {
			tbl, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return false, err
			}
			tableNames = append(tableNames, tbl.FullyQualifiedName())
		}
		// Other tests may be running with this dataset, so there might be more
		// than just our table in the list. So don't try for an exact match; just
		// make sure that our table is there somewhere.
		for _, tn := range tableNames {
			if tn == wantName {
				return true, nil
			}
		}
		return false, fmt.Errorf("got %v\nwant %s in the list", tableNames, wantName)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_UploadAndRead(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	// Populate the table.
	upl := table.Uploader()
	var (
		wantRows  [][]Value
		saverRows []*ValuesSaver
	)
	for i, name := range []string{"a", "b", "c"} {
		row := []Value{name, []Value{int64(i)}, []Value{true}}
		wantRows = append(wantRows, row)
		saverRows = append(saverRows, &ValuesSaver{
			Schema:   schema,
			InsertID: name,
			Row:      row,
		})
	}
	if err := upl.Put(ctx, saverRows); err != nil {
		t.Fatal(putError(err))
	}

	// Wait until the data has been uploaded. This can take a few seconds, according
	// to https://cloud.google.com/bigquery/streaming-data-into-bigquery.
	if err := waitForRow(ctx, table); err != nil {
		t.Fatal(err)
	}

	// Read the table.
	checkRead(t, "upload", table.Read(ctx), wantRows)

	// Query the table.
	q := client.Query(fmt.Sprintf("select name, nums, rec from %s", table.TableID))
	q.UseStandardSQL = true
	q.DefaultProjectID = dataset.ProjectID
	q.DefaultDatasetID = dataset.DatasetID

	rit, err := q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkRead(t, "query", rit, wantRows)

	// Query the long way.
	job1, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	job2, err := client.JobFromID(ctx, job1.ID())
	if err != nil {
		t.Fatal(err)
	}
	rit, err = job2.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkRead(t, "job.Read", rit, wantRows)

	// Get statistics.
	jobStatus, err := job2.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if jobStatus.Statistics == nil {
		t.Fatal("jobStatus missing statistics")
	}
	if _, ok := jobStatus.Statistics.Details.(*QueryStatistics); !ok {
		t.Errorf("expected QueryStatistics, got %T", jobStatus.Statistics.Details)
	}

	// Test reading directly into a []Value.
	valueLists, err := readAll(table.Read(ctx))
	if err != nil {
		t.Fatal(err)
	}
	it := table.Read(ctx)
	for i, vl := range valueLists {
		var got []Value
		if err := it.Next(&got); err != nil {
			t.Fatal(err)
		}
		want := []Value(vl)
		if !testutil.Equal(got, want) {
			t.Errorf("%d: got %v, want %v", i, got, want)
		}
	}

	// Test reading into a map.
	it = table.Read(ctx)
	for _, vl := range valueLists {
		var vm map[string]Value
		if err := it.Next(&vm); err != nil {
			t.Fatal(err)
		}
		if got, want := len(vm), len(vl); got != want {
			t.Fatalf("valueMap len: got %d, want %d", got, want)
		}
		// With maps, structs become nested maps.
		vl[2] = map[string]Value{"bool": vl[2].([]Value)[0]}
		for i, v := range vl {
			if got, want := vm[schema[i].Name], v; !testutil.Equal(got, want) {
				t.Errorf("%d, name=%s: got %#v, want %#v",
					i, schema[i].Name, got, want)
			}
		}
	}
}

type SubSubTestStruct struct {
	Integer int64
}

type SubTestStruct struct {
	String      string
	Record      SubSubTestStruct
	RecordArray []SubSubTestStruct
}

type TestStruct struct {
	Name      string
	Bytes     []byte
	Integer   int64
	Float     float64
	Boolean   bool
	Timestamp time.Time
	Date      civil.Date
	Time      civil.Time
	DateTime  civil.DateTime

	StringArray    []string
	IntegerArray   []int64
	FloatArray     []float64
	BooleanArray   []bool
	TimestampArray []time.Time
	DateArray      []civil.Date
	TimeArray      []civil.Time
	DateTimeArray  []civil.DateTime

	Record      SubTestStruct
	RecordArray []SubTestStruct
}

func TestIntegration_UploadAndReadStructs(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	schema, err := InferSchema(TestStruct{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	d := civil.Date{2016, 3, 20}
	tm := civil.Time{15, 4, 5, 0}
	ts := time.Date(2016, 3, 20, 15, 4, 5, 0, time.UTC)
	dtm := civil.DateTime{d, tm}

	d2 := civil.Date{1994, 5, 15}
	tm2 := civil.Time{1, 2, 4, 0}
	ts2 := time.Date(1994, 5, 15, 1, 2, 4, 0, time.UTC)
	dtm2 := civil.DateTime{d2, tm2}

	// Populate the table.
	upl := table.Uploader()
	want := []*TestStruct{
		{
			"a",
			[]byte("byte"),
			42,
			3.14,
			true,
			ts,
			d,
			tm,
			dtm,
			[]string{"a", "b"},
			[]int64{1, 2},
			[]float64{1, 1.41},
			[]bool{true, false},
			[]time.Time{ts, ts2},
			[]civil.Date{d, d2},
			[]civil.Time{tm, tm2},
			[]civil.DateTime{dtm, dtm2},
			SubTestStruct{
				"string",
				SubSubTestStruct{24},
				[]SubSubTestStruct{{1}, {2}},
			},
			[]SubTestStruct{
				{String: "empty"},
				{
					"full",
					SubSubTestStruct{1},
					[]SubSubTestStruct{{1}, {2}},
				},
			},
		},
		{
			Name:      "b",
			Bytes:     []byte("byte2"),
			Integer:   24,
			Float:     4.13,
			Boolean:   false,
			Timestamp: ts,
			Date:      d,
			Time:      tm,
			DateTime:  dtm,
		},
	}
	var savers []*StructSaver
	for _, s := range want {
		savers = append(savers, &StructSaver{Schema: schema, Struct: s})
	}
	if err := upl.Put(ctx, savers); err != nil {
		t.Fatal(putError(err))
	}

	// Wait until the data has been uploaded. This can take a few seconds, according
	// to https://cloud.google.com/bigquery/streaming-data-into-bigquery.
	if err := waitForRow(ctx, table); err != nil {
		t.Fatal(err)
	}

	// Test iteration with structs.
	it := table.Read(ctx)
	var got []*TestStruct
	for {
		var g TestStruct
		err := it.Next(&g)
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, &g)
	}
	sort.Sort(byName(got))

	// BigQuery does not elide nils. It reports an error for nil fields.
	for i, g := range got {
		if i >= len(want) {
			t.Errorf("%d: got %v, past end of want", i, pretty.Value(g))
		} else if w := want[i]; !testutil.Equal(g, w) {
			t.Errorf("%d: got %v, want %v", i, pretty.Value(g), pretty.Value(w))
		}
	}
}

type byName []*TestStruct

func (b byName) Len() int           { return len(b) }
func (b byName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byName) Less(i, j int) bool { return b[i].Name < b[j].Name }

func TestIntegration_TableUpdate(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	// Test Update of non-schema fields.
	tm, err := table.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantDescription := tm.Description + "more"
	wantName := tm.Name + "more"
	wantExpiration := tm.ExpirationTime.Add(time.Hour * 24)
	got, err := table.Update(ctx, TableMetadataToUpdate{
		Description:    wantDescription,
		Name:           wantName,
		ExpirationTime: wantExpiration,
	}, tm.ETag)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != wantDescription {
		t.Errorf("Description: got %q, want %q", got.Description, wantDescription)
	}
	if got.Name != wantName {
		t.Errorf("Name: got %q, want %q", got.Name, wantName)
	}
	if got.ExpirationTime != wantExpiration {
		t.Errorf("ExpirationTime: got %q, want %q", got.ExpirationTime, wantExpiration)
	}
	if !testutil.Equal(got.Schema, schema) {
		t.Errorf("Schema: got %v, want %v", pretty.Value(got.Schema), pretty.Value(schema))
	}

	// Blind write succeeds.
	_, err = table.Update(ctx, TableMetadataToUpdate{Name: "x"}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Write with old etag fails.
	_, err = table.Update(ctx, TableMetadataToUpdate{Name: "y"}, got.ETag)
	if err == nil {
		t.Fatal("Update with old ETag succeeded, wanted failure")
	}

	// Test schema update.
	// Columns can be added. schema2 is the same as schema, except for the
	// added column in the middle.
	nested := Schema{
		{Name: "nested", Type: BooleanFieldType},
		{Name: "other", Type: StringFieldType},
	}
	schema2 := Schema{
		schema[0],
		{Name: "rec2", Type: RecordFieldType, Schema: nested},
		schema[1],
		schema[2],
	}

	got, err = table.Update(ctx, TableMetadataToUpdate{Schema: schema2}, "")
	if err != nil {
		t.Fatal(err)
	}

	// Wherever you add the column, it appears at the end.
	schema3 := Schema{schema2[0], schema2[2], schema2[3], schema2[1]}
	if !testutil.Equal(got.Schema, schema3) {
		t.Errorf("add field:\ngot  %v\nwant %v",
			pretty.Value(got.Schema), pretty.Value(schema3))
	}

	// Updating with the empty schema succeeds, but is a no-op.
	got, err = table.Update(ctx, TableMetadataToUpdate{Schema: Schema{}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(got.Schema, schema3) {
		t.Errorf("empty schema:\ngot  %v\nwant %v",
			pretty.Value(got.Schema), pretty.Value(schema3))
	}

	// Error cases when updating schema.
	for _, test := range []struct {
		desc   string
		fields []*FieldSchema
	}{
		{"change from optional to required", []*FieldSchema{
			{Name: "name", Type: StringFieldType, Required: true},
			schema3[1],
			schema3[2],
			schema3[3],
		}},
		{"add a required field", []*FieldSchema{
			schema3[0], schema3[1], schema3[2], schema3[3],
			{Name: "req", Type: StringFieldType, Required: true},
		}},
		{"remove a field", []*FieldSchema{schema3[0], schema3[1], schema3[2]}},
		{"remove a nested field", []*FieldSchema{
			schema3[0], schema3[1], schema3[2],
			{Name: "rec2", Type: RecordFieldType, Schema: Schema{nested[0]}}}},
		{"remove all nested fields", []*FieldSchema{
			schema3[0], schema3[1], schema3[2],
			{Name: "rec2", Type: RecordFieldType, Schema: Schema{}}}},
	} {
		_, err = table.Update(ctx, TableMetadataToUpdate{Schema: Schema(test.fields)}, "")
		if err == nil {
			t.Errorf("%s: want error, got nil", test.desc)
		} else if !hasStatusCode(err, 400) {
			t.Errorf("%s: want 400, got %v", test.desc, err)
		}
	}
}

func TestIntegration_Load(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	// CSV data can't be loaded into a repeated field, so we use a different schema.
	table := newTable(t, Schema{
		{Name: "name", Type: StringFieldType},
		{Name: "nums", Type: IntegerFieldType},
	})
	defer table.Delete(ctx)

	// Load the table from a reader.
	r := strings.NewReader("a,0\nb,1\nc,2\n")
	wantRows := [][]Value{
		[]Value{"a", int64(0)},
		[]Value{"b", int64(1)},
		[]Value{"c", int64(2)},
	}
	rs := NewReaderSource(r)
	loader := table.LoaderFrom(rs)
	loader.WriteDisposition = WriteTruncate
	job, err := loader.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := wait(ctx, job); err != nil {
		t.Fatal(err)
	}
	checkRead(t, "reader load", table.Read(ctx), wantRows)
}

func TestIntegration_DML(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	// Retry insert; sometimes it fails with INTERNAL.
	err := internal.Retry(ctx, gax.Backoff{}, func() (bool, error) {
		table := newTable(t, schema)
		defer table.Delete(ctx)

		// Use DML to insert.
		wantRows := [][]Value{
			[]Value{"a", []Value{int64(0)}, []Value{true}},
			[]Value{"b", []Value{int64(1)}, []Value{false}},
			[]Value{"c", []Value{int64(2)}, []Value{true}},
		}
		query := fmt.Sprintf("INSERT bigquery_integration_test.%s (name, nums, rec) "+
			"VALUES ('a', [0], STRUCT<BOOL>(TRUE)), ('b', [1], STRUCT<BOOL>(FALSE)), ('c', [2], STRUCT<BOOL>(TRUE))",
			table.TableID)
		q := client.Query(query)
		q.UseStandardSQL = true // necessary for DML
		job, err := q.Run(ctx)
		if err != nil {
			if e, ok := err.(*googleapi.Error); ok && e.Code < 500 {
				return true, err // fail on 4xx
			}
			return false, err
		}
		if err := wait(ctx, job); err != nil {
			fmt.Printf("wait: %v\n", err)
			return false, err
		}
		if msg, ok := compareRead(table.Read(ctx), wantRows); !ok {
			// Stop on read error, because that has never been flaky.
			return true, errors.New(msg)
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_TimeTypes(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	dtSchema := Schema{
		{Name: "d", Type: DateFieldType},
		{Name: "t", Type: TimeFieldType},
		{Name: "dt", Type: DateTimeFieldType},
		{Name: "ts", Type: TimestampFieldType},
	}
	table := newTable(t, dtSchema)
	defer table.Delete(ctx)

	d := civil.Date{2016, 3, 20}
	tm := civil.Time{12, 30, 0, 0}
	ts := time.Date(2016, 3, 20, 15, 04, 05, 0, time.UTC)
	wantRows := [][]Value{
		[]Value{d, tm, civil.DateTime{d, tm}, ts},
	}
	upl := table.Uploader()
	if err := upl.Put(ctx, []*ValuesSaver{
		{Schema: dtSchema, Row: wantRows[0]},
	}); err != nil {
		t.Fatal(putError(err))
	}
	if err := waitForRow(ctx, table); err != nil {
		t.Fatal(err)
	}

	// SQL wants DATETIMEs with a space between date and time, but the service
	// returns them in RFC3339 form, with a "T" between.
	query := fmt.Sprintf("INSERT bigquery_integration_test.%s (d, t, dt, ts) "+
		"VALUES ('%s', '%s', '%s %s', '%s')",
		table.TableID, d, tm, d, tm, ts.Format("2006-01-02 15:04:05"))
	q := client.Query(query)
	q.UseStandardSQL = true // necessary for DML
	job, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := wait(ctx, job); err != nil {
		t.Fatal(err)
	}
	wantRows = append(wantRows, wantRows[0])
	checkRead(t, "TimeTypes", table.Read(ctx), wantRows)
}

func TestIntegration_StandardQuery(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()

	d := civil.Date{2016, 3, 20}
	tm := civil.Time{15, 04, 05, 0}
	ts := time.Date(2016, 3, 20, 15, 04, 05, 0, time.UTC)
	dtm := ts.Format("2006-01-02 15:04:05")

	// Constructs Value slices made up of int64s.
	ints := func(args ...int) []Value {
		vals := make([]Value, len(args))
		for i, arg := range args {
			vals[i] = int64(arg)
		}
		return vals
	}

	testCases := []struct {
		query   string
		wantRow []Value
	}{
		{"SELECT 1", ints(1)},
		{"SELECT 1.3", []Value{1.3}},
		{"SELECT TRUE", []Value{true}},
		{"SELECT 'ABC'", []Value{"ABC"}},
		{"SELECT CAST('foo' AS BYTES)", []Value{[]byte("foo")}},
		{fmt.Sprintf("SELECT TIMESTAMP '%s'", dtm), []Value{ts}},
		{fmt.Sprintf("SELECT [TIMESTAMP '%s', TIMESTAMP '%s']", dtm, dtm), []Value{[]Value{ts, ts}}},
		{fmt.Sprintf("SELECT ('hello', TIMESTAMP '%s')", dtm), []Value{[]Value{"hello", ts}}},
		{fmt.Sprintf("SELECT DATETIME(TIMESTAMP '%s')", dtm), []Value{civil.DateTime{d, tm}}},
		{fmt.Sprintf("SELECT DATE(TIMESTAMP '%s')", dtm), []Value{d}},
		{fmt.Sprintf("SELECT TIME(TIMESTAMP '%s')", dtm), []Value{tm}},
		{"SELECT (1, 2)", []Value{ints(1, 2)}},
		{"SELECT [1, 2, 3]", []Value{ints(1, 2, 3)}},
		{"SELECT ([1, 2], 3, [4, 5])", []Value{[]Value{ints(1, 2), int64(3), ints(4, 5)}}},
		{"SELECT [(1, 2, 3), (4, 5, 6)]", []Value{[]Value{ints(1, 2, 3), ints(4, 5, 6)}}},
		{"SELECT [([1, 2, 3], 4), ([5, 6], 7)]", []Value{[]Value{[]Value{ints(1, 2, 3), int64(4)}, []Value{ints(5, 6), int64(7)}}}},
		{"SELECT ARRAY(SELECT STRUCT([1, 2]))", []Value{[]Value{[]Value{ints(1, 2)}}}},
	}
	for _, c := range testCases {
		q := client.Query(c.query)
		q.UseStandardSQL = true
		it, err := q.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkRead(t, "StandardQuery", it, [][]Value{c.wantRow})
	}
}

func TestIntegration_LegacyQuery(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()

	ts := time.Date(2016, 3, 20, 15, 04, 05, 0, time.UTC)
	dtm := ts.Format("2006-01-02 15:04:05")

	testCases := []struct {
		query   string
		wantRow []Value
	}{
		{"SELECT 1", []Value{int64(1)}},
		{"SELECT 1.3", []Value{1.3}},
		{"SELECT TRUE", []Value{true}},
		{"SELECT 'ABC'", []Value{"ABC"}},
		{"SELECT CAST('foo' AS BYTES)", []Value{[]byte("foo")}},
		{fmt.Sprintf("SELECT TIMESTAMP('%s')", dtm), []Value{ts}},
		{fmt.Sprintf("SELECT DATE(TIMESTAMP('%s'))", dtm), []Value{"2016-03-20"}},
		{fmt.Sprintf("SELECT TIME(TIMESTAMP('%s'))", dtm), []Value{"15:04:05"}},
	}
	for _, c := range testCases {
		q := client.Query(c.query)
		q.UseLegacySQL = true
		it, err := q.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkRead(t, "LegacyQuery", it, [][]Value{c.wantRow})
	}
}

func TestIntegration_QueryParameters(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()

	d := civil.Date{2016, 3, 20}
	tm := civil.Time{15, 04, 05, 0}
	dtm := civil.DateTime{d, tm}
	ts := time.Date(2016, 3, 20, 15, 04, 05, 0, time.UTC)

	type ss struct {
		String string
	}

	type s struct {
		Timestamp      time.Time
		StringArray    []string
		SubStruct      ss
		SubStructArray []ss
	}

	testCases := []struct {
		query      string
		parameters []QueryParameter
		wantRow    []Value
	}{
		{"SELECT @val", []QueryParameter{{"val", 1}}, []Value{int64(1)}},
		{"SELECT @val", []QueryParameter{{"val", 1.3}}, []Value{1.3}},
		{"SELECT @val", []QueryParameter{{"val", true}}, []Value{true}},
		{"SELECT @val", []QueryParameter{{"val", "ABC"}}, []Value{"ABC"}},
		{"SELECT @val", []QueryParameter{{"val", []byte("foo")}}, []Value{[]byte("foo")}},
		{"SELECT @val", []QueryParameter{{"val", ts}}, []Value{ts}},
		{"SELECT @val", []QueryParameter{{"val", []time.Time{ts, ts}}}, []Value{[]Value{ts, ts}}},
		{"SELECT @val", []QueryParameter{{"val", dtm}}, []Value{dtm}},
		{"SELECT @val", []QueryParameter{{"val", d}}, []Value{d}},
		{"SELECT @val", []QueryParameter{{"val", tm}}, []Value{tm}},
		{"SELECT @val", []QueryParameter{{"val", s{ts, []string{"a", "b"}, ss{"c"}, []ss{{"d"}, {"e"}}}}},
			[]Value{[]Value{ts, []Value{"a", "b"}, []Value{"c"}, []Value{[]Value{"d"}, []Value{"e"}}}}},
		{"SELECT @val.Timestamp, @val.SubStruct.String", []QueryParameter{{"val", s{Timestamp: ts, SubStruct: ss{"a"}}}}, []Value{ts, "a"}},
	}
	for _, c := range testCases {
		q := client.Query(c.query)
		q.Parameters = c.parameters
		it, err := q.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkRead(t, "QueryParameters", it, [][]Value{c.wantRow})
	}
}

func TestIntegration_ReadNullIntoStruct(t *testing.T) {
	// Reading a null into a struct field should return an error (not panic).
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	upl := table.Uploader()
	row := &ValuesSaver{
		Schema: schema,
		Row:    []Value{nil, []Value{}, []Value{nil}},
	}
	if err := upl.Put(ctx, []*ValuesSaver{row}); err != nil {
		t.Fatal(putError(err))
	}
	if err := waitForRow(ctx, table); err != nil {
		t.Fatal(err)
	}

	q := client.Query(fmt.Sprintf("select name from %s", table.TableID))
	q.DefaultProjectID = dataset.ProjectID
	q.DefaultDatasetID = dataset.DatasetID
	it, err := q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	type S struct{ Name string }
	var s S
	if err := it.Next(&s); err == nil {
		t.Fatal("got nil, want error")
	}
}

const (
	stdName    = "`bigquery-public-data.samples.shakespeare`"
	legacyName = "[bigquery-public-data:samples.shakespeare]"
)

// These tests exploit the fact that the two SQL versions have different syntaxes for
// fully-qualified table names.
var useLegacySqlTests = []struct {
	t           string // name of table
	std, legacy bool   // use standard/legacy SQL
	err         bool   // do we expect an error?
}{
	{t: legacyName, std: false, legacy: true, err: false},
	{t: legacyName, std: true, legacy: false, err: true},
	{t: legacyName, std: false, legacy: false, err: true}, // standard SQL is default
	{t: legacyName, std: true, legacy: true, err: true},
	{t: stdName, std: false, legacy: true, err: true},
	{t: stdName, std: true, legacy: false, err: false},
	{t: stdName, std: false, legacy: false, err: false}, // standard SQL is default
	{t: stdName, std: true, legacy: true, err: true},
}

func TestIntegration_QueryUseLegacySQL(t *testing.T) {
	// Test the UseLegacySQL and UseStandardSQL options for queries.
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	for _, test := range useLegacySqlTests {
		q := client.Query(fmt.Sprintf("select word from %s limit 1", test.t))
		q.UseStandardSQL = test.std
		q.UseLegacySQL = test.legacy
		_, err := q.Read(ctx)
		gotErr := err != nil
		if gotErr && !test.err {
			t.Errorf("%+v:\nunexpected error: %v", test, err)
		} else if !gotErr && test.err {
			t.Errorf("%+v:\nsucceeded, but want error", test)
		}
	}
}

func TestIntegration_TableUseLegacySQL(t *testing.T) {
	// Test UseLegacySQL and UseStandardSQL for Table.Create.
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)
	for i, test := range useLegacySqlTests {
		view := dataset.Table(fmt.Sprintf("t_view_%d", i))
		tm := &TableMetadata{
			ViewQuery:      fmt.Sprintf("SELECT word from %s", test.t),
			UseStandardSQL: test.std,
			UseLegacySQL:   test.legacy,
		}
		err := view.Create(ctx, tm)
		gotErr := err != nil
		if gotErr && !test.err {
			t.Errorf("%+v:\nunexpected error: %v", test, err)
		} else if !gotErr && test.err {
			t.Errorf("%+v:\nsucceeded, but want error", test)
		}
		view.Delete(ctx)
	}
}

func TestIntegration_ListJobs(t *testing.T) {
	// It's difficult to test the list of jobs, because we can't easily
	// control what's in it. Also, there are many jobs in the test project,
	// and it takes considerable time to list them all.
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()

	// About all we can do is list a few jobs.
	const max = 20
	var jis []JobInfo
	it := client.Jobs(ctx)
	for {
		ji, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		jis = append(jis, ji)
		if len(jis) >= max {
			break
		}
	}
	// We expect that there is at least one job in the last few months.
	if len(jis) == 0 {
		t.Fatal("did not get any jobs")
	}
}

// Creates a new, temporary table with a unique name and the given schema.
func newTable(t *testing.T, s Schema) *Table {
	name := fmt.Sprintf("t%d", time.Now().UnixNano())
	table := dataset.Table(name)
	err := table.Create(context.Background(), &TableMetadata{
		Schema:         s,
		ExpirationTime: testTableExpiration,
	})
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func checkRead(t *testing.T, msg string, it *RowIterator, want [][]Value) {
	if msg2, ok := compareRead(it, want); !ok {
		t.Errorf("%s: %s", msg, msg2)
	}
}

func compareRead(it *RowIterator, want [][]Value) (msg string, ok bool) {
	got, err := readAll(it)
	if err != nil {
		return err.Error(), false
	}
	if len(got) != len(want) {
		return fmt.Sprintf("got %d rows, want %d", len(got), len(want)), false
	}
	sort.Sort(byCol0(got))
	for i, r := range got {
		gotRow := []Value(r)
		wantRow := want[i]
		if !testutil.Equal(gotRow, wantRow) {
			return fmt.Sprintf("#%d: got %#v, want %#v", i, gotRow, wantRow), false
		}
	}
	return "", true
}

func readAll(it *RowIterator) ([][]Value, error) {
	var rows [][]Value
	for {
		var vals []Value
		err := it.Next(&vals)
		if err == iterator.Done {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, vals)
	}
}

type byCol0 [][]Value

func (b byCol0) Len() int      { return len(b) }
func (b byCol0) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byCol0) Less(i, j int) bool {
	switch a := b[i][0].(type) {
	case string:
		return a < b[j][0].(string)
	case civil.Date:
		return a.Before(b[j][0].(civil.Date))
	default:
		panic("unknown type")
	}
}

func hasStatusCode(err error, code int) bool {
	if e, ok := err.(*googleapi.Error); ok && e.Code == code {
		return true
	}
	return false
}

// wait polls the job until it is complete or an error is returned.
func wait(ctx context.Context, job *Job) error {
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("getting job status: %v", err)
	}
	if status.Err() != nil {
		return fmt.Errorf("job status error: %#v", status.Err())
	}
	if status.Statistics == nil {
		return errors.New("nil Statistics")
	}
	if status.Statistics.EndTime.IsZero() {
		return errors.New("EndTime is zero")
	}
	if status.Statistics.Details == nil {
		return errors.New("nil Statistics.Details")
	}
	return nil
}

// waitForRow polls the table until it contains a row.
// TODO(jba): use internal.Retry.
func waitForRow(ctx context.Context, table *Table) error {
	for {
		it := table.Read(ctx)
		var v []Value
		err := it.Next(&v)
		if err == nil {
			return nil
		}
		if err != iterator.Done {
			return err
		}
		time.Sleep(1 * time.Second)
	}
}

func putError(err error) string {
	pme, ok := err.(PutMultiError)
	if !ok {
		return err.Error()
	}
	var msgs []string
	for _, err := range pme {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "\n")
}
