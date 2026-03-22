package ilink

import (
	"path/filepath"
	"strings"
)

var extToMIME = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".zip":  "application/zip",
	".tar":  "application/x-tar",
	".gz":   "application/gzip",
	".mp3":  "audio/mpeg",
	".ogg":  "audio/ogg",
	".wav":  "audio/wav",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
}

var mimeToExt = map[string]string{
	"application/pdf":       ".pdf",
	"application/msword":    ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
	"application/vnd.ms-powerpoint":                                     ".ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"text/plain":         ".txt",
	"text/csv":           ".csv",
	"application/zip":    ".zip",
	"application/x-tar":  ".tar",
	"application/gzip":   ".gz",
	"audio/mpeg":         ".mp3",
	"audio/ogg":          ".ogg",
	"audio/wav":          ".wav",
	"video/mp4":          ".mp4",
	"video/quicktime":    ".mov",
	"video/webm":         ".webm",
	"video/x-matroska":   ".mkv",
	"video/x-msvideo":    ".avi",
	"image/png":          ".png",
	"image/jpeg":         ".jpg",
	"image/gif":          ".gif",
	"image/webp":         ".webp",
	"image/bmp":          ".bmp",
}

// MIMEFromFilename returns the MIME type for a filename based on its extension.
// Returns "application/octet-stream" for unknown extensions.
func MIMEFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := extToMIME[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// ExtensionFromMIME returns the file extension for a MIME type.
// Returns ".bin" for unknown types.
func ExtensionFromMIME(mime string) string {
	// Strip parameters (e.g., "text/plain; charset=utf-8" -> "text/plain")
	if idx := strings.IndexByte(mime, ';'); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	if ext, ok := mimeToExt[mime]; ok {
		return ext
	}
	return ".bin"
}

// IsImageMIME reports whether the MIME type is an image type.
func IsImageMIME(mime string) bool { return strings.HasPrefix(mime, "image/") }

// IsVideoMIME reports whether the MIME type is a video type.
func IsVideoMIME(mime string) bool { return strings.HasPrefix(mime, "video/") }
