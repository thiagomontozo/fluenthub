package storage

import (
	"errors"
	"net/http"
	"strings"
)

var allowedMIME = map[string]struct{}{"application/pdf": {}, "application/vnd.openxmlformats-officedocument.wordprocessingml.document": {}, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {}, "image/png": {}, "image/jpeg": {}, "image/webp": {}, "audio/mpeg": {}, "audio/mp4": {}, "audio/wav": {}, "audio/x-wav": {}, "video/mp4": {}, "video/webm": {}, "text/plain; charset=utf-8": {}, "text/plain": {}}

func DetectAllowed(header []byte, declared string) (string, error) {
	detected := http.DetectContentType(header)
	if _, ok := allowedMIME[detected]; !ok {
		return "", errors.New("file MIME is not allowed")
	}
	if declared != "" && !strings.HasPrefix(declared, strings.Split(detected, ";")[0]) {
		return "", errors.New("declared MIME does not match content")
	}
	return detected, nil
}
