package main

import (
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
	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse thumbnail form", err)
		return
	}
	defer file.Close()

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

	// get the extension from the media type
	mediaType = mediaType[strings.LastIndex(mediaType, "/")+1:]
	thumbnailPath := filepath.Join(cfg.assetsRoot, videoIDString+"."+mediaType)
	dest, err := os.Create(thumbnailPath)
	fmt.Println("thumbnail path", thumbnailPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}
	defer dest.Close()
	_, err = io.Copy(dest, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy thumbnail file", err)
		return
	}

	// get video metadata from database
	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video", err)
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
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
