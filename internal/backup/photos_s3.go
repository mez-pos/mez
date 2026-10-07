package backup

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// defaultS3Region is what a service with an endpoint and no region is
// signed for, as Litestream does: the S3-compatibles take it or ignore it.
const defaultS3Region = "us-east-1"

// bucketPhotos keeps photos in a bucket, under <path>/photos/.
//
// It is the one place mez calls the AWS SDK itself (ADR-275): Litestream's
// client is for its own files. The client is built from what backup.json
// says and nothing else. Litestream builds its own through the SDK's
// loader, which also reads AWS_* from the environment; this does not.
type bucketPhotos struct {
	client *s3.Client
	bucket string
	prefix string // ends in a slash
}

func newBucketPhotos(cfg S3) bucketPhotos {
	region := cfg.Region
	if region == "" {
		region = defaultS3Region
	}
	opts := s3.Options{
		Region: region,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey}, nil
		}),
	}
	if cfg.Endpoint != "" {
		opts.BaseEndpoint = aws.String(cfg.Endpoint)
		opts.UsePathStyle = true
		// The S3-compatibles do not all take the checksum trailers the
		// SDK adds by default; Litestream turns them off for the same
		// services, and a photo's name is its checksum already.
		opts.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		opts.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}
	return bucketPhotos{
		client: s3.New(opts),
		bucket: cfg.Bucket,
		prefix: cfg.prefix() + "/" + photosDir + "/",
	}
}

func (b bucketPhotos) list(ctx context.Context) ([]string, error) {
	var names []string
	pages := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(b.prefix),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			name := strings.TrimPrefix(aws.ToString(obj.Key), b.prefix)
			if photoName.MatchString(name) {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func (b bucketPhotos) put(ctx context.Context, name string, body []byte) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(b.prefix + name),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("image/jpeg"),
	})
	return err
}

// folders names the "folders" directly under a prefix ending in a slash.
func (b bucketPhotos) folders(ctx context.Context, prefix string) ([]string, error) {
	var names []string
	pages := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(b.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range page.CommonPrefixes {
			name := strings.TrimSuffix(strings.TrimPrefix(aws.ToString(p.Prefix), prefix), "/")
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// putNote writes a small text object at a key of the caller's choosing.
func (b bucketPhotos) putNote(ctx context.Context, key, text string) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(text),
		ContentType: aws.String("text/plain; charset=utf-8"),
	})
	return err
}

func (b bucketPhotos) open(ctx context.Context, name string) (io.ReadCloser, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.prefix + name),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}
