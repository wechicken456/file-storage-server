package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxUploadSize = 10 << 20 // 10MB
	r.ParseMultipartForm(maxUploadSize)
	thumbnailFile, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse thumbnail form", err)
		return
	}
	defer thumbnailFile.Close()

	// save image to local filesystem
	// path is "/assets/<videoID>.<mediaType>"
	// second way: mediaType := fileHeader.Filename[strings.LastIndex(fileHeader.Filename, ".")+1:] // get file extension
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" { // only accept png and jpeg
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	// get the extension from the media type, then create the thumbnail file on disk
	mediaType = mediaType[strings.LastIndex(mediaType, "/")+1:]
	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create random bytes", err)
		return
	}
	thumbnailPath := base64.RawURLEncoding.EncodeToString(randBytes) + "." + mediaType
	thumbnailPath = filepath.Join(cfg.assetsRoot, thumbnailPath)
	dest, err := os.Create(thumbnailPath)
	fmt.Println("thumbnail path", thumbnailPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't create thumbnail file")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}
	defer dest.Close()
	_, err = io.Copy(dest, thumbnailFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't copy thumbnail file")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}

	// get video metadata from database
	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't get video from database")
		respondWithError(w, http.StatusBadRequest, "Internal server error", err)
		return
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You cannot modify this video", err)
		return
	}

	// store thumbnail in database
	thumbnailPath = filepath.Join(string(filepath.Separator), thumbnailPath) // add a leading slash to the path
	metadata.ID = videoID
	metadata.CreatedAt = time.Now()
	metadata.UpdatedAt = time.Now()
	metadata.ThumbnailURL = &thumbnailPath
	if err = cfg.db.UpdateVideo(metadata); err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't update video to database")
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
