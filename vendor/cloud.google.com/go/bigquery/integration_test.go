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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	gax "github.com/googleapis/gax-go"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/internal"
	"cloud.google.com/go/internal/pretty"
	"cloud.google.com/go/internal/testutil"
	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	client        *Client
	storageClient *storage.Client
	dataset       *Dataset
	schema        = Schema{
		{Name: "name", Type: StringFieldType},
		{Name: "nums", Type: IntegerFieldType, Repeated: true},
		{Name: "rec", Type: RecordFieldType, Schema: Schema{
			{Name: "bool", Type: BooleanFieldType},
		}},
	}
	testTableExpiration time.Time
	// BigQuery does not accept hyphens in dataset or table IDs, so we create IDs
	// with underscores.
	datasetIDs = testutil.NewUIDSpaceSep("dataset", '_')
	tableIDs   = testutil.NewUIDSpaceSep("table", '_')
)

// Note: integration tests cannot be run in parallel, because TestIntegration_Location
// modifies the client.

func TestMain(m *testing.M) {
	cleanup := initIntegrationTest()
	r := m.Run()
	cleanup()
	os.Exit(r)
}

func getClient(t *testing.T) *Client {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	return client
}

// If integration tests will be run, create a unique bucket for them.
func initIntegrationTest() func() {
	flag.Parse() // needed for testing.Short()
	if testing.Short() {
		return func() {}
	}
	ctx := context.Background()
	ts := testutil.TokenSource(ctx, Scope)
	if ts == nil {
		log.Println("Integration tests skipped. See CONTRIBUTING.md for details")
		return func() {}
	}
	projID := testutil.ProjID()
	var err error
	client, err = NewClient(ctx, projID, option.WithTokenSource(ts))
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	storageClient, err = storage.NewClient(ctx,
		option.WithTokenSource(testutil.TokenSource(ctx, storage.ScopeFullControl)))
	if err != nil {
		log.Fatalf("storage.NewClient: %v", err)
	}
	dataset = client.Dataset(datasetIDs.New())
	if err := dataset.Create(ctx, nil); err != nil {
		log.Fatalf("creating dataset %s: %v", dataset.DatasetID, err)
	}
	testTableExpiration = time.Now().Add(10 * time.Minute).Round(time.Second)
	return func() {
		if err := deleteDataset(ctx, dataset); err != nil {
			log.Printf("could not delete %s", dataset.DatasetID)
		}
	}
}

func deleteDataset(ctx context.Context, ds *Dataset) error {
	it := ds.Tables(ctx)
	for {
		tbl, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if err := tbl.Delete(ctx); err != nil {
			return err
		}
	}
	return ds.Delete(ctx)
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
		timePartitioning TimePartitioning
		wantExpiration   time.Duration
		wantField        string
	}{
		{TimePartitioning{}, time.Duration(0), ""},
		{TimePartitioning{Expiration: time.Second}, time.Second, ""},
		{
			TimePartitioning{
				Expiration: time.Second,
				Field:      "date",
			}, time.Second, "date"},
	}

	schema2 := Schema{
		{Name: "name", Type: StringFieldType},
		{Name: "date", Type: DateFieldType},
	}

	for i, c := range partitionCases {
		table := dataset.Table(fmt.Sprintf("t_metadata_partition_%v", i))
		err = table.Create(context.Background(), &TableMetadata{
			Schema:           schema2,
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
		want := &TimePartitioning{
			Expiration: c.wantExpiration,
			Field:      c.wantField,
		}
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
	ds := client.Dataset(datasetIDs.New())
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
		t.Fatalf("deleting dataset %v: %v", ds, err)
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
	ds := client.Dataset(datasetIDs.New())
	if err := ds.Create(ctx, nil); err != nil {
		t.Fatalf("creating dataset %s: %v", ds.DatasetID, err)
	}
	if err := ds.Delete(ctx); err != nil {
		t.Fatalf("deleting dataset %s: %v", ds.DatasetID, err)
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

func TestIntegration_DatasetUpdateAccess(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	md, err := dataset.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	origAccess := append([]*AccessEntry(nil), md.Access...)
	newEntry := &AccessEntry{
		Role:       ReaderRole,
		Entity:     "Joe@example.com",
		EntityType: UserEmailEntity,
	}
	newAccess := append(md.Access, newEntry)
	dm := DatasetMetadataToUpdate{Access: newAccess}
	md, err = dataset.Update(ctx, dm, md.ETag)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := dataset.Update(ctx, DatasetMetadataToUpdate{Access: origAccess}, md.ETag)
		if err != nil {
			t.Log("could not restore dataset access list")
		}
	}()
	if diff := testutil.Diff(md.Access, newAccess); diff != "" {
		t.Fatalf("got=-, want=+:\n%s", diff)
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

func TestIntegration_TableUpdateLabels(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	var tm TableMetadataToUpdate
	tm.SetLabel("label", "value")
	md, err := table.Update(ctx, tm, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := md.Labels["label"], "value"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	tm = TableMetadataToUpdate{}
	tm.DeleteLabel("label")
	md, err = table.Update(ctx, tm, "")
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
	if job1.LastStatus() == nil {
		t.Error("no LastStatus")
	}
	job2, err := client.JobFromID(ctx, job1.ID())
	if err != nil {
		t.Fatal(err)
	}
	if job2.LastStatus() == nil {
		t.Error("no LastStatus")
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
	valueLists, schema, _, err := readAll(table.Read(ctx))
	if err != nil {
		t.Fatal(err)
	}
	it := table.Read(ctx)
	for i, vl := range valueLists {
		var got []Value
		if err := it.Next(&got); err != nil {
			t.Fatal(err)
		}
		if !testutil.Equal(it.Schema, schema) {
			t.Fatalf("got schema %v, want %v", it.Schema, schema)
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

// Round times to the microsecond for comparison purposes.
var roundToMicros = cmp.Transformer("RoundToMicros",
	func(t time.Time) time.Time { return t.Round(time.Microsecond) })

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

	d := civil.Date{Year: 2016, Month: 3, Day: 20}
	tm := civil.Time{Hour: 15, Minute: 4, Second: 5, Nanosecond: 6000}
	ts := time.Date(2016, 3, 20, 15, 4, 5, 6000, time.UTC)
	dtm := civil.DateTime{Date: d, Time: tm}
	d2 := civil.Date{Year: 1994, Month: 5, Day: 15}
	tm2 := civil.Time{Hour: 1, Minute: 2, Second: 4, Nanosecond: 0}
	ts2 := time.Date(1994, 5, 15, 1, 2, 4, 0, time.UTC)
	dtm2 := civil.DateTime{Date: d2, Time: tm2}

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
		} else if diff := testutil.Diff(g, want[i], roundToMicros); diff != "" {
			t.Errorf("%d: got=-, want=+:\n%s", i, diff)
		}
	}
}

type byName []*TestStruct

func (b byName) Len() int           { return len(b) }
func (b byName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byName) Less(i, j int) bool { return b[i].Name < b[j].Name }

func TestIntegration_UploadAndReadNullable(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctm := civil.Time{Hour: 15, Minute: 4, Second: 5, Nanosecond: 6000}
	cdt := civil.DateTime{Date: testDate, Time: ctm}
	testUploadAndReadNullable(t, testStructNullable{}, make([]Value, len(testStructNullableSchema)))
	testUploadAndReadNullable(t, testStructNullable{
		String:    NullString{"x", true},
		Bytes:     []byte{1, 2, 3},
		Integer:   NullInt64{1, true},
		Float:     NullFloat64{2.3, true},
		Boolean:   NullBool{true, true},
		Timestamp: NullTimestamp{testTimestamp, true},
		Date:      NullDate{testDate, true},
		Time:      NullTime{ctm, true},
		DateTime:  NullDateTime{cdt, true},
		Record:    &subNullable{X: NullInt64{4, true}},
	},
		[]Value{"x", []byte{1, 2, 3}, int64(1), 2.3, true, testTimestamp, testDate, ctm, cdt, []Value{int64(4)}})
}

func testUploadAndReadNullable(t *testing.T, ts testStructNullable, wantRow []Value) {
	ctx := context.Background()
	table := newTable(t, testStructNullableSchema)
	defer table.Delete(ctx)

	// Populate the table.
	upl := table.Uploader()
	if err := upl.Put(ctx, []*StructSaver{{Schema: testStructNullableSchema, Struct: ts}}); err != nil {
		t.Fatal(putError(err))
	}
	// Wait until the data has been uploaded. This can take a few seconds, according
	// to https://cloud.google.com/bigquery/streaming-data-into-bigquery.
	if err := waitForRow(ctx, table); err != nil {
		t.Fatal(err)
	}

	// Read into a []Value.
	iter := table.Read(ctx)
	gotRows, _, _, err := readAll(iter)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRows) != 1 {
		t.Fatalf("got %d rows, want 1", len(gotRows))
	}
	if diff := testutil.Diff(gotRows[0], wantRow, roundToMicros); diff != "" {
		t.Error(diff)
	}

	// Read into a struct.
	want := ts
	var sn testStructNullable
	it := table.Read(ctx)
	if err := it.Next(&sn); err != nil {
		t.Fatal(err)
	}
	if diff := testutil.Diff(sn, want, roundToMicros); diff != "" {
		t.Error(diff)
	}
}

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
		fields Schema
	}{
		{"change from optional to required", Schema{
			{Name: "name", Type: StringFieldType, Required: true},
			schema3[1],
			schema3[2],
			schema3[3],
		}},
		{"add a required field", Schema{
			schema3[0], schema3[1], schema3[2], schema3[3],
			{Name: "req", Type: StringFieldType, Required: true},
		}},
		{"remove a field", Schema{schema3[0], schema3[1], schema3[2]}},
		{"remove a nested field", Schema{
			schema3[0], schema3[1], schema3[2],
			{Name: "rec2", Type: RecordFieldType, Schema: Schema{nested[0]}}}},
		{"remove all nested fields", Schema{
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
	loader.Labels = map[string]string{"test": "go"}
	job, err := loader.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job.LastStatus() == nil {
		t.Error("no LastStatus")
	}
	conf, err := job.Config()
	if err != nil {
		t.Fatal(err)
	}
	config, ok := conf.(*LoadConfig)
	if !ok {
		t.Fatalf("got %T, want LoadConfig", conf)
	}
	diff := testutil.Diff(config, &loader.LoadConfig,
		cmp.AllowUnexported(Table{}),
		cmpopts.IgnoreUnexported(Client{}, ReaderSource{}),
		// returned schema is at top level, not in the config
		cmpopts.IgnoreFields(FileConfig{}, "Schema"))
	if diff != "" {
		t.Errorf("got=-, want=+:\n%s", diff)
	}
	if err := wait(ctx, job); err != nil {
		t.Fatal(err)
	}
	checkReadAndTotalRows(t, "reader load", table.Read(ctx), wantRows)

}

func TestIntegration_DML(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	table := newTable(t, schema)
	defer table.Delete(ctx)

	sql := fmt.Sprintf(`INSERT %s.%s (name, nums, rec)
						VALUES ('a', [0], STRUCT<BOOL>(TRUE)),
							   ('b', [1], STRUCT<BOOL>(FALSE)),
							   ('c', [2], STRUCT<BOOL>(TRUE))`,
		table.DatasetID, table.TableID)
	if err := dmlInsert(ctx, sql); err != nil {
		t.Fatal(err)
	}
	wantRows := [][]Value{
		[]Value{"a", []Value{int64(0)}, []Value{true}},
		[]Value{"b", []Value{int64(1)}, []Value{false}},
		[]Value{"c", []Value{int64(2)}, []Value{true}},
	}
	checkRead(t, "DML", table.Read(ctx), wantRows)
}

func dmlInsert(ctx context.Context, sql string) error {
	// Retry insert; sometimes it fails with INTERNAL.
	return internal.Retry(ctx, gax.Backoff{}, func() (bool, error) {
		// Use DML to insert.
		q := client.Query(sql)
		job, err := q.Run(ctx)
		if err != nil {
			if e, ok := err.(*googleapi.Error); ok && e.Code < 500 {
				return true, err // fail on 4xx
			}
			return false, err
		}
		if err := wait(ctx, job); err != nil {
			if e, ok := err.(*googleapi.Error); ok && e.Code < 500 {
				return true, err // fail on 4xx
			}
			return false, err
		}
		return true, nil
	})
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

	d := civil.Date{Year: 2016, Month: 3, Day: 20}
	tm := civil.Time{Hour: 12, Minute: 30, Second: 0, Nanosecond: 6000}
	dtm := civil.DateTime{Date: d, Time: tm}
	ts := time.Date(2016, 3, 20, 15, 04, 05, 0, time.UTC)
	wantRows := [][]Value{
		[]Value{d, tm, dtm, ts},
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
	query := fmt.Sprintf("INSERT %s.%s (d, t, dt, ts) "+
		"VALUES ('%s', '%s', '%s', '%s')",
		table.DatasetID, table.TableID,
		d, CivilTimeString(tm), CivilDateTimeString(dtm), ts.Format("2006-01-02 15:04:05"))
	if err := dmlInsert(ctx, query); err != nil {
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

	d := civil.Date{Year: 2016, Month: 3, Day: 20}
	tm := civil.Time{Hour: 15, Minute: 04, Second: 05, Nanosecond: 0}
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
		{fmt.Sprintf("SELECT DATETIME(TIMESTAMP '%s')", dtm), []Value{civil.DateTime{Date: d, Time: tm}}},
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

	d := civil.Date{Year: 2016, Month: 3, Day: 20}
	tm := civil.Time{Hour: 15, Minute: 04, Second: 05, Nanosecond: 3008}
	rtm := tm
	rtm.Nanosecond = 3000 // round to microseconds
	dtm := civil.DateTime{Date: d, Time: tm}
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
		wantConfig interface{}
	}{
		{
			"SELECT @val",
			[]QueryParameter{{"val", 1}},
			[]Value{int64(1)},
			int64(1),
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", 1.3}},
			[]Value{1.3},
			1.3,
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", true}},
			[]Value{true},
			true,
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", "ABC"}},
			[]Value{"ABC"},
			"ABC",
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", []byte("foo")}},
			[]Value{[]byte("foo")},
			[]byte("foo"),
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", ts}},
			[]Value{ts},
			ts,
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", []time.Time{ts, ts}}},
			[]Value{[]Value{ts, ts}},
			[]interface{}{ts, ts},
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", dtm}},
			[]Value{civil.DateTime{Date: d, Time: rtm}},
			civil.DateTime{Date: d, Time: rtm},
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", d}},
			[]Value{d},
			d,
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", tm}},
			[]Value{rtm},
			rtm,
		},
		{
			"SELECT @val",
			[]QueryParameter{{"val", s{ts, []string{"a", "b"}, ss{"c"}, []ss{{"d"}, {"e"}}}}},
			[]Value{[]Value{ts, []Value{"a", "b"}, []Value{"c"}, []Value{[]Value{"d"}, []Value{"e"}}}},
			map[string]interface{}{
				"Timestamp":   ts,
				"StringArray": []interface{}{"a", "b"},
				"SubStruct":   map[string]interface{}{"String": "c"},
				"SubStructArray": []interface{}{
					map[string]interface{}{"String": "d"},
					map[string]interface{}{"String": "e"},
				},
			},
		},
		{
			"SELECT @val.Timestamp, @val.SubStruct.String",
			[]QueryParameter{{"val", s{Timestamp: ts, SubStruct: ss{"a"}}}},
			[]Value{ts, "a"},
			map[string]interface{}{
				"Timestamp":      ts,
				"SubStruct":      map[string]interface{}{"String": "a"},
				"StringArray":    nil,
				"SubStructArray": nil,
			},
		},
	}
	for _, c := range testCases {
		q := client.Query(c.query)
		q.Parameters = c.parameters
		job, err := q.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if job.LastStatus() == nil {
			t.Error("no LastStatus")
		}
		it, err := job.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkRead(t, "QueryParameters", it, [][]Value{c.wantRow})
		config, err := job.Config()
		if err != nil {
			t.Fatal(err)
		}
		got := config.(*QueryConfig).Parameters[0].Value
		if !testutil.Equal(got, c.wantConfig) {
			t.Errorf("param %[1]v (%[1]T): config:\ngot %[2]v (%[2]T)\nwant %[3]v (%[3]T)",
				c.parameters[0].Value, got, c.wantConfig)
		}
	}
}

func TestIntegration_QueryDryRun(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	q := client.Query("SELECT word from " + stdName + " LIMIT 10")
	q.DryRun = true
	job, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	s := job.LastStatus()
	if s.State != Done {
		t.Errorf("state is %v, expected Done", s.State)
	}
	if s.Statistics == nil {
		t.Fatal("no statistics")
	}
	if s.Statistics.Details.(*QueryStatistics).Schema == nil {
		t.Fatal("no schema")
	}
}

func TestIntegration_ExtractExternal(t *testing.T) {
	// Create a table, extract it to GCS, then query it externally.
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	ctx := context.Background()
	schema := Schema{
		{Name: "name", Type: StringFieldType},
		{Name: "num", Type: IntegerFieldType},
	}
	table := newTable(t, schema)
	defer table.Delete(ctx)

	// Insert table data.
	sql := fmt.Sprintf(`INSERT %s.%s (name, num)
		                VALUES ('a', 1), ('b', 2), ('c', 3)`,
		table.DatasetID, table.TableID)
	if err := dmlInsert(ctx, sql); err != nil {
		t.Fatal(err)
	}
	// Extract to a GCS object as CSV.
	bucketName := testutil.ProjID()
	objectName := fmt.Sprintf("bq-test-%s.csv", table.TableID)
	uri := fmt.Sprintf("gs://%s/%s", bucketName, objectName)
	defer storageClient.Bucket(bucketName).Object(objectName).Delete(ctx)
	gr := NewGCSReference(uri)
	gr.DestinationFormat = CSV
	e := table.ExtractorTo(gr)
	job, err := e.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conf, err := job.Config()
	if err != nil {
		t.Fatal(err)
	}
	config, ok := conf.(*ExtractConfig)
	if !ok {
		t.Fatalf("got %T, want ExtractConfig", conf)
	}
	diff := testutil.Diff(config, &e.ExtractConfig,
		cmp.AllowUnexported(Table{}),
		cmpopts.IgnoreUnexported(Client{}))
	if diff != "" {
		t.Errorf("got=-, want=+:\n%s", diff)
	}
	if err := wait(ctx, job); err != nil {
		t.Fatal(err)
	}

	edc := &ExternalDataConfig{
		SourceFormat: CSV,
		SourceURIs:   []string{uri},
		Schema:       schema,
		Options:      &CSVOptions{SkipLeadingRows: 1},
	}
	// Query that CSV file directly.
	q := client.Query("SELECT * FROM csv")
	q.TableDefinitions = map[string]ExternalData{"csv": edc}
	wantRows := [][]Value{
		[]Value{"a", int64(1)},
		[]Value{"b", int64(2)},
		[]Value{"c", int64(3)},
	}
	iter, err := q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkReadAndTotalRows(t, "external query", iter, wantRows)

	// Make a table pointing to the file, and query it.
	// BigQuery does not allow a Table.Read on an external table.
	table = dataset.Table(tableIDs.New())
	err = table.Create(context.Background(), &TableMetadata{
		Schema:             schema,
		ExpirationTime:     testTableExpiration,
		ExternalDataConfig: edc,
	})
	if err != nil {
		t.Fatal(err)
	}
	q = client.Query(fmt.Sprintf("SELECT * FROM %s.%s", table.DatasetID, table.TableID))
	iter, err = q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkReadAndTotalRows(t, "external table", iter, wantRows)

	// While we're here, check that the table metadata is correct.
	md, err := table.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// One difference: since BigQuery returns the schema as part of the ordinary
	// table metadata, it does not populate ExternalDataConfig.Schema.
	md.ExternalDataConfig.Schema = md.Schema
	if diff := testutil.Diff(md.ExternalDataConfig, edc); diff != "" {
		t.Errorf("got=-, want=+\n%s", diff)
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
	var jobs []*Job
	it := client.Jobs(ctx)
	for {
		job, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, job)
		if len(jobs) >= max {
			break
		}
	}
	// We expect that there is at least one job in the last few months.
	if len(jobs) == 0 {
		t.Fatal("did not get any jobs")
	}
}

const tokyo = "asia-northeast1"

func TestIntegration_Location(t *testing.T) {
	if client == nil {
		t.Skip("Integration tests skipped")
	}
	client.Location = ""
	testLocation(t, tokyo)
	client.Location = tokyo
	defer func() {
		client.Location = ""
	}()
	testLocation(t, "")
}

func testLocation(t *testing.T, loc string) {
	ctx := context.Background()
	tokyoDataset := client.Dataset("tokyo")
	err := tokyoDataset.Create(ctx, &DatasetMetadata{Location: loc})
	if err != nil && !hasStatusCode(err, 409) { // 409 = already exists
		t.Fatal(err)
	}
	md, err := tokyoDataset.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if md.Location != tokyo {
		t.Fatalf("dataset location: got %s, want %s", md.Location, tokyo)
	}
	table := tokyoDataset.Table(tableIDs.New())
	err = table.Create(context.Background(), &TableMetadata{
		Schema: Schema{
			{Name: "name", Type: StringFieldType},
			{Name: "nums", Type: IntegerFieldType},
		},
		ExpirationTime: testTableExpiration,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Delete(ctx)
	loader := table.LoaderFrom(NewReaderSource(strings.NewReader("a,0\nb,1\nc,2\n")))
	loader.Location = loc
	job, err := loader.Run(ctx)
	if err != nil {
		t.Fatal("loader.Run", err)
	}
	if job.Location() != tokyo {
		t.Fatalf("job location: got %s, want %s", job.Location(), tokyo)
	}
	_, err = client.JobFromID(ctx, job.ID())
	if client.Location == "" && err == nil {
		t.Error("JobFromID with Tokyo job, no client location: want error, got nil")
	}
	if client.Location != "" && err != nil {
		t.Errorf("JobFromID with Tokyo job, with client location: want nil, got %v", err)
	}
	_, err = client.JobFromIDLocation(ctx, job.ID(), "US")
	if err == nil {
		t.Error("JobFromIDLocation with US: want error, got nil")
	}
	job2, err := client.JobFromIDLocation(ctx, job.ID(), loc)
	if loc == tokyo && err != nil {
		t.Errorf("loc=tokyo: %v", err)
	}
	if loc == "" && err == nil {
		t.Error("loc empty: got nil, want error")
	}
	if job2 != nil && (job2.ID() != job.ID() || job2.Location() != tokyo) {
		t.Errorf("got id %s loc %s, want id%s loc %s", job2.ID(), job2.Location(), job.ID(), tokyo)
	}
	if err := wait(ctx, job); err != nil {
		t.Fatal(err)
	}
	// Cancel should succeed even if the job is done.
	if err := job.Cancel(ctx); err != nil {
		t.Fatal(err)
	}

	q := client.Query(fmt.Sprintf("SELECT * FROM %s.%s", table.DatasetID, table.TableID))
	q.Location = loc
	iter, err := q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantRows := [][]Value{
		[]Value{"a", int64(0)},
		[]Value{"b", int64(1)},
		[]Value{"c", int64(2)},
	}
	checkRead(t, "location", iter, wantRows)

	table2 := tokyoDataset.Table(tableIDs.New())
	copier := table2.CopierFrom(table)
	copier.Location = loc
	if _, err := copier.Run(ctx); err != nil {
		t.Fatal(err)
	}
	bucketName := testutil.ProjID()
	objectName := fmt.Sprintf("bq-test-%s.csv", table.TableID)
	uri := fmt.Sprintf("gs://%s/%s", bucketName, objectName)
	defer storageClient.Bucket(bucketName).Object(objectName).Delete(ctx)
	gr := NewGCSReference(uri)
	gr.DestinationFormat = CSV
	e := table.ExtractorTo(gr)
	e.Location = loc
	if _, err := e.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

// Creates a new, temporary table with a unique name and the given schema.
func newTable(t *testing.T, s Schema) *Table {
	table := dataset.Table(tableIDs.New())
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
	if msg2, ok := compareRead(it, want, false); !ok {
		t.Errorf("%s: %s", msg, msg2)
	}
}

func checkReadAndTotalRows(t *testing.T, msg string, it *RowIterator, want [][]Value) {
	if msg2, ok := compareRead(it, want, true); !ok {
		t.Errorf("%s: %s", msg, msg2)
	}
}

func compareRead(it *RowIterator, want [][]Value, compareTotalRows bool) (msg string, ok bool) {
	got, _, totalRows, err := readAll(it)
	if err != nil {
		return err.Error(), false
	}
	if len(got) != len(want) {
		return fmt.Sprintf("got %d rows, want %d", len(got), len(want)), false
	}
	if compareTotalRows && len(got) != int(totalRows) {
		return fmt.Sprintf("got %d rows, but totalRows = %d", len(got), totalRows), false
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

func readAll(it *RowIterator) ([][]Value, Schema, uint64, error) {
	var (
		rows      [][]Value
		schema    Schema
		totalRows uint64
	)
	for {
		var vals []Value
		err := it.Next(&vals)
		if err == iterator.Done {
			return rows, schema, totalRows, nil
		}
		if err != nil {
			return nil, nil, 0, err
		}
		rows = append(rows, vals)
		schema = it.Schema
		totalRows = it.TotalRows
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
		return err
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
