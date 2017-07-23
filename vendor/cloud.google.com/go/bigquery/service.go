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
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/internal"
	"cloud.google.com/go/internal/version"
	gax "github.com/googleapis/gax-go"

	"golang.org/x/net/context"
	bq "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/googleapi"
)

// service provides an internal abstraction to isolate the generated
// BigQuery API; most of this package uses this interface instead.
// The single implementation, *bigqueryService, contains all the knowledge
// of the generated BigQuery API.
type service interface {
	// Jobs
	insertJob(ctx context.Context, projectId string, conf *insertJobConf) (*Job, error)
	getJob(ctx context.Context, projectId, jobID string) (*Job, error)
	jobCancel(ctx context.Context, projectId, jobID string) error
	jobStatus(ctx context.Context, projectId, jobID string) (*JobStatus, error)

	// Tables
	createTable(ctx context.Context, conf *createTableConf) error
	getTableMetadata(ctx context.Context, projectID, datasetID, tableID string) (*TableMetadata, error)
	deleteTable(ctx context.Context, projectID, datasetID, tableID string) error

	// listTables returns a page of Tables and a next page token. Note: the Tables do not have their c field populated.
	listTables(ctx context.Context, projectID, datasetID string, pageSize int, pageToken string) ([]*Table, string, error)
	patchTable(ctx context.Context, projectID, datasetID, tableID string, conf *patchTableConf) (*TableMetadata, error)

	// Table data
	readTabledata(ctx context.Context, conf *readTableConf, pageToken string) (*readDataResult, error)
	insertRows(ctx context.Context, projectID, datasetID, tableID string, rows []*insertionRow, conf *insertRowsConf) error

	// Datasets
	insertDataset(ctx context.Context, datasetID, projectID string) error
	deleteDataset(ctx context.Context, datasetID, projectID string) error
	getDatasetMetadata(ctx context.Context, projectID, datasetID string) (*DatasetMetadata, error)

	// Misc

	// Waits for a query to complete.
	waitForQuery(ctx context.Context, projectID, jobID string) (Schema, error)

	// listDatasets returns a page of Datasets and a next page token. Note: the Datasets do not have their c field populated.
	listDatasets(ctx context.Context, projectID string, maxResults int, pageToken string, all bool, filter string) ([]*Dataset, string, error)
}

var xGoogHeader = fmt.Sprintf("gl-go/%s gccl/%s", version.Go(), version.Repo)

func setClientHeader(headers http.Header) {
	headers.Set("x-goog-api-client", xGoogHeader)
}

type bigqueryService struct {
	s *bq.Service
}

func newBigqueryService(client *http.Client, endpoint string) (*bigqueryService, error) {
	s, err := bq.New(client)
	if err != nil {
		return nil, fmt.Errorf("constructing bigquery client: %v", err)
	}
	s.BasePath = endpoint

	return &bigqueryService{s: s}, nil
}

// getPages calls the supplied getPage function repeatedly until there are no pages left to get.
// token is the token of the initial page to start from.  Use an empty string to start from the beginning.
func getPages(token string, getPage func(token string) (nextToken string, err error)) error {
	for {
		var err error
		token, err = getPage(token)
		if err != nil {
			return err
		}
		if token == "" {
			return nil
		}
	}
}

type insertJobConf struct {
	job   *bq.Job
	media io.Reader
}

// Calls the Jobs.Insert RPC and returns a Job. Callers must set the returned Job's
// client.
func (s *bigqueryService) insertJob(ctx context.Context, projectID string, conf *insertJobConf) (*Job, error) {
	call := s.s.Jobs.Insert(projectID, conf.job).Context(ctx)
	setClientHeader(call.Header())
	if conf.media != nil {
		call.Media(conf.media)
	}
	var res *bq.Job
	var err error
	invoke := func() error {
		res, err = call.Do()
		return err
	}
	// A job with a client-generated ID can be retried; the presence of the
	// ID makes the insert operation idempotent.
	// We don't retry if there is media, because it is an io.Reader. We'd
	// have to read the contents and keep it in memory, and that could be expensive.
	// TODO(jba): Look into retrying if media != nil.
	if conf.job.JobReference != nil && conf.media == nil {
		err = runWithRetry(ctx, invoke)
	} else {
		err = invoke()
	}
	if err != nil {
		return nil, err
	}

	var dt *bq.TableReference
	if qc := res.Configuration.Query; qc != nil {
		dt = qc.DestinationTable
	}
	return &Job{
		projectID:        projectID,
		jobID:            res.JobReference.JobId,
		destinationTable: dt,
	}, nil
}

type pagingConf struct {
	recordsPerRequest    int64
	setRecordsPerRequest bool

	startIndex uint64
}

type readTableConf struct {
	projectID, datasetID, tableID string
	paging                        pagingConf
	schema                        Schema // lazily initialized when the first page of data is fetched.
}

func (conf *readTableConf) fetch(ctx context.Context, s service, token string) (*readDataResult, error) {
	return s.readTabledata(ctx, conf, token)
}

func (conf *readTableConf) setPaging(pc *pagingConf) { conf.paging = *pc }

type readDataResult struct {
	pageToken string
	rows      [][]Value
	totalRows uint64
	schema    Schema
}

func (s *bigqueryService) readTabledata(ctx context.Context, conf *readTableConf, pageToken string) (*readDataResult, error) {
	// Prepare request to fetch one page of table data.
	req := s.s.Tabledata.List(conf.projectID, conf.datasetID, conf.tableID)
	setClientHeader(req.Header())

	if pageToken != "" {
		req.PageToken(pageToken)
	} else {
		req.StartIndex(conf.paging.startIndex)
	}

	if conf.paging.setRecordsPerRequest {
		req.MaxResults(conf.paging.recordsPerRequest)
	}

	// Fetch the table schema in the background, if necessary.
	var schemaErr error
	var schemaFetch sync.WaitGroup
	if conf.schema == nil {
		schemaFetch.Add(1)
		go func() {
			defer schemaFetch.Done()
			var t *bq.Table
			t, schemaErr = s.s.Tables.Get(conf.projectID, conf.datasetID, conf.tableID).
				Fields("schema").
				Context(ctx).
				Do()
			if schemaErr == nil && t.Schema != nil {
				conf.schema = convertTableSchema(t.Schema)
			}
		}()
	}

	res, err := req.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	schemaFetch.Wait()
	if schemaErr != nil {
		return nil, schemaErr
	}

	result := &readDataResult{
		pageToken: res.PageToken,
		totalRows: uint64(res.TotalRows),
		schema:    conf.schema,
	}
	result.rows, err = convertRows(res.Rows, conf.schema)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *bigqueryService) waitForQuery(ctx context.Context, projectID, jobID string) (Schema, error) {
	// Use GetQueryResults only to wait for completion, not to read results.
	req := s.s.Jobs.GetQueryResults(projectID, jobID).Context(ctx).MaxResults(0)
	setClientHeader(req.Header())
	backoff := gax.Backoff{
		Initial:    1 * time.Second,
		Multiplier: 2,
		Max:        60 * time.Second,
	}
	var res *bq.GetQueryResultsResponse
	err := internal.Retry(ctx, backoff, func() (stop bool, err error) {
		res, err = req.Do()
		if err != nil {
			return !retryableError(err), err
		}
		if !res.JobComplete { // GetQueryResults may return early without error; retry.
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return convertTableSchema(res.Schema), nil
}

type insertRowsConf struct {
	templateSuffix      string
	ignoreUnknownValues bool
	skipInvalidRows     bool
}

func (s *bigqueryService) insertRows(ctx context.Context, projectID, datasetID, tableID string, rows []*insertionRow, conf *insertRowsConf) error {
	req := &bq.TableDataInsertAllRequest{
		TemplateSuffix:      conf.templateSuffix,
		IgnoreUnknownValues: conf.ignoreUnknownValues,
		SkipInvalidRows:     conf.skipInvalidRows,
	}
	for _, row := range rows {
		m := make(map[string]bq.JsonValue)
		for k, v := range row.Row {
			m[k] = bq.JsonValue(v)
		}
		req.Rows = append(req.Rows, &bq.TableDataInsertAllRequestRows{
			InsertId: row.InsertID,
			Json:     m,
		})
	}
	var res *bq.TableDataInsertAllResponse
	err := runWithRetry(ctx, func() error {
		var err error
		req := s.s.Tabledata.InsertAll(projectID, datasetID, tableID, req).Context(ctx)
		setClientHeader(req.Header())
		res, err = req.Do()
		return err
	})
	if err != nil {
		return err
	}
	if len(res.InsertErrors) == 0 {
		return nil
	}

	var errs PutMultiError
	for _, e := range res.InsertErrors {
		if int(e.Index) > len(rows) {
			return fmt.Errorf("internal error: unexpected row index: %v", e.Index)
		}
		rie := RowInsertionError{
			InsertID: rows[e.Index].InsertID,
			RowIndex: int(e.Index),
		}
		for _, errp := range e.Errors {
			rie.Errors = append(rie.Errors, errorFromErrorProto(errp))
		}
		errs = append(errs, rie)
	}
	return errs
}

func (s *bigqueryService) getJob(ctx context.Context, projectID, jobID string) (*Job, error) {
	res, err := s.s.Jobs.Get(projectID, jobID).
		Fields("configuration").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	var isQuery bool
	var dest *bq.TableReference
	if res.Configuration.Query != nil {
		isQuery = true
		dest = res.Configuration.Query.DestinationTable
	}
	return &Job{
		projectID:        projectID,
		jobID:            jobID,
		isQuery:          isQuery,
		destinationTable: dest,
	}, nil
}

func (s *bigqueryService) jobCancel(ctx context.Context, projectID, jobID string) error {
	// Jobs.Cancel returns a job entity, but the only relevant piece of
	// data it may contain (the status of the job) is unreliable.  From the
	// docs: "This call will return immediately, and the client will need
	// to poll for the job status to see if the cancel completed
	// successfully".  So it would be misleading to return a status.
	_, err := s.s.Jobs.Cancel(projectID, jobID).
		Fields(). // We don't need any of the response data.
		Context(ctx).
		Do()
	return err
}

func (s *bigqueryService) jobStatus(ctx context.Context, projectID, jobID string) (*JobStatus, error) {
	res, err := s.s.Jobs.Get(projectID, jobID).
		Fields("status", "statistics"). // Only fetch what we need.
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	st, err := jobStatusFromProto(res.Status)
	if err != nil {
		return nil, err
	}
	st.Statistics = jobStatisticsFromProto(res.Statistics)
	return st, nil
}

var stateMap = map[string]State{"PENDING": Pending, "RUNNING": Running, "DONE": Done}

func jobStatusFromProto(status *bq.JobStatus) (*JobStatus, error) {
	state, ok := stateMap[status.State]
	if !ok {
		return nil, fmt.Errorf("unexpected job state: %v", status.State)
	}

	newStatus := &JobStatus{
		State: state,
		err:   nil,
	}
	if err := errorFromErrorProto(status.ErrorResult); state == Done && err != nil {
		newStatus.err = err
	}

	for _, ep := range status.Errors {
		newStatus.Errors = append(newStatus.Errors, errorFromErrorProto(ep))
	}
	return newStatus, nil
}

func jobStatisticsFromProto(s *bq.JobStatistics) *JobStatistics {
	js := &JobStatistics{
		CreationTime:        unixMillisToTime(s.CreationTime),
		StartTime:           unixMillisToTime(s.StartTime),
		EndTime:             unixMillisToTime(s.EndTime),
		TotalBytesProcessed: s.TotalBytesProcessed,
	}
	switch {
	case s.Extract != nil:
		js.Details = &ExtractStatistics{
			DestinationURIFileCounts: []int64(s.Extract.DestinationUriFileCounts),
		}
	case s.Load != nil:
		js.Details = &LoadStatistics{
			InputFileBytes: s.Load.InputFileBytes,
			InputFiles:     s.Load.InputFiles,
			OutputBytes:    s.Load.OutputBytes,
			OutputRows:     s.Load.OutputRows,
		}
	case s.Query != nil:
		var names []string
		for _, qp := range s.Query.UndeclaredQueryParameters {
			names = append(names, qp.Name)
		}
		var tables []*Table
		for _, tr := range s.Query.ReferencedTables {
			tables = append(tables, convertTableReference(tr))
		}
		js.Details = &QueryStatistics{
			BillingTier:                   s.Query.BillingTier,
			CacheHit:                      s.Query.CacheHit,
			StatementType:                 s.Query.StatementType,
			TotalBytesBilled:              s.Query.TotalBytesBilled,
			TotalBytesProcessed:           s.Query.TotalBytesProcessed,
			NumDMLAffectedRows:            s.Query.NumDmlAffectedRows,
			QueryPlan:                     queryPlanFromProto(s.Query.QueryPlan),
			Schema:                        convertTableSchema(s.Query.Schema),
			ReferencedTables:              tables,
			UndeclaredQueryParameterNames: names,
		}
	}
	return js
}

func queryPlanFromProto(stages []*bq.ExplainQueryStage) []*ExplainQueryStage {
	var res []*ExplainQueryStage
	for _, s := range stages {
		var steps []*ExplainQueryStep
		for _, p := range s.Steps {
			steps = append(steps, &ExplainQueryStep{
				Kind:     p.Kind,
				Substeps: p.Substeps,
			})
		}
		res = append(res, &ExplainQueryStage{
			ComputeRatioAvg: s.ComputeRatioAvg,
			ComputeRatioMax: s.ComputeRatioMax,
			ID:              s.Id,
			Name:            s.Name,
			ReadRatioAvg:    s.ReadRatioAvg,
			ReadRatioMax:    s.ReadRatioMax,
			RecordsRead:     s.RecordsRead,
			RecordsWritten:  s.RecordsWritten,
			Status:          s.Status,
			Steps:           steps,
			WaitRatioAvg:    s.WaitRatioAvg,
			WaitRatioMax:    s.WaitRatioMax,
			WriteRatioAvg:   s.WriteRatioAvg,
			WriteRatioMax:   s.WriteRatioMax,
		})
	}
	return res
}

// listTables returns a subset of tables that belong to a dataset, and a token for fetching the next subset.
func (s *bigqueryService) listTables(ctx context.Context, projectID, datasetID string, pageSize int, pageToken string) ([]*Table, string, error) {
	var tables []*Table
	req := s.s.Tables.List(projectID, datasetID).
		PageToken(pageToken).
		Context(ctx)
	setClientHeader(req.Header())
	if pageSize > 0 {
		req.MaxResults(int64(pageSize))
	}
	res, err := req.Do()
	if err != nil {
		return nil, "", err
	}
	for _, t := range res.Tables {
		tables = append(tables, convertTableReference(t.TableReference))
	}
	return tables, res.NextPageToken, nil
}

type createTableConf struct {
	projectID, datasetID, tableID string
	expiration                    time.Time
	viewQuery                     string
	schema                        *bq.TableSchema
	useStandardSQL                bool
	timePartitioning              *TimePartitioning
}

// createTable creates a table in the BigQuery service.
// expiration is an optional time after which the table will be deleted and its storage reclaimed.
// If viewQuery is non-empty, the created table will be of type VIEW.
// Note: expiration can only be set during table creation.
// Note: after table creation, a view can be modified only if its table was initially created with a view.
func (s *bigqueryService) createTable(ctx context.Context, conf *createTableConf) error {
	table := &bq.Table{
		TableReference: &bq.TableReference{
			ProjectId: conf.projectID,
			DatasetId: conf.datasetID,
			TableId:   conf.tableID,
		},
	}
	if !conf.expiration.IsZero() {
		table.ExpirationTime = conf.expiration.UnixNano() / 1e6
	}
	// TODO(jba): make it impossible to provide both a view query and a schema.
	if conf.viewQuery != "" {
		table.View = &bq.ViewDefinition{
			Query: conf.viewQuery,
		}
		if conf.useStandardSQL {
			table.View.UseLegacySql = false
			table.View.ForceSendFields = append(table.View.ForceSendFields, "UseLegacySql")
		}
	}
	if conf.schema != nil {
		table.Schema = conf.schema
	}
	if conf.timePartitioning != nil {
		table.TimePartitioning = &bq.TimePartitioning{
			Type:         "DAY",
			ExpirationMs: int64(conf.timePartitioning.Expiration.Seconds() * 1000),
		}
	}

	req := s.s.Tables.Insert(conf.projectID, conf.datasetID, table).Context(ctx)
	setClientHeader(req.Header())
	_, err := req.Do()
	return err
}

func (s *bigqueryService) getTableMetadata(ctx context.Context, projectID, datasetID, tableID string) (*TableMetadata, error) {
	req := s.s.Tables.Get(projectID, datasetID, tableID).Context(ctx)
	setClientHeader(req.Header())
	table, err := req.Do()
	if err != nil {
		return nil, err
	}
	return bqTableToMetadata(table), nil
}

func (s *bigqueryService) deleteTable(ctx context.Context, projectID, datasetID, tableID string) error {
	req := s.s.Tables.Delete(projectID, datasetID, tableID).Context(ctx)
	setClientHeader(req.Header())
	return req.Do()
}

func bqTableToMetadata(t *bq.Table) *TableMetadata {
	md := &TableMetadata{
		Description:      t.Description,
		Name:             t.FriendlyName,
		Type:             TableType(t.Type),
		ID:               t.Id,
		NumBytes:         t.NumBytes,
		NumRows:          t.NumRows,
		ExpirationTime:   unixMillisToTime(t.ExpirationTime),
		CreationTime:     unixMillisToTime(t.CreationTime),
		LastModifiedTime: unixMillisToTime(int64(t.LastModifiedTime)),
	}
	if t.Schema != nil {
		md.Schema = convertTableSchema(t.Schema)
	}
	if t.View != nil {
		md.View = t.View.Query
	}
	if t.TimePartitioning != nil {
		md.TimePartitioning = &TimePartitioning{
			Expiration: time.Duration(t.TimePartitioning.ExpirationMs) * time.Millisecond,
		}
	}
	if t.StreamingBuffer != nil {
		md.StreamingBuffer = &StreamingBuffer{
			EstimatedBytes:  t.StreamingBuffer.EstimatedBytes,
			EstimatedRows:   t.StreamingBuffer.EstimatedRows,
			OldestEntryTime: unixMillisToTime(int64(t.StreamingBuffer.OldestEntryTime)),
		}
	}
	return md
}

func bqDatasetToMetadata(d *bq.Dataset) *DatasetMetadata {
	/// TODO(jba): access
	return &DatasetMetadata{
		CreationTime:           unixMillisToTime(d.CreationTime),
		LastModifiedTime:       unixMillisToTime(d.LastModifiedTime),
		DefaultTableExpiration: time.Duration(d.DefaultTableExpirationMs) * time.Millisecond,
		Description:            d.Description,
		Name:                   d.FriendlyName,
		ID:                     d.Id,
		Location:               d.Location,
		Labels:                 d.Labels,
	}
}

// Convert a number of milliseconds since the Unix epoch to a time.Time.
// Treat an input of zero specially: convert it to the zero time,
// rather than the start of the epoch.
func unixMillisToTime(m int64) time.Time {
	if m == 0 {
		return time.Time{}
	}
	return time.Unix(0, m*1e6)
}

func convertTableReference(tr *bq.TableReference) *Table {
	return &Table{
		ProjectID: tr.ProjectId,
		DatasetID: tr.DatasetId,
		TableID:   tr.TableId,
	}
}

// patchTableConf contains fields to be patched.
type patchTableConf struct {
	// These fields are omitted from the patch operation if nil.
	Description *string
	Name        *string
	Schema      Schema
}

func (s *bigqueryService) patchTable(ctx context.Context, projectID, datasetID, tableID string, conf *patchTableConf) (*TableMetadata, error) {
	t := &bq.Table{}
	forceSend := func(field string) {
		t.ForceSendFields = append(t.ForceSendFields, field)
	}

	if conf.Description != nil {
		t.Description = *conf.Description
		forceSend("Description")
	}
	if conf.Name != nil {
		t.FriendlyName = *conf.Name
		forceSend("FriendlyName")
	}
	if conf.Schema != nil {
		t.Schema = conf.Schema.asTableSchema()
		forceSend("Schema")
	}
	table, err := s.s.Tables.Patch(projectID, datasetID, tableID, t).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	return bqTableToMetadata(table), nil
}

func (s *bigqueryService) insertDataset(ctx context.Context, datasetID, projectID string) error {
	ds := &bq.Dataset{
		DatasetReference: &bq.DatasetReference{DatasetId: datasetID},
	}
	req := s.s.Datasets.Insert(projectID, ds).Context(ctx)
	setClientHeader(req.Header())
	_, err := req.Do()
	return err
}

func (s *bigqueryService) deleteDataset(ctx context.Context, datasetID, projectID string) error {
	req := s.s.Datasets.Delete(projectID, datasetID).Context(ctx)
	setClientHeader(req.Header())
	return req.Do()
}

func (s *bigqueryService) getDatasetMetadata(ctx context.Context, projectID, datasetID string) (*DatasetMetadata, error) {
	req := s.s.Datasets.Get(projectID, datasetID).Context(ctx)
	setClientHeader(req.Header())
	table, err := req.Do()
	if err != nil {
		return nil, err
	}
	return bqDatasetToMetadata(table), nil
}

func (s *bigqueryService) listDatasets(ctx context.Context, projectID string, maxResults int, pageToken string, all bool, filter string) ([]*Dataset, string, error) {
	req := s.s.Datasets.List(projectID).
		Context(ctx).
		PageToken(pageToken).
		All(all)
	setClientHeader(req.Header())
	if maxResults > 0 {
		req.MaxResults(int64(maxResults))
	}
	if filter != "" {
		req.Filter(filter)
	}
	res, err := req.Do()
	if err != nil {
		return nil, "", err
	}
	var datasets []*Dataset
	for _, d := range res.Datasets {
		datasets = append(datasets, s.convertListedDataset(d))
	}
	return datasets, res.NextPageToken, nil
}

func (s *bigqueryService) convertListedDataset(d *bq.DatasetListDatasets) *Dataset {
	return &Dataset{
		ProjectID: d.DatasetReference.ProjectId,
		DatasetID: d.DatasetReference.DatasetId,
	}
}

// runWithRetry calls the function until it returns nil or a non-retryable error, or
// the context is done.
// See the similar function in ../storage/invoke.go. The main difference is the
// reason for retrying.
func runWithRetry(ctx context.Context, call func() error) error {
	backoff := gax.Backoff{
		Initial:    2 * time.Second,
		Max:        32 * time.Second,
		Multiplier: 2,
	}
	return internal.Retry(ctx, backoff, func() (stop bool, err error) {
		err = call()
		if err == nil {
			return true, nil
		}
		return !retryableError(err), err
	})
}

// Use the criteria in https://cloud.google.com/bigquery/troubleshooting-errors.
func retryableError(err error) bool {
	e, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	var reason string
	if len(e.Errors) > 0 {
		reason = e.Errors[0].Reason
	}
	return reason == "backendError" && (e.Code == 500 || e.Code == 503)
}
