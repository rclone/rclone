package s3

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalAWSClient(t *testing.T) {
	for _, remote := range []string{"", ":memory,description=conditional-aws:"} {
		t.Run(remote, func(t *testing.T) {
			var accessKey, secretKey string
			core, _, bucket := newMultipartTestServerOpt(t, remote, false, func(opt *Options) {
				accessKey, secretKey, _ = strings.Cut(opt.AuthKey[0], ",")
			})
			client := awss3.New(awss3.Options{
				BaseEndpoint:               aws.String(core.EndpointURL().String()),
				Region:                     "us-east-1",
				Credentials:                credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
				UsePathStyle:               true,
				RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
				ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
			})
			ctx := context.Background()
			const original, replacement = "old contents", "new contents"
			_, err := core.PutObject(ctx, bucket, "source", strings.NewReader(replacement), int64(len(replacement)), "", "", minio.PutObjectOptions{})
			require.NoError(t, err)
			for _, method := range []string{"Put", "Copy", "Multipart"} {
				for _, quoted := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/Quoted=%t", method, quoted), func(t *testing.T) {
						key := fmt.Sprintf("%s-%t", method, quoted)
						_, err := core.PutObject(ctx, bucket, key, strings.NewReader(original), int64(len(original)), "", "", minio.PutObjectOptions{})
						require.NoError(t, err)
						info, err := core.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
						require.NoError(t, err)
						etag := info.ETag
						if quoted {
							etag = `"` + etag + `"`
						}
						for attempt := range 2 {
							switch method {
							case "Put":
								_, err = client.PutObject(ctx, &awss3.PutObjectInput{
									Bucket: aws.String(bucket), Key: aws.String(key),
									Body: strings.NewReader(replacement), IfMatch: aws.String(etag),
								})
							case "Copy":
								_, err = client.CopyObject(ctx, &awss3.CopyObjectInput{
									Bucket: aws.String(bucket), Key: aws.String(key),
									CopySource: aws.String(bucket + "/source"), IfMatch: aws.String(etag),
								})
							case "Multipart":
								id, createErr := core.NewMultipartUpload(ctx, bucket, key, minio.PutObjectOptions{})
								require.NoError(t, createErr)
								t.Cleanup(func() { _ = core.AbortMultipartUpload(ctx, bucket, key, id) })
								part, putErr := core.PutObjectPart(ctx, bucket, key, id, 1, strings.NewReader(replacement), int64(len(replacement)), minio.PutObjectPartOptions{})
								require.NoError(t, putErr)
								_, err = client.CompleteMultipartUpload(ctx, &awss3.CompleteMultipartUploadInput{
									Bucket: aws.String(bucket), Key: aws.String(key),
									UploadId: aws.String(id), IfMatch: aws.String(etag),
									MultipartUpload: &types.CompletedMultipartUpload{
										Parts: []types.CompletedPart{{PartNumber: aws.Int32(1), ETag: aws.String(part.ETag)}},
									},
								})
							}
							if attempt == 0 {
								require.NoError(t, err)
							} else {
								var apiErr smithy.APIError
								require.ErrorAs(t, err, &apiErr)
								assert.Equal(t, "PreconditionFailed", apiErr.ErrorCode())
								var responseErr *smithyhttp.ResponseError
								require.ErrorAs(t, err, &responseErr)
								assert.Equal(t, http.StatusPreconditionFailed, responseErr.HTTPStatusCode())
							}
							reader, _, _, getErr := core.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
							require.NoError(t, getErr)
							got, readErr := io.ReadAll(reader)
							require.NoError(t, reader.Close())
							require.NoError(t, readErr)
							assert.Equal(t, replacement, string(got))
						}
					})
				}
			}
		})
	}
}
