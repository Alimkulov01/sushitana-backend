package filemanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.uber.org/fx"

	"sushitana/pkg/config"
	"sushitana/pkg/logger"
)

var Module = fx.Provide(New)

type File interface {
	Download(ctx context.Context, dir, filename string) (io.Reader, error)
	Upload(ctx context.Context, uploadFile io.Reader, dir string, filename string) error
	Remove(ctx context.Context, dir, filename string) error
}

type Params struct {
	fx.In

	Logger logger.Logger
	Config config.IConfig
}

type file struct {
	logger logger.Logger
	config config.IConfig

	awsS3ManagerDownloader *s3manager.Downloader
	awsS3ManagerUploader   *s3manager.Uploader
	awsS3                  *s3.S3
	bucket                 string
}

func New(p Params) File {
	f := &file{
		logger: p.Logger,
		config: p.Config,
		bucket: "stolik-utechgroup",
	}

	crd := credentials.NewStaticCredentials(
		f.config.GetString("aws_access_key_id"),
		f.config.GetString("aws_secret_access_key"),
		"",
	)
	region := aws.String(f.config.GetString("aws_region"))

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{Region: region, Credentials: crd},
	}))
	f.awsS3ManagerUploader = s3manager.NewUploader(sess)
	f.awsS3ManagerDownloader = s3manager.NewDownloader(sess)
	f.awsS3 = s3.New(sess)

	return f
}

func (f *file) Download(ctx context.Context, dir, fileName string) (io.Reader, error) {
	downloadedFile, err := f.s3Download(ctx, dir, fileName)
	if err != nil {
		return nil, fmt.Errorf("s3 download: %w", err)
	}
	return downloadedFile, nil
}

func (f *file) s3Download(ctx context.Context, dir, fileName string) (io.Reader, error) {
	var downloadedFile = &bytes.Buffer{}

	newFile, err := os.Create(fileName)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}

	defer func() {
		_ = newFile.Close()
		_ = os.Remove(fileName)
	}()

	_, err = f.awsS3ManagerDownloader.Download(newFile, &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(dir + fileName),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 download: %w", err)
	}

	_, err = io.Copy(downloadedFile, newFile)
	if err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}

	return downloadedFile, nil
}

func (f *file) Upload(ctx context.Context, uploadFile io.Reader, dir, fileName string) error {
	err := f.s3Upload(ctx, uploadFile, dir, fileName)
	if err != nil {
		return fmt.Errorf("upload to s3: %w", err)
	}

	return nil
}

func (f *file) s3Upload(ctx context.Context, uploadFile io.Reader, dir, fileName string) error {
	_, err := f.awsS3ManagerUploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(dir + fileName),
		Body:   uploadFile,
	})

	if err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}

	return nil
}

func (f *file) Remove(ctx context.Context, dir, filename string) error {
	_, err := f.awsS3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(dir + filename),
	})

	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}

	return nil
}
