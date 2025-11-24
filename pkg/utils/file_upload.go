package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sushitana/pkg/config"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const baseDir = "statics"

func UploadFromURL(fileURL, folder string) (string, error) {
	if fileURL == "" {
		return "", fmt.Errorf("empty file url")
	}

	cfg := config.NewConfig()

	awsRegion := cfg.GetString("aws_region")
	awsBucket := cfg.GetString("aws_s3_bucket")
	awsAccessKeyID := cfg.GetString("aws_access_key_id")
	awsSecretAccessKey := cfg.GetString("aws_secret_access_key")

	if awsRegion == "" || awsBucket == "" || awsAccessKeyID == "" || awsSecretAccessKey == "" {
		return "", fmt.Errorf("missing AWS configuration values")
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	targetPath := filepath.Join(baseDir, folder)
	if err := os.MkdirAll(targetPath, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	filename := fmt.Sprintf("%d.jpg", time.Now().UnixNano())
	fullPath := filepath.Join(targetPath, filename)

	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(awsRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(awsAccessKeyID, awsSecretAccessKey, ""),
		),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer file.Close()

	key := fmt.Sprintf("%s/%s", folder, filename)
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(awsBucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to s3: %w", err)
	}

	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", awsBucket, awsRegion, key)

	_ = os.Remove(fullPath)

	return publicURL, nil
}
