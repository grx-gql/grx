package core

import (
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
)

// MediaTypeMultipartForm is the request Content-Type used by the GraphQL
// multipart request specification for file uploads.
const MediaTypeMultipartForm = "multipart/form-data"

// Upload is a file received through a GraphQL multipart request
// (https://github.com/jaydenseric/graphql-multipart-request-spec).
//
// Transports substitute an Upload in place of the null placeholder that the
// request's "map" field referenced, so resolvers read it directly from their
// arguments. Call [Upload.Open] to stream the file contents; the underlying
// handle is owned by the HTTP request and is closed when the request ends.
type Upload struct {
	Filename    string
	ContentType string
	Size        int64

	header *multipart.FileHeader
}

// NewUpload builds an Upload from a parsed multipart file header.
func NewUpload(header *multipart.FileHeader) *Upload {
	if header == nil {
		return nil
	}
	ct := header.Header.Get("Content-Type")
	return &Upload{
		Filename:    header.Filename,
		ContentType: ct,
		Size:        header.Size,
		header:      header,
	}
}

// Open returns a reader over the uploaded file contents. The caller is
// responsible for closing the returned file.
func (u *Upload) Open() (multipart.File, error) {
	return u.header.Open()
}

// IsMultipartRequest reports whether r carries a GraphQL multipart request
// body (Content-Type: multipart/form-data).
func IsMultipartRequest(r *http.Request) bool {
	raw := strings.TrimSpace(r.Header.Get("Content-Type"))
	if raw == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return mediaType == MediaTypeMultipartForm
}
