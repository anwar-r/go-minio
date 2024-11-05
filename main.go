package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	_ "go-minio/docs"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

var minioClient *minio.Client

const (
	bucketName = "judge-me-review"
)

func initMinIO() {
	var err error
	// Ensure that the port here is correct for S3 API requests
	endpoint := "localhost:9000" // Use the correct MinIO S3 API port
	accessKeyID := "zeaYDByeoWIEuDKno4XC"
	secretAccessKey := "JazeZfOA6a0ZKitT43imvNav6P2g4aPgUEtV9fbk"
	useSSL := false

	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		log.Fatalln(err)
	}

	err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: ""})
	if err != nil {
		exists, errBucketExists := minioClient.BucketExists(context.Background(), bucketName)
		if errBucketExists == nil && exists {
			log.Printf("Bucket %s already exists", bucketName)
		} else {
			log.Fatalln(err)
		}
	} else {
		log.Printf("Successfully created bucket %s", bucketName)
	}
}

// @Summary Upload file to MinIO
// @Description Upload a file to the specified bucket
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Success 200 {string} string "File uploaded successfully"
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /upload [post]
func uploadFile(w http.ResponseWriter, r *http.Request) {
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File upload error", http.StatusBadRequest)
		return
	}
	defer file.Close()

	_, err = minioClient.PutObject(context.Background(), bucketName, handler.Filename, file, handler.Size, minio.PutObjectOptions{ContentType: handler.Header.Get("Content-Type")})
	if err != nil {
		http.Error(w, "Error uploading file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File uploaded successfully: %s", handler.Filename)
}

// @Summary Get file from MinIO
// @Description Retrieve a file from MinIO by filename
// @Produce octet-stream
// @Param filename path string true "File name"
// @Success 200
// @Failure 404 {string} string "File not found"
// @Router /files/{filename} [get]
func getFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["filename"]

	object, err := minioClient.GetObject(context.Background(), bucketName, fileName, minio.GetObjectOptions{})
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer object.Close()

	stat, err := object.Stat()
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
	w.Header().Set("Content-Type", stat.ContentType)
	http.ServeContent(w, r, fileName, stat.LastModified, object)
}

// @Summary Delete file from MinIO
// @Description Delete a file from MinIO by filename
// @Param filename path string true "File name"
// @Success 200 {string} string "File deleted successfully"
// @Failure 404 {string} string "File not found"
// @Router /files/{filename} [delete]
func deleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["filename"]

	err := minioClient.RemoveObject(context.Background(), bucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, "File deleted successfully: %s", fileName)
}

// GeneratePresignedURL generates a pre-signed URL for an object in MinIO
// @Summary Generate pre-signed URL for MinIO object
// @Description Generate a pre-signed URL for an object in MinIO by filename
// @Produce plain
// @Param filename path string true "File name"
// @Success 200 {string} string "Pre-signed URL"
// @Router /presigned/{filename} [get]
func generatePresignedURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["filename"]

	expiration := time.Minute * 15 // URL expiration time

	reqParams := make(url.Values)
	presignedURL, err := minioClient.PresignedGetObject(context.Background(), bucketName, fileName, expiration, reqParams)
	if err != nil {
		http.Error(w, "Error generating pre-signed URL", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Pre-signed URL: %s", presignedURL.String())
}

func main() {
	// Initialize MinIO
	initMinIO()

	// Initialize router
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/upload", uploadFile).Methods(http.MethodPost)
	router.HandleFunc("/files/{filename}", getFile).Methods(http.MethodGet)
	router.HandleFunc("/files/{filename}", deleteFile).Methods(http.MethodDelete)
	router.HandleFunc("/presigned/{filename}", generatePresignedURL).Methods(http.MethodGet)

	// Swagger documentation
	router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("http://localhost:1111/swagger/doc.json"), // Change to your Swagger URL

		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	)).Methods(http.MethodGet)

	// Start the server
	port := "1111"
	log.Printf("Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
