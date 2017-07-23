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
	"time"

	"cloud.google.com/go/internal"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	bq "google.golang.org/api/bigquery/v2"
)

// A Job represents an operation which has been submitted to BigQuery for processing.
type Job struct {
	c         *Client
	projectID string
	jobID     string

	isQuery          bool
	destinationTable *bq.TableReference // table to read query results from
}

// JobFromID creates a Job which refers to an existing BigQuery job. The job
// need not have been created by this package. For example, the job may have
// been created in the BigQuery console.
func (c *Client) JobFromID(ctx context.Context, id string) (*Job, error) {
	job, err := c.service.getJob(ctx, c.projectID, id)
	if err != nil {
		return nil, err
	}
	job.c = c
	return job, nil
}

func (j *Job) ID() string {
	return j.jobID
}

// State is one of a sequence of states that a Job progresses through as it is processed.
type State int

const (
	Pending State = iota
	Running
	Done
)

// JobStatus contains the current State of a job, and errors encountered while processing that job.
type JobStatus struct {
	State State

	err error

	// All errors encountered during the running of the job.
	// Not all Errors are fatal, so errors here do not necessarily mean that the job has completed or was unsuccessful.
	Errors []*Error

	// Statistics about the job.
	Statistics *JobStatistics
}

// setJobRef initializes job's JobReference if given a non-empty jobID.
// projectID must be non-empty.
func setJobRef(job *bq.Job, jobID, projectID string) {
	if jobID == "" {
		return
	}
	// We don't check whether projectID is empty; the server will return an
	// error when it encounters the resulting JobReference.

	job.JobReference = &bq.JobReference{
		JobId:     jobID,
		ProjectId: projectID,
	}
}

// Done reports whether the job has completed.
// After Done returns true, the Err method will return an error if the job completed unsuccesfully.
func (s *JobStatus) Done() bool {
	return s.State == Done
}

// Err returns the error that caused the job to complete unsuccesfully (if any).
func (s *JobStatus) Err() error {
	return s.err
}

// Status returns the current status of the job. It fails if the Status could not be determined.
func (j *Job) Status(ctx context.Context) (*JobStatus, error) {
	js, err := j.c.service.jobStatus(ctx, j.projectID, j.jobID)
	if err != nil {
		return nil, err
	}
	// Fill in the client field of Tables in the statistics.
	if js.Statistics != nil {
		if qs, ok := js.Statistics.Details.(*QueryStatistics); ok {
			for _, t := range qs.ReferencedTables {
				t.c = j.c
			}
		}
	}
	return js, nil
}

// Cancel requests that a job be cancelled. This method returns without waiting for
// cancellation to take effect. To check whether the job has terminated, use Job.Status.
// Cancelled jobs may still incur costs.
func (j *Job) Cancel(ctx context.Context) error {
	return j.c.service.jobCancel(ctx, j.projectID, j.jobID)
}

// Wait blocks until the job or the context is done. It returns the final status
// of the job.
// If an error occurs while retrieving the status, Wait returns that error. But
// Wait returns nil if the status was retrieved successfully, even if
// status.Err() != nil. So callers must check both errors. See the example.
func (j *Job) Wait(ctx context.Context) (*JobStatus, error) {
	if j.isQuery {
		// We can avoid polling for query jobs.
		if _, err := j.c.service.waitForQuery(ctx, j.projectID, j.jobID); err != nil {
			return nil, err
		}
		// Note: extra RPC even if you just want to wait for the query to finish.
		js, err := j.Status(ctx)
		if err != nil {
			return nil, err
		}
		return js, nil
	}
	// Non-query jobs must poll.
	var js *JobStatus
	err := internal.Retry(ctx, gax.Backoff{}, func() (stop bool, err error) {
		js, err = j.Status(ctx)
		if err != nil {
			return true, err
		}
		if js.Done() {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return js, nil
}

// Read fetches the results of a query job.
// If j is not a query job, Read returns an error.
func (j *Job) Read(ctx context.Context) (*RowIterator, error) {
	if !j.isQuery {
		return nil, errors.New("bigquery: cannot read from a non-query job")
	}
	var projectID string
	if j.destinationTable != nil {
		projectID = j.destinationTable.ProjectId
	} else {
		projectID = j.c.projectID
	}

	schema, err := j.c.service.waitForQuery(ctx, projectID, j.jobID)
	if err != nil {
		return nil, err
	}
	// The destination table should only be nil if there was a query error.
	if j.destinationTable == nil {
		return nil, errors.New("bigquery: query job missing destination table")
	}
	return newRowIterator(ctx, j.c.service, &readTableConf{
		projectID: j.destinationTable.ProjectId,
		datasetID: j.destinationTable.DatasetId,
		tableID:   j.destinationTable.TableId,
		schema:    schema,
	}), nil
}

// JobStatistics contains statistics about a job.
type JobStatistics struct {
	CreationTime        time.Time
	StartTime           time.Time
	EndTime             time.Time
	TotalBytesProcessed int64

	Details Statistics
}

// Statistics is one of ExtractStatistics, LoadStatistics or QueryStatistics.
type Statistics interface {
	implementsStatistics()
}

// ExtractStatistics contains statistics about an extract job.
type ExtractStatistics struct {
	// The number of files per destination URI or URI pattern specified in the
	// extract configuration. These values will be in the same order as the
	// URIs specified in the 'destinationUris' field.
	DestinationURIFileCounts []int64
}

// LoadStatistics contains statistics about a load job.
type LoadStatistics struct {
	// The number of bytes of source data in a load job.
	InputFileBytes int64

	// The number of source files in a load job.
	InputFiles int64

	// Size of the loaded data in bytes. Note that while a load job is in the
	// running state, this value may change.
	OutputBytes int64

	// The number of rows imported in a load job. Note that while an import job is
	// in the running state, this value may change.
	OutputRows int64
}

// QueryStatistics contains statistics about a query job.
type QueryStatistics struct {
	// Billing tier for the job.
	BillingTier int64

	// Whether the query result was fetched from the query cache.
	CacheHit bool

	// The type of query statement, if valid.
	StatementType string

	// Total bytes billed for the job.
	TotalBytesBilled int64

	// Total bytes processed for the job.
	TotalBytesProcessed int64

	// Describes execution plan for the query.
	QueryPlan []*ExplainQueryStage

	// The number of rows affected by a DML statement. Present only for DML
	// statements INSERT, UPDATE or DELETE.
	NumDMLAffectedRows int64

	// ReferencedTables: [Output-only, Experimental] Referenced tables for
	// the job. Queries that reference more than 50 tables will not have a
	// complete list.
	ReferencedTables []*Table

	// The schema of the results. Present only for successful dry run of
	// non-legacy SQL queries.
	Schema Schema

	// Standard SQL: list of undeclared query parameter names detected during a
	// dry run validation.
	UndeclaredQueryParameterNames []string
}

// ExplainQueryStage describes one stage of a query.
type ExplainQueryStage struct {
	// Relative amount of the total time the average shard spent on CPU-bound tasks.
	ComputeRatioAvg float64

	// Relative amount of the total time the slowest shard spent on CPU-bound tasks.
	ComputeRatioMax float64

	// Unique ID for stage within plan.
	ID int64

	// Human-readable name for stage.
	Name string

	// Relative amount of the total time the average shard spent reading input.
	ReadRatioAvg float64

	// Relative amount of the total time the slowest shard spent reading input.
	ReadRatioMax float64

	// Number of records read into the stage.
	RecordsRead int64

	// Number of records written by the stage.
	RecordsWritten int64

	// Current status for the stage.
	Status string

	// List of operations within the stage in dependency order (approximately
	// chronological).
	Steps []*ExplainQueryStep

	// Relative amount of the total time the average shard spent waiting to be scheduled.
	WaitRatioAvg float64

	// Relative amount of the total time the slowest shard spent waiting to be scheduled.
	WaitRatioMax float64

	// Relative amount of the total time the average shard spent on writing output.
	WriteRatioAvg float64

	// Relative amount of the total time the slowest shard spent on writing output.
	WriteRatioMax float64
}

// ExplainQueryStep describes one step of a query stage.
type ExplainQueryStep struct {
	// Machine-readable operation type.
	Kind string

	// Human-readable stage descriptions.
	Substeps []string
}

func (*ExtractStatistics) implementsStatistics() {}
func (*LoadStatistics) implementsStatistics()    {}
func (*QueryStatistics) implementsStatistics()   {}
