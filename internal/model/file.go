package model

// FileType identifies the kind of file attachment.
type FileType string

const (
	FileTypePhoto    FileType = "photo"
	FileTypeDocument FileType = "document"
	FileTypeAudio    FileType = "audio"
	FileTypeVideo    FileType = "video"
	FileTypeVoice    FileType = "voice"
	FileTypeSticker  FileType = "sticker"
)

// FileRef is a lightweight, serializable reference to a file in the FileStore.
// Plugins receive FileRef (never raw bytes) and use host functions to read content.
type FileRef struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	MIMEType string   `json:"mime_type"`
	Size     int64    `json:"size"`
	FileType FileType `json:"file_type"`
}
