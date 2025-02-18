package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
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

	fmt.Println(tempFile.Name())
	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get video's aspect ratio: %s", err)
		return
	}

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
	if aspectRatio == "16:9" {
		s3FileKey = "landscape/" + s3FileKey
	} else if aspectRatio == "9:16" {
		s3FileKey = "portrait/" + s3FileKey
	} else {
		s3FileKey = "other/" + s3FileKey
	}
	fmt.Println(s3FileKey)

	s3PutObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(s3FileKey),
		Body:        tempFile,
		ContentType: aws.String("video/mp4"),
	}
	_, err = cfg.s3Client.PutObject(context.TODO(), s3PutObjectInput)

	if err != nil {
		fmt.Println("Couldn't put object into S3")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}

	// update video url in database
	var _url string = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, s3FileKey)
	videoMetadata.VideoURL = &_url
	if err = cfg.db.UpdateVideo(videoMetadata); err != nil {
		fmt.Println("Couldn't update video in database")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetadata)
}

func gcd(a int, b int) int {
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

func getVideoAspectRatio(filePath string) (string, error) {
	command := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-select_streams", "v:0", "-show_entries", "stream=width,height", filePath)
	commandStdout := &bytes.Buffer{}
	command.Stdout = commandStdout
	err := command.Run()
	if err != nil {
		return "", err
	}
	result := struct {
		Program []struct {
		} `json:"program"`
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}{}
	json.Unmarshal(commandStdout.Bytes(), &result)
	if len(result.Streams) == 0 {
		return "", fmt.Errorf("No video stream found")
	}

	var width int = result.Streams[0].Width
	var height int = result.Streams[0].Height
	var _gcd int = gcd(width, height)
	var fwidth float64 = float64(width)
	var fheight float64 = float64(height)
	var fgcd float64 = float64(_gcd)
	var aspectRatio string
	if math.Abs((fwidth/fgcd)/(fheight/fgcd))-float64(9.0/16.0) <= 0.01 {
		aspectRatio = "9:16"
	} else if math.Abs((fheight/fgcd)/(fwidth/fgcd))-float64(16.0/9.0) <= 0.01 {
		aspectRatio = "16:9"
	} else {
		aspectRatio = "other"
	}

	return aspectRatio, nil
}
