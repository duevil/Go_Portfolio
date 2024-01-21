package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/russross/blackfriday/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"io"
	"log"
	"time"
)

var (
	// ErrNotMD is returned by MarkdownToHTML when the requested file is not markdown
	ErrNotMD = errors.New("file is not markdown")
)

// MetaData contains metadata about a file
type MetaData struct {
	Type     Filetype  `bson:"type,omitempty"`
	Modified time.Time `bson:"modified,omitempty"`
}

// GridFSFile represents a file stored in the database
type GridFSFile struct {
	id       primitive.ObjectID `bson:"_id"`
	Name     string             `bson:"filename"`
	Size     int64              `bson:"length"`
	MetaData MetaData           `bson:"metadata"`
}

// HTMLData contains the data needed to render a markdown file as HTML
type HTMLData struct {
	Title    string
	Body     template.HTML
	Modified time.Time
}

// Read reads the file from the database; if the file is already read, i.e. the id
// is already set, it does nothing
func (f *GridFSFile) Read() error {
	// if id is already set, we assume that the file was already read
	if !f.id.IsZero() {
		return nil
	}

	// query and decode file from database
	cur, err := bucket.Find(f)
	if err != nil {
		log.Println("Error finding file:", err)
		return err
	}
	if !cur.Next(context.Background()) {
		return gridfs.ErrFileNotFound
	}
	log.Println("Found file", cur.Current.String())
	return cur.Decode(&f)
}

// Upload uploads a file to the database; if the file already exists, it is deleted
// first and then re-uploaded
func (f *GridFSFile) Upload(r io.Reader) error {
	opts := options.GridFSUpload().SetMetadata(f.MetaData)
	_, err := bucket.UploadFromStream(f.Name, r, opts)
	return err
}

// Download downloads a file from the database to the given writer
func (f *GridFSFile) Download(w io.Writer) error {
	err := f.Read()
	if err != nil {
		return err
	}
	_, err = bucket.DownloadToStream(f.id, w)
	log.Println("here", err)
	return err
}

// Rename renames a file in the database
func (f *GridFSFile) Rename(newName string) error {
	err := f.Read()
	if err != nil {
		return err
	}
	return bucket.Rename(f.id, newName)
}

// Delete deletes a file from the database
func (f *GridFSFile) Delete() error {
	err := f.Read()
	if err != nil {
		return err
	}
	return bucket.Delete(f.id)
}

/*// setMimeType sets the mime type of a file in the database; it is called by
// Upload
func (f *GridFSFile) setMimeType() error {
	err := f.Read()
	if err != nil {
		return err
	}

	// if mime type is already set, return
	if f.MetaData.MimeType != "" {
		return nil
	}

	// to set the mime type, we open the file and try to detect the mime type using
	// the mimetype package
	ds, err := bucket.OpenDownloadStream(f.id)
	if err != nil {
		return err
	}
	defer func(ds *gridfs.DownloadStream) { _ = ds.Close() }(ds)
	mime, err := mimetype.DetectReader(ds)
	if err != nil {
		return err
	}

	// set mime type in database
	f.MetaData.MimeType = mime.String()
	log.Println("Set mime type of", f.Name, "to", f.MetaData.MimeType)
	_, err = bucket.GetFilesCollection().UpdateOne(
		ctx,
		bson.M{"_id": f.id},
		bson.M{"$set": bson.M{"metadata.mime": f.MetaData.MimeType}},
	)
	return err
}*/

// MarkdownToHTML converts a markdown file to HTML using blackfriday.Run()
func (f *GridFSFile) MarkdownToHTML() (HTMLData, error) {
	if f.MetaData.Type != Markdown {
		return HTMLData{}, ErrNotMD
	}

	// convert markdown to html
	var buf bytes.Buffer
	err := f.Download(&buf)
	if err != nil {
		return HTMLData{}, err
	}
	return HTMLData{
		Title:    f.Name,
		Body:     template.HTML(blackfriday.Run(buf.Bytes())),
		Modified: f.MetaData.Modified,
	}, nil
}

// ListGridFSFiles lists all files in the GridFS bucket
func ListGridFSFiles() ([]GridFSFile, error) {
	var files []GridFSFile
	cur, err := bucket.Find(bson.M{})
	if err != nil {
		return nil, err
	}
	err = cur.All(context.Background(), &files)
	return files, err
}
