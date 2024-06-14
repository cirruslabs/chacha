package s3

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3pkg "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/samber/lo"
	"io"
	"sync"
)

type MultipartUpload struct {
	client   *s3pkg.Client
	bucket   string
	key      string
	uploadID string

	parts []types.CompletedPart
	mtx   sync.Mutex
}

func (mu *MultipartUpload) UploadPart(ctx context.Context, number int32, r io.Reader) error {
	result, err := mu.client.UploadPart(ctx, &s3pkg.UploadPartInput{
		Bucket:     aws.String(mu.bucket),
		Key:        aws.String(mu.key),
		UploadId:   aws.String(mu.uploadID),
		PartNumber: aws.Int32(number),
		Body:       r,
	})
	if err != nil {
		return err
	}

	mu.mtx.Lock()
	mu.parts = append(mu.parts, types.CompletedPart{
		ETag:       result.ETag,
		PartNumber: aws.Int32(number),
	})
	mu.mtx.Unlock()

	return nil
}

func (mu *MultipartUpload) Size(ctx context.Context) (int64, error) {
	result, err := mu.client.ListParts(ctx, &s3pkg.ListPartsInput{
		Bucket:   aws.String(mu.bucket),
		Key:      aws.String(mu.key),
		UploadId: aws.String(mu.uploadID),
	})
	if err != nil {
		return 0, err
	}

	return lo.SumBy(result.Parts, func(part types.Part) int64 {
		return *part.Size
	}), nil
}

func (mu *MultipartUpload) Commit(ctx context.Context) error {
	mu.mtx.Lock()
	defer mu.mtx.Unlock()

	_, err := mu.client.CompleteMultipartUpload(ctx, &s3pkg.CompleteMultipartUploadInput{
		Bucket:   aws.String(mu.bucket),
		Key:      aws.String(mu.key),
		UploadId: aws.String(mu.uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: mu.parts,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (mu *MultipartUpload) Rollback(ctx context.Context) error {
	_, err := mu.client.AbortMultipartUpload(ctx, &s3pkg.AbortMultipartUploadInput{
		Bucket:   aws.String(mu.bucket),
		Key:      aws.String(mu.key),
		UploadId: aws.String(mu.uploadID),
	})
	if err != nil {
		return err
	}

	return nil
}
