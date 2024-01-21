package files

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/russross/blackfriday/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"io"
	"time"
)

// MetaData is used to store the Config object and the last modified time of a file.
type MetaData struct {
	// content is used to store the file content of markdown files.
	content      []byte    `bson:"content,omitempty"`
	Config       Config    `bson:"config,omitempty"`
	LastModified time.Time `bson:"last_modified,omitempty"`
	MimeType     string    `bson:"mime_type,omitempty"`
}

// FileDB represents a file in the database as it is stored in the GridFS files collection.
// The Metadata field is used to store the Config object for the file.
//   - Asset files are stored as GridFS files with the content field set to nil.
//   - Markdown files are not stored as GridFS files, but as documents in the
//     files collection, with the content field set to the file content.
//   - Static files are stored as documents in the files collection, with the
//     content field set to nil.
//   - Templated files will not be stored in the database and are thus ignored.
type FileDB struct {
	id       primitive.ObjectID `bson:"_id,omitempty"`
	Length   int64              `bson:"length,omitempty"`
	Name     string             `bson:"filename,omitempty"`
	MetaData MetaData           `bson:"metadata,omitempty"`
}

// Files is a slice of FileDB objects. It's primary use is to marshal the FileDB
// objects into JSON data.
type Files []FileDB

// ToHTML returns the content of the file as a template.HTML object. If the
// content is not set, it is queried from the database using the file as the
// query filter.
//
// If the file is not of type Markdown, the returned value is undefined.
//
// Returns an ErrNotFound error if the file was not found in the database.
func (f *FileDB) ToHTML(ctx context.Context, bucket *gridfs.Bucket) (template.HTML, error) {
	if f.MetaData.content == nil {
		// content is not set, so we need to query it
		opts := contentProjection[options.FindOneOptions](options.FindOne(), true)
		err := bucket.GetFilesCollection().FindOne(ctx, f, opts).Decode(&f)
		if err != nil {
			return "", errNotFound(err)
		}
	}
	return template.HTML(blackfriday.Run(f.MetaData.content)), nil
}

// Download downloads the file from the database. The file is identified by its
// id or Name. If both are set, the id is used.
//
// The file is downloaded to the given io.Writer.
//
// Returns an ErrNotFound error if the file was not found in the database.
func (f *FileDB) Download(bucket *gridfs.Bucket, snk io.Writer) (int64, error) {
	if !f.id.IsZero() {
		i, err := bucket.DownloadToStream(f.id, snk)
		return i, errNotFound(err)
	}
	if f.Name != "" {
		i, err := bucket.DownloadToStreamByName(f.Name, snk)
		return i, errNotFound(err)
	}
	return 0, errors.New("no id or filename set")
}

// Rename renames the file in the database. The file is identified by its id.
//
// Returns an ErrNotFound error if the file was not found in the database.
func (f *FileDB) Rename(ctx context.Context, bucket *gridfs.Bucket, new string) error {
	if !f.id.IsZero() {
		return errNotFound(bucket.RenameContext(ctx, f.id, new))
	}
	return errors.New("no id set")
}

// Delete deletes the file from the database. The file is identified by its id.
//
// Returns an ErrNotFound error if the file was not found in the database.
func (f *FileDB) Delete(ctx context.Context, bucket *gridfs.Bucket) error {
	if !f.id.IsZero() {
		return errNotFound(bucket.DeleteContext(ctx, f.id))
	}
	return errors.New("no id set")
}

// MarshalJSON marshals the FileDB object into JSON data. The config field is
// broken down into its individual fields.
func (fs Files) MarshalJSON() ([]byte, error) {
	type file struct {
		Title        string    `json:"title,omitempty"`
		Name         string    `json:"name,omitempty"`
		URL          string    `json:"url,omitempty"`
		LastModified time.Time `json:"last_modified,omitempty"`
		MimeType     string    `json:"mime_type,omitempty"`
		FileType     FileType  `json:"file_type,omitempty"`
		Length       int64     `json:"length,omitempty"`
	}
	var tmp []file
	for _, f := range fs {
		tmp = append(tmp, file{
			Title:        f.MetaData.Config.Title,
			Name:         f.Name,
			URL:          f.MetaData.Config.URL,
			LastModified: f.MetaData.LastModified,
			MimeType:     f.MetaData.MimeType,
			FileType:     f.MetaData.Config.FileType,
			Length:       f.Length,
		})
	}
	return json.Marshal(tmp)
}
