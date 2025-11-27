package utils

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// imageMimeTypes maps file extensions to their MIME types
var imageMimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".webp": "image/webp",
}

// resumeMimeTypes maps resume file extensions to their MIME types
var resumeMimeTypes = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

// UploadFile uploads a file (image, pdf, doc...) to an AWS S3 bucket with metadata.
func UploadFile(file *multipart.FileHeader, folderName string) (string, error) {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		awsRegion = "us-east-1"
		log.Printf("AWS_REGION not set, using default: %s", awsRegion)
	}

	bucketName := os.Getenv("AWS_S3_BUCKET")
	if bucketName == "" {
		return "", fmt.Errorf("AWS_S3_BUCKET environment variable is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		return "", fmt.Errorf("error loading AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	f, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(file.Filename))
	uniqueID := uuid.New().String()

	// ✅ Fayl S3da folder ichida joylashadi
	key := fmt.Sprintf("%s/%s%s", folderName, uniqueID, ext)

	// ✅ MIME turini aniqlash (image yoki resume)
	contentType := "application/octet-stream"
	if v, ok := imageMimeTypes[ext]; ok {
		contentType = v
	} else if v, ok := resumeMimeTypes[ext]; ok {
		contentType = v
	}

	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024
		u.Concurrency = 3
	})
	result, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType),
	})
	fmt.Println(err)
	if err != nil {
		log.Printf("S3 UPLOAD ERROR: bucket=%s key=%s contentType=%s err=%v", bucketName, key, contentType, err)
		return "", fmt.Errorf("error uploading file to S3: %w", err)
	}
	return result.Location, nil
}

// UploadImage is a backward-compatible helper for images only
func UploadImage(file *multipart.FileHeader, folderName string) (string, error) {
	return UploadFile(file, folderName)
}
