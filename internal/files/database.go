package files

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gabriel-vasile/mimetype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"log"
	"os"
)

// opts is an interface that is used to set the projection for a query. Needed
// to allow the use of one function for multiple options types.
type opts[T any] interface {
	SetProjection(projection interface{}) *T
}

// UploadToDB uploads the given file to the database. The file is handled based
// on its FileType:
//   - Files of type files.Asset are stored as GridFS files with the content
//     field set to nil.
//   - Files of type files.Markdown are stored as documents in the GridFS files
//     collection with the content field set to the file content.
//   - Files of type files.Static are stored as documents in the GridFS files
//     collection with the content field set to nil.
//   - Files of type files.Templated are not stored in the database and are thus
//     ignored.
//
// If the latter is the case, the content is read using file. If the file
// specified by Config.URL already exists, the content is replaced with the new
// content, else a new file is inserted.
//
// If the file is of type files.Static, src is ignored.
func UploadToDB(ctx context.Context, bucket *gridfs.Bucket, config Config, file *os.File, fi os.FileInfo) error {
	meta := MetaData{
		Config:       config,
		LastModified: fi.ModTime(),
	}
	var err error
	switch config.FileType {
	case Asset:
		mime, err := mimetype.DetectFile(file.Name())
		if err != nil {
			return err
		}
		meta.MimeType = mime.String()
		opts := options.GridFSUpload().SetMetadata(meta)
		_, err = bucket.UploadFromStream(config.URL, file, opts)
	case Markdown:
		var content []byte
		content, err = io.ReadAll(file)
		if err != nil {
			return err
		}
		meta.content = content
		fallthrough // storing markdown files in the database is the same as storing static files
	case Static:
		err = markdownOrStaticToDB(ctx, bucket, config, meta)
	}
	return err
}

// markdownOrStaticToDB is a helper function for UploadToDB used to store
// files of type Markdown or Static in the database.
func markdownOrStaticToDB(ctx context.Context, bucket *gridfs.Bucket, config Config, meta MetaData) error {
	f, err := QueryFromDB(ctx, bucket, config.URL)
	isNotFound := err != nil && errors.Is(err, ErrNotFound)
	if err != nil && !isNotFound {
		return err
	}
	if isNotFound {
		f = FileDB{
			Name:     config.URL,
			MetaData: meta,
		}
		_, err = bucket.GetFilesCollection().InsertOne(ctx, f)
	} else {
		f.MetaData = meta
		_, err = bucket.GetFilesCollection().ReplaceOne(ctx, bson.M{"_id": f.id}, f)
	}
	return errNotFound(err)
}

// QueryFromDB returns the FileDB object for the given url from the database. The
// url is used as the FileDB.Name to search for the file.
//
// Returns an ErrNotFound error if the file was not found in the database.
func QueryFromDB(ctx context.Context, bucket *gridfs.Bucket, url string) (FileDB, error) {
	f := FileDB{Name: url}
	// we don't need the content field
	opts := contentProjection[options.FindOneOptions](options.FindOne(), false)
	err := bucket.GetFilesCollection().FindOne(ctx, f, opts).Decode(&f)
	if err != nil {
		return f, errNotFound(err)
	}
	return f, nil
}

// ListFromDB returns a list of all files in the database.
func ListFromDB(ctx context.Context, bucket *gridfs.Bucket) (Files, error) {
	opts := contentProjection[options.FindOptions](options.Find(), false)
	cursor, err := bucket.GetFilesCollection().Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var files bson.A
	err = cursor.All(ctx, &files)
	if err != nil {
		return nil, err
	}
	bs, _ := json.Marshal(files)
	log.Println(string(bs))
	var f Files
	for _, file := range files {
		f = append(f, file.(FileDB))
	}
	return f, nil
}

// contentProjection is a map that maps the type of the options object to the
// projection that is used to query the content field.
func contentProjection[T any](opts opts[T], query bool) *T {
	s := "metadata.content"
	p := bson.M{s: 0}
	if query {
		p = bson.M{s: 1}
	}
	return opts.SetProjection(p)
}
