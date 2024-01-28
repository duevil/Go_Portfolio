package files

import (
	"bytes"
	"context"
	"errors"
	"github.com/russross/blackfriday/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"io"
	"log"
	"os"
	"path"
	"time"
)

var (
	Context context.Context
	col     *mongo.Collection
)

// maxFileSize is the maximum size for file to be stored in the database
const maxFileSize = 15 << 20 // 15 MiB
// URIRoot is the uri root for files
const URIRoot = "files"

var ErrNotFound = errors.Join(mongo.ErrNoDocuments, errors.New("file not found"))

// MongoFile is the representation of a file that is stored in the database
//
//goland:noinspection GoVetStructTag
type MongoFile struct {
	URI      string           `bson:"uri,omitempty" json:"uri,omitempty"`
	Filesize int64            `bson:"size,omitempty" json:"size,omitempty"`
	LastMod  time.Time        `bson:"last_mod,omitempty" json:"last_mod,omitempty"`
	Content  primitive.Binary `bson:"content,omitempty" json:"-"`
	IsMD     bool             `bson:"is_md,omitempty" json:"-"`
	IsLocal  bool             `bson:"is_local,omitempty" json:"-"`
	Mime     string           `bson:"mimetype,omitempty" json:"mimetype,omitempty"`
}

// Store reads the file's content from the given reader, stores it depending
// on its size and writes the file's metadata to the database.
//
// If the file's size is greater than maxFileSize, the file's content is stored
// on the file system and the file's IsLocal field is set to true. Otherwise,
// the file's content is stored in the database and the file's IsLocal field is
// set to false.
//
// If the file already exists in the database, the previous file is overwritten.
//
// Assumes that the file's URI and Filesize fields are set and returns an error
// otherwise.
func (p *MongoFile) Store(reader io.Reader) error {
	// check fields
	if p.URI == "" || p.Filesize == 0 {
		return errors.New("file's Filesize, URI or LastMod field is not set")
	}
	if p.Filesize > maxFileSize {
		log.Println("File is to big; contents will be stored on file system:", p.URI)
		// we must ensure that the file's directory exists
		err := os.MkdirAll(path.Join(URIRoot, path.Dir(p.URI)), os.ModePerm)
		if err != nil {
			return err
		}
		// create the file
		f, err := os.Create(path.Join(URIRoot, p.URI))
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		// write the file's content
		_, err = io.Copy(f, reader)
		if err != nil {
			return err
		}
		p.IsLocal = true
	} else {
		log.Println("File is small enough; contents will be stored in database:", p.URI)
		// read the file's content
		buf := bytes.Buffer{}
		_, err := io.Copy(&buf, reader)
		if err != nil {
			return err
		}
		p.Content = primitive.Binary{Data: buf.Bytes()}
		p.IsLocal = false
	}
	log.Println("Writing file to database:", p.URI)
	// set options to either insert or update the file
	opts := options.Update().SetUpsert(true)
	// update the file in the database
	res, err := col.UpdateOne(Context, bson.M{"name": p.URI}, bson.M{"$set": p}, opts)
	if err != nil {
		return err
	}
	// check result
	if res.MatchedCount == 1 {
		log.Println("Updated file:", p.URI)
	} else {
		log.Println("Inserted file:", p.URI)
	}
	return nil
}

// Open returns a reader for the file's content. If the file is stored locally,
// the file's content is read from the file system. Otherwise, the file's
// content is read from the database and a bytes.Reader is returned.
func (p *MongoFile) Open() (io.ReadCloser, error) {
	if p.IsLocal {
		log.Println("Opening file from file system:", p.URI)
		return os.Open(path.Join(URIRoot, p.URI))
	}
	log.Println("Opening file from database:", p.URI)
	opts := options.FindOne().SetProjection(bson.M{"content": 1})
	err := col.FindOne(Context, bson.M{"uri": p.URI}, opts).Decode(p)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(p.Content.Data)), nil
}

// ToPage parses the file's content as markdown and returns a Page. Returns an
// error if the file was not flagged to be markdown. The file is fully read from
// the database. If the file is stored locally, the file's content is read from
// the file system.
func (p *MongoFile) ToPage() (Page, error) {
	log.Println("Parsing file:", p.URI)
	if !p.IsMD {
		return Page{}, errors.New("file is not a markdown file")
	}
	err := col.FindOne(Context, bson.M{"uri": p.URI}).Decode(p)
	if err != nil {
		return Page{}, err
	}
	if p.IsLocal {
		log.Println("Reading file content from file system:", p.URI)
		f, err := os.Open(path.Join(URIRoot, p.URI))
		if err != nil {
			return Page{}, err
		}
		defer func() { _ = f.Close() }()
		buf := bytes.Buffer{}
		_, err = io.Copy(&buf, f)
		if err != nil {
			return Page{}, err
		}
		p.Content = primitive.Binary{Data: buf.Bytes()}
	}
	return Page{
		Title:   p.URI,
		Content: template.HTML(blackfriday.Run(p.Content.Data)),
		LastMod: p.LastMod,
		Year:    time.Now().Year(),
	}, nil
}

// Delete deletes the file from the database and file system if it exists
func (p *MongoFile) Delete() error {
	log.Println("Deleting file from database:", p.URI)
	// we only need to know whether the file is local
	opts := options.FindOneAndDelete().SetProjection(bson.M{"is_local": 1, "uri": 1})
	err := col.FindOneAndDelete(Context, bson.M{"uri": p.URI}, opts).Decode(p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return err
	}
	// delete file from file system if it exists
	if p.IsLocal {
		log.Println("Deleting file from file system:", p.URI)
		err := os.Remove(path.Join(URIRoot, p.URI))
		if err != nil {
			return err
		}
	}
	return nil
}

/* Methods for implementing the os.FileInfo interface */

// Name returns the file's uri or, if the file is a markdown file, the file's
// uri with the extension changed to ".html"
func (p *MongoFile) Name() string {
	if p.IsMD {
		return p.URI[:len(p.URI)-len(path.Ext(p.URI))] + ".html"
	}
	return p.URI
}
func (p *MongoFile) Size() int64        { return p.Filesize }
func (p *MongoFile) Mode() os.FileMode  { return os.ModePerm }
func (p *MongoFile) ModTime() time.Time { return p.LastMod }
func (p *MongoFile) IsDir() bool        { return false }
func (p *MongoFile) Sys() interface{}   { return nil }

// GetFromDB returns the file with the given uri from the database. The file's
// content is not read.
func GetFromDB(uri string) (MongoFile, error) {
	log.Println("Getting file from database:", uri)
	var file MongoFile
	opts := options.FindOne().SetProjection(bson.M{"content": 0})
	err := col.FindOne(Context, bson.M{"uri": uri}, opts).Decode(&file)
	// if the file is not found and the file is a html file, we search for the file
	// as a markdown file
	if errors.Is(ErrNotFound, err) && path.Ext(uri) == ".html" {
		uri = uri[:len(uri)-len(path.Ext(uri))] + ".md"
		err = col.FindOne(Context, bson.M{"uri": uri}, opts).Decode(&file)
		if err != nil {
			return MongoFile{}, err
		}
		file.URI = uri
		file.IsMD = true
	}
	if err != nil {
		return MongoFile{}, err
	}
	return file, nil
}

// ListAll lists all files in the database except for MongoFile.Content
func ListAll() ([]MongoFile, error) {
	opts := options.Find().SetProjection(bson.M{"content": 0})
	cursor, err := col.Find(Context, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var files []MongoFile
	err = cursor.All(Context, &files)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func SetCollection(c *mongo.Collection) { col = c }
