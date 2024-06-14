package s3

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3pkg "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cirruslabs/chacha/internal/cache"
	"io"
	"net/url"
)

type S3 struct {
	client *s3pkg.Client
	bucket string
}

type Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
}

func New(ctx context.Context, bucket string) (*S3, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &S3{
		client: s3pkg.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

func NewFromConfig(ctx context.Context, config *Config) (*S3, error) {
	awsConfig := aws.Config{
		Region: config.Region,
	}

	s3EndpointURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, err
	}

	client := s3pkg.NewFromConfig(awsConfig, func(options *s3pkg.Options) {
		options.EndpointResolverV2 = &s3EndpointResolver{url: s3EndpointURL}
	})

	_, err = client.CreateBucket(ctx, &s3pkg.CreateBucketInput{
		Bucket: aws.String(config.Bucket),
	})
	if err != nil {
		return nil, err
	}

	if config.AccessKeyID != "" {
		awsConfig.Credentials = credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.AccessKeySecret,
			"",
		)
	}

	return &S3{
		client: client,
		bucket: config.Bucket,
	}, nil
}

func (s3 *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s3.client.GetObject(ctx, &s3pkg.GetObjectInput{
		Bucket: aws.String(s3.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, convertErr(err)
	}

	return result.Body, nil
}

func (s3 *S3) Put(ctx context.Context, key string) (cache.MultipartUpload, error) {
	result, err := s3.client.CreateMultipartUpload(ctx, &s3pkg.CreateMultipartUploadInput{
		Bucket: aws.String(s3.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return &MultipartUpload{
		client:   s3.client,
		bucket:   s3.bucket,
		key:      key,
		uploadID: *result.UploadId,
	}, nil
}

func (s3 *S3) Info(ctx context.Context, key string, exact bool) (*cache.Info, error) {
	if exact {
		result, err := s3.client.HeadObject(ctx, &s3pkg.HeadObjectInput{
			Bucket: aws.String(s3.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, convertErr(err)
		}

		return &cache.Info{
			Key:  key,
			Size: *result.ContentLength,
		}, nil
	}

	result, err := s3.client.ListObjectsV2(ctx, &s3pkg.ListObjectsV2Input{
		Bucket: aws.String(s3.bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	if len(result.Contents) == 0 {
		return nil, cache.ErrNotFound
	}

	return &cache.Info{
		Key:  *result.Contents[0].Key,
		Size: *result.Contents[0].Size,
	}, nil
}

func (s3 *S3) Delete(ctx context.Context, key string) error {
	_, err := s3.client.DeleteObject(ctx, &s3pkg.DeleteObjectInput{
		Bucket: aws.String(s3.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return convertErr(err)
	}

	return nil
}

func convertErr(err error) error {
	var notFound *types.NotFound
	var noSuchKey *types.NoSuchKey

	if errors.As(err, &notFound) || errors.As(err, &noSuchKey) {
		return cache.ErrNotFound
	}

	return err
}
