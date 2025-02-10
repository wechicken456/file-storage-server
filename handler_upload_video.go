package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 1 << 30 // 1GB
	// parse video ID
	url_param := r.PathValue("videoID")
	videoID, err := uuid.Parse(url_param)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// authenticate user to get a user ID
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// get video metadata from database to check if user is allowed to upload the video
	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't get video from database")
		respondWithError(w, http.StatusBadRequest, "Internal server error", err)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are NOT allowed to upload this video", err)
		return
	}

	// parse multipart form to get the uploaded video
	// first make sure the uploaded file is not too big, since malicious requests could overload the server
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		respondWithError(w, http.StatusBadRequest, "File too large", err)
		return
	}
	videoFile, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse video form", err)
		return
	}
	defer videoFile.Close()

	// validate the uploaded file to ensure it's an MP4 video
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	// save the uploaded file to a temporary file on disk
	tempFile, err := os.CreateTemp("", "tubely-upload-video-*.mp4")
	defer os.Remove(tempFile.Name()) // defer is LIFO, so the file will be closed before being removed
	defer tempFile.Close()
	_, err = io.Copy(tempFile, videoFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't copy video file")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}
	tempFile.Seek(0, io.SeekStart) // read the file again from the beginning, as we already moved the offset to the end of the file by copying it above

	// put the object into S3
	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		fmt.Println("Couldn't create random bytes for S3 file key")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}
	s3FileKey := base64.RawURLEncoding.EncodeToString(randBytes)
	s3FileKey = s3FileKey + ".mp4"

	s3PutObjectInput := &s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key:    &s3FileKey,
		Body:   tempFile,
	}
	cfg.s3Client.PutObject(r.Context(), s3PutObjectInput)
}
