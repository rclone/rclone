// Copyright 2017 Google Inc. All Rights Reserved.
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

package firestore

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"

	"golang.org/x/net/context"

	pb "google.golang.org/genproto/googleapis/firestore/v1beta1"

	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/api/iterator"
)

// Query represents a Firestore query.
//
// Query values are immutable. Each Query method creates
// a new Query; it does not modify the old.
type Query struct {
	c                      *Client
	parentPath             string // path of the collection's parent
	collectionID           string
	selection              []FieldPath
	filters                []filter
	orders                 []order
	offset                 int32
	limit                  *wrappers.Int32Value
	startVals, endVals     []interface{}
	startDoc, endDoc       *DocumentSnapshot
	startBefore, endBefore bool
	err                    error
}

func (q *Query) collectionPath() string {
	return q.parentPath + "/documents/" + q.collectionID
}

// DocumentID is the special field name representing the ID of a document
// in queries.
const DocumentID = "__name__"

// Select returns a new Query that specifies the paths
// to return from the result documents.
// Each path argument can be a single field or a dot-separated sequence of
// fields, and must not contain any of the runes "˜*/[]".
func (q Query) Select(paths ...string) Query {
	var fps []FieldPath
	for _, s := range paths {
		fp, err := parseDotSeparatedString(s)
		if err != nil {
			q.err = err
			return q
		}
		fps = append(fps, fp)
	}
	return q.SelectPaths(fps...)
}

// SelectPaths returns a new Query that specifies the field paths
// to return from the result documents.
func (q Query) SelectPaths(fieldPaths ...FieldPath) Query {
	if len(fieldPaths) == 0 {
		q.selection = []FieldPath{{DocumentID}}
	} else {
		q.selection = fieldPaths
	}
	return q
}

// Where returns a new Query that filters the set of results.
// A Query can have multiple filters.
// The path argument can be a single field or a dot-separated sequence of
// fields, and must not contain any of the runes "˜*/[]".
// The op argument must be one of "==", "<", "<=", ">" or ">=".
func (q Query) Where(path, op string, value interface{}) Query {
	fp, err := parseDotSeparatedString(path)
	if err != nil {
		q.err = err
		return q
	}
	q.filters = append(append([]filter(nil), q.filters...), filter{fp, op, value})
	return q
}

// WherePath returns a new Query that filters the set of results.
// A Query can have multiple filters.
// The op argument must be one of "==", "<", "<=", ">" or ">=".
func (q Query) WherePath(fp FieldPath, op string, value interface{}) Query {
	q.filters = append(append([]filter(nil), q.filters...), filter{fp, op, value})
	return q
}

// Direction is the sort direction for result ordering.
type Direction int32

const (
	// Asc sorts results from smallest to largest.
	Asc Direction = Direction(pb.StructuredQuery_ASCENDING)

	// Desc sorts results from largest to smallest.
	Desc Direction = Direction(pb.StructuredQuery_DESCENDING)
)

// OrderBy returns a new Query that specifies the order in which results are
// returned. A Query can have multiple OrderBy/OrderByPath specifications. OrderBy
// appends the specification to the list of existing ones.
//
// The path argument can be a single field or a dot-separated sequence of
// fields, and must not contain any of the runes "˜*/[]".
//
// To order by document name, use the special field path DocumentID.
func (q Query) OrderBy(path string, dir Direction) Query {
	fp, err := parseDotSeparatedString(path)
	if err != nil {
		q.err = err
		return q
	}
	q.orders = append(q.copyOrders(), order{fp, dir})
	return q
}

// OrderByPath returns a new Query that specifies the order in which results are
// returned. A Query can have multiple OrderBy/OrderByPath specifications.
// OrderByPath appends the specification to the list of existing ones.
func (q Query) OrderByPath(fp FieldPath, dir Direction) Query {
	q.orders = append(q.copyOrders(), order{fp, dir})
	return q
}

func (q *Query) copyOrders() []order {
	return append([]order(nil), q.orders...)
}

// Offset returns a new Query that specifies the number of initial results to skip.
// It must not be negative.
func (q Query) Offset(n int) Query {
	q.offset = trunc32(n)
	return q
}

// Limit returns a new Query that specifies the maximum number of results to return.
// It must not be negative.
func (q Query) Limit(n int) Query {
	q.limit = &wrappers.Int32Value{trunc32(n)}
	return q
}

// StartAt returns a new Query that specifies that results should start at
// the document with the given field values.
//
// If StartAt is called with a single DocumentSnapshot, its field values are used.
// The DocumentSnapshot must have all the fields mentioned in the OrderBy clauses.
//
// Otherwise, StartAt should be called with one field value for each OrderBy clause,
// in the order that they appear. For example, in
//   q.OrderBy("X", Asc).OrderBy("Y", Desc).StartAt(1, 2)
// results will begin at the first document where X = 1 and Y = 2.
//
// If an OrderBy call uses the special DocumentID field path, the corresponding value
// should be the document ID relative to the query's collection. For example, to
// start at the document "NewYork" in the "States" collection, write
//
//   client.Collection("States").OrderBy(DocumentID, firestore.Asc).StartAt("NewYork")
//
// Calling StartAt overrides a previous call to StartAt or StartAfter.
func (q Query) StartAt(docSnapshotOrFieldValues ...interface{}) Query {
	q.startBefore = true
	q.startVals, q.startDoc, q.err = q.processCursorArg("StartAt", docSnapshotOrFieldValues)
	return q
}

// StartAfter returns a new Query that specifies that results should start just after
// the document with the given field values. See Query.StartAt for more information.
//
// Calling StartAfter overrides a previous call to StartAt or StartAfter.
func (q Query) StartAfter(docSnapshotOrFieldValues ...interface{}) Query {
	q.startBefore = false
	q.startVals, q.startDoc, q.err = q.processCursorArg("StartAfter", docSnapshotOrFieldValues)
	return q
}

// EndAt returns a new Query that specifies that results should end at the
// document with the given field values. See Query.StartAt for more information.
//
// Calling EndAt overrides a previous call to EndAt or EndBefore.
func (q Query) EndAt(docSnapshotOrFieldValues ...interface{}) Query {
	q.endBefore = false
	q.endVals, q.endDoc, q.err = q.processCursorArg("EndAt", docSnapshotOrFieldValues)
	return q
}

// EndBefore returns a new Query that specifies that results should end just before
// the document with the given field values. See Query.StartAt for more information.
//
// Calling EndBefore overrides a previous call to EndAt or EndBefore.
func (q Query) EndBefore(docSnapshotOrFieldValues ...interface{}) Query {
	q.endBefore = true
	q.endVals, q.endDoc, q.err = q.processCursorArg("EndBefore", docSnapshotOrFieldValues)
	return q
}

func (q *Query) processCursorArg(name string, docSnapshotOrFieldValues []interface{}) ([]interface{}, *DocumentSnapshot, error) {
	for _, e := range docSnapshotOrFieldValues {
		if ds, ok := e.(*DocumentSnapshot); ok {
			if len(docSnapshotOrFieldValues) == 1 {
				return nil, ds, nil
			}
			return nil, nil, fmt.Errorf("firestore: a document snapshot must be the only argument to %s", name)
		}
	}
	return docSnapshotOrFieldValues, nil, nil
}

func (q Query) query() *Query { return &q }

func (q Query) toProto() (*pb.StructuredQuery, error) {
	if q.err != nil {
		return nil, q.err
	}
	if q.collectionID == "" {
		return nil, errors.New("firestore: query created without CollectionRef")
	}
	p := &pb.StructuredQuery{
		From:   []*pb.StructuredQuery_CollectionSelector{{CollectionId: q.collectionID}},
		Offset: q.offset,
		Limit:  q.limit,
	}
	if len(q.selection) > 0 {
		p.Select = &pb.StructuredQuery_Projection{}
		for _, fp := range q.selection {
			if err := fp.validate(); err != nil {
				return nil, err
			}
			p.Select.Fields = append(p.Select.Fields, fref(fp))
		}
	}
	// If there is only filter, use it directly. Otherwise, construct
	// a CompositeFilter.
	if len(q.filters) == 1 {
		pf, err := q.filters[0].toProto()
		if err != nil {
			return nil, err
		}
		p.Where = pf
	} else if len(q.filters) > 1 {
		cf := &pb.StructuredQuery_CompositeFilter{
			Op: pb.StructuredQuery_CompositeFilter_AND,
		}
		p.Where = &pb.StructuredQuery_Filter{
			FilterType: &pb.StructuredQuery_Filter_CompositeFilter{cf},
		}
		for _, f := range q.filters {
			pf, err := f.toProto()
			if err != nil {
				return nil, err
			}
			cf.Filters = append(cf.Filters, pf)
		}
	}
	orders := q.orders
	if q.startDoc != nil || q.endDoc != nil {
		orders = q.adjustOrders()
	}
	for _, ord := range orders {
		po, err := ord.toProto()
		if err != nil {
			return nil, err
		}
		p.OrderBy = append(p.OrderBy, po)
	}

	cursor, err := q.toCursor(q.startVals, q.startDoc, q.startBefore, orders)
	if err != nil {
		return nil, err
	}
	p.StartAt = cursor
	cursor, err = q.toCursor(q.endVals, q.endDoc, q.endBefore, orders)
	if err != nil {
		return nil, err
	}
	p.EndAt = cursor
	return p, nil
}

// If there is a start/end that uses a Document Snapshot, we may need to adjust the OrderBy
// clauses that the user provided: we add OrderBy(__name__) if it isn't already present, and
// we make sure we don't invalidate the original query by adding an OrderBy for inequality filters.
func (q *Query) adjustOrders() []order {
	// If the user is already ordering by document ID, don't change anything.
	for _, ord := range q.orders {
		if ord.isDocumentID() {
			return q.orders
		}
	}
	// If there are OrderBy clauses, append an OrderBy(DocumentID), using the direction of the last OrderBy clause.
	if len(q.orders) > 0 {
		return append(q.copyOrders(), order{
			fieldPath: FieldPath{DocumentID},
			dir:       q.orders[len(q.orders)-1].dir,
		})
	}
	// If there are no OrderBy clauses but there is an inequality, add an OrderBy clause
	// for the field of the first inequality.
	var orders []order
	for _, f := range q.filters {
		if f.op != "==" {
			orders = []order{{fieldPath: f.fieldPath, dir: Asc}}
			break
		}
	}
	// Add an ascending OrderBy(DocumentID).
	return append(orders, order{fieldPath: FieldPath{DocumentID}, dir: Asc})
}

func (q *Query) toCursor(fieldValues []interface{}, ds *DocumentSnapshot, before bool, orders []order) (*pb.Cursor, error) {
	var vals []*pb.Value
	var err error
	if ds != nil {
		vals, err = q.docSnapshotToCursorValues(ds, orders)
	} else if len(fieldValues) != 0 {
		vals, err = q.fieldValuesToCursorValues(fieldValues)
	} else {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pb.Cursor{Values: vals, Before: before}, nil
}

// toPositionValues converts the field values to protos.
func (q *Query) fieldValuesToCursorValues(fieldValues []interface{}) ([]*pb.Value, error) {
	if len(fieldValues) != len(q.orders) {
		return nil, errors.New("firestore: number of field values in StartAt/StartAfter/EndAt/EndBefore does not match number of OrderBy fields")
	}
	vals := make([]*pb.Value, len(fieldValues))
	var err error
	for i, ord := range q.orders {
		fval := fieldValues[i]
		if ord.isDocumentID() {
			// TODO(jba): support DocumentRefs as well as strings.
			// TODO(jba): error if document ref does not belong to the right collection.
			docID, ok := fval.(string)
			if !ok {
				return nil, fmt.Errorf("firestore: expected doc ID for DocumentID field, got %T", fval)
			}
			vals[i] = &pb.Value{&pb.Value_ReferenceValue{q.collectionPath() + "/" + docID}}
		} else {
			var sawTransform bool
			vals[i], sawTransform, err = toProtoValue(reflect.ValueOf(fval))
			if err != nil {
				return nil, err
			}
			if sawTransform {
				return nil, errors.New("firestore: ServerTimestamp disallowed in query value")
			}
		}
	}
	return vals, nil
}

func (q *Query) docSnapshotToCursorValues(ds *DocumentSnapshot, orders []order) ([]*pb.Value, error) {
	// TODO(jba): error if doc snap does not belong to the right collection.
	vals := make([]*pb.Value, len(orders))
	for i, ord := range orders {
		if ord.isDocumentID() {
			dp, qp := ds.Ref.Parent.Path, q.collectionPath()
			if dp != qp {
				return nil, fmt.Errorf("firestore: document snapshot for %s passed to query on %s", dp, qp)
			}
			vals[i] = &pb.Value{&pb.Value_ReferenceValue{ds.Ref.Path}}
		} else {
			val, err := valueAtPath(ord.fieldPath, ds.proto.Fields)
			if err != nil {
				return nil, err
			}
			vals[i] = val
		}
	}
	return vals, nil
}

type filter struct {
	fieldPath FieldPath
	op        string
	value     interface{}
}

func (f filter) toProto() (*pb.StructuredQuery_Filter, error) {
	if err := f.fieldPath.validate(); err != nil {
		return nil, err
	}
	var op pb.StructuredQuery_FieldFilter_Operator
	switch f.op {
	case "<":
		op = pb.StructuredQuery_FieldFilter_LESS_THAN
	case "<=":
		op = pb.StructuredQuery_FieldFilter_LESS_THAN_OR_EQUAL
	case ">":
		op = pb.StructuredQuery_FieldFilter_GREATER_THAN
	case ">=":
		op = pb.StructuredQuery_FieldFilter_GREATER_THAN_OR_EQUAL
	case "==":
		op = pb.StructuredQuery_FieldFilter_EQUAL
	default:
		return nil, fmt.Errorf("firestore: invalid operator %q", f.op)
	}
	val, sawTransform, err := toProtoValue(reflect.ValueOf(f.value))
	if err != nil {
		return nil, err
	}
	if sawTransform {
		return nil, errors.New("firestore: ServerTimestamp disallowed in query value")
	}
	return &pb.StructuredQuery_Filter{
		FilterType: &pb.StructuredQuery_Filter_FieldFilter{
			&pb.StructuredQuery_FieldFilter{
				Field: fref(f.fieldPath),
				Op:    op,
				Value: val,
			},
		},
	}, nil
}

type order struct {
	fieldPath FieldPath
	dir       Direction
}

func (r order) isDocumentID() bool {
	return len(r.fieldPath) == 1 && r.fieldPath[0] == DocumentID
}

func (r order) toProto() (*pb.StructuredQuery_Order, error) {
	if err := r.fieldPath.validate(); err != nil {
		return nil, err
	}
	return &pb.StructuredQuery_Order{
		Field:     fref(r.fieldPath),
		Direction: pb.StructuredQuery_Direction(r.dir),
	}, nil
}

func fref(fp FieldPath) *pb.StructuredQuery_FieldReference {
	return &pb.StructuredQuery_FieldReference{fp.toServiceFieldPath()}
}

func trunc32(i int) int32 {
	if i > math.MaxInt32 {
		i = math.MaxInt32
	}
	return int32(i)
}

// Documents returns an iterator over the query's resulting documents.
func (q Query) Documents(ctx context.Context) *DocumentIterator {
	return &DocumentIterator{
		ctx: withResourceHeader(ctx, q.c.path()),
		q:   &q,
		err: checkTransaction(ctx),
	}
}

// DocumentIterator is an iterator over documents returned by a query.
type DocumentIterator struct {
	ctx          context.Context
	q            *Query
	tid          []byte // transaction ID, if any
	streamClient pb.Firestore_RunQueryClient
	err          error
}

// Next returns the next result. Its second return value is iterator.Done if there
// are no more results. Once Next returns Done, all subsequent calls will return
// Done.
func (it *DocumentIterator) Next() (*DocumentSnapshot, error) {
	if it.err != nil {
		return nil, it.err
	}
	client := it.q.c
	if it.streamClient == nil {
		sq, err := it.q.toProto()
		if err != nil {
			it.err = err
			return nil, err
		}
		req := &pb.RunQueryRequest{
			Parent:    it.q.parentPath,
			QueryType: &pb.RunQueryRequest_StructuredQuery{sq},
		}
		if it.tid != nil {
			req.ConsistencySelector = &pb.RunQueryRequest_Transaction{it.tid}
		}
		it.streamClient, it.err = client.c.RunQuery(it.ctx, req)
		if it.err != nil {
			return nil, it.err
		}
	}
	var res *pb.RunQueryResponse
	var err error
	for {
		res, err = it.streamClient.Recv()
		if err == io.EOF {
			err = iterator.Done
		}
		if err != nil {
			it.err = err
			return nil, it.err
		}
		if res.Document != nil {
			break
		}
		// No document => partial progress; keep receiving.
	}
	docRef, err := pathToDoc(res.Document.Name, client)
	if err != nil {
		it.err = err
		return nil, err
	}
	doc, err := newDocumentSnapshot(docRef, res.Document, client)
	if err != nil {
		it.err = err
		return nil, err
	}
	return doc, nil
}

// GetAll returns all the documents remaining from the iterator.
func (it *DocumentIterator) GetAll() ([]*DocumentSnapshot, error) {
	var docs []*DocumentSnapshot
	for {
		doc, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// TODO(jba): Does the iterator need a Stop or Close method? I don't think so--
// I don't think the client can terminate a streaming receive except perhaps
// by cancelling the context, and the user can do that themselves if they wish.
// Find out for sure.
