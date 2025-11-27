package utils

import (
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func UploadMultipartFile(file multipart.File, originalName, folder string) (string, error) {
	ctx := context.Background()

	awsRegion := os.Getenv("AWS_REGION")
	awsBucket := os.Getenv("AWS_S3_BUCKET")
	key := fmt.Sprintf("%s/%d%s",
		folder,
		time.Now().UnixNano(),
		filepath.Ext(originalName),
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("aws config error: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(awsBucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("upload error: %w", err)
	}

	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", awsBucket, awsRegion, key)
	return publicURL, nil
}
