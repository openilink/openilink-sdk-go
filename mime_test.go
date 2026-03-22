package ilink

import "testing"

func TestMIMEFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.JPEG", "image/jpeg"},
		{"image.png", "image/png"},
		{"image.gif", "image/gif"},
		{"image.webp", "image/webp"},
		{"image.bmp", "image/bmp"},
		{"video.mp4", "video/mp4"},
		{"video.mov", "video/quicktime"},
		{"video.webm", "video/webm"},
		{"video.mkv", "video/x-matroska"},
		{"video.avi", "video/x-msvideo"},
		{"doc.pdf", "application/pdf"},
		{"doc.doc", "application/msword"},
		{"doc.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"sheet.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"audio.mp3", "audio/mpeg"},
		{"audio.wav", "audio/wav"},
		{"archive.zip", "application/zip"},
		{"data.csv", "text/csv"},
		{"notes.txt", "text/plain"},
		{"unknown.xyz", "application/octet-stream"},
		{"noext", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := MIMEFromFilename(tt.filename)
		if got != tt.want {
			t.Errorf("MIMEFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestExtensionFromMIME(t *testing.T) {
	tests := []struct {
		mime string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"video/mp4", ".mp4"},
		{"application/pdf", ".pdf"},
		{"text/plain", ".txt"},
		{"text/plain; charset=utf-8", ".txt"},
		{"unknown/type", ".bin"},
		{"", ".bin"},
	}

	for _, tt := range tests {
		got := ExtensionFromMIME(tt.mime)
		if got != tt.want {
			t.Errorf("ExtensionFromMIME(%q) = %q, want %q", tt.mime, got, tt.want)
		}
	}
}

func TestIsImageMIME(t *testing.T) {
	if !IsImageMIME("image/png") {
		t.Error("image/png should be image")
	}
	if !IsImageMIME("image/jpeg") {
		t.Error("image/jpeg should be image")
	}
	if IsImageMIME("video/mp4") {
		t.Error("video/mp4 should not be image")
	}
	if IsImageMIME("application/pdf") {
		t.Error("application/pdf should not be image")
	}
}

func TestIsVideoMIME(t *testing.T) {
	if !IsVideoMIME("video/mp4") {
		t.Error("video/mp4 should be video")
	}
	if !IsVideoMIME("video/webm") {
		t.Error("video/webm should be video")
	}
	if IsVideoMIME("image/png") {
		t.Error("image/png should not be video")
	}
}
