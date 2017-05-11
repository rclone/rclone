// AttributeValue Marshaling and Unmarshaling Helpers
//
// Utility helpers to marshal and unmarshal AttributeValue to and
// from Go types can be found in the dynamodbattribute sub package. This package
// provides has specialized functions for the common ways of working with
// AttributeValues. Such as map[string]*AttributeValue, []*AttributeValue, and
// directly with *AttributeValue. This is helpful for marshaling Go types for API
// operations such as PutItem, and unmarshaling Query and Scan APIs' responses.
//
// See the dynamodbattribute package documentation for more information.
// https://docs.aws.amazon.com/sdk-for-go/api/service/dynamodb/dynamodbattribute/
//
// AttributeValue Marshaling
//
// To marshal a Go type to an AttributeValue you can use the Marshal
// functions in the dynamodbattribute package. There are specialized versions
// of these functions for collections of AttributeValue, such as maps and lists.
//
// The following example uses MarshalMap to convert the Record Go type to a
// dynamodb.AttributeValue type and use the value to make a PutItem API request.
//
//     type Record struct {
//         ID     string
//         URLs   []string
//     }
//
//     //...
//
//     r := Record{
//         ID:   "ABC123",
//         URLs: []string{
//             "https://example.com/first/link",
//             "https://example.com/second/url",
//         },
//     }
//     av, err := dynamodbattribute.MarshalMap(r)
//     if err != nil {
//         panic(fmt.Sprintf("failed to DynamoDB marshal Record, %v", err))
//     }
//
//     _, err = svc.PutItem(&dynamodb.PutItemInput{
//         TableName: aws.String(myTableName),
//         Item:      av,
//     })
//     if err != nil {
//         panic(fmt.Sprintf("failed to put Record to DynamoDB, %v", err))
//     }
//
// AttributeValue Unmarshaling
//
// To unmarshal a dynamodb.AttributeValue to a Go type you can use the Unmarshal
// functions in the dynamodbattribute package. There are specialized versions
// of these functions for collections of AttributeValue, such as maps and lists.
//
// The following example will unmarshal the DynamoDB's Scan API operation. The
// Items returned by the operation will be unmarshaled into the slice of Records
// Go type.
//
//     type Record struct {
//         ID     string
//         URLs   []string
//     }
//
//     //...
//
//     var records []Record
//
//     // Use the ScanPages method to perform the scan with pagination. Use
//     // just Scan method to make the API call without pagination.
//     err := svc.ScanPages(&dynamodb.ScanInput{
//         TableName: aws.String(myTableName),
//     }, func(page *dynamodb.ScanOutput, last bool) bool {
//         recs := []Record{}
//
//         err := dynamodbattribute.UnmarshalListOfMaps(page.Items, &recs)
//         if err != nil {
//              panic(fmt.Sprintf("failed to unmarshal Dynamodb Scan Items, %v", err))
//         }
//
//         records = append(records, recs...)
//
//         return true // keep paging
//     })
package dynamodb
