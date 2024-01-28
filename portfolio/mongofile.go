package main

import (
	"bytes"
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
	pathLib "path"
	"time"
)

// MaxFileSize is the maximum size for file to be stored in the database
const MaxFileSize = 15 << 20 // 15 MiB
// FilePathRoot is the uri root for files
const FilePathRoot = "files"

// ErrNotMD is returned by MongoFile.ToPage if the file is not a markdown file
var ErrNotMD = errors.New("file is not a markdown file")
var ErrNotFound = errors.Join(mongo.ErrNoDocuments, errors.New("file not found"))

// MongoFile is the representation of a file that is stored in the database
//
//goland:noinspection GoVetStructTag
type MongoFile struct {
	id       primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Path     string             `bson:"path,omitempty" json:"path,omitempty"`
	Filesize int64              `bson:"size,omitempty" json:"size,omitempty"`
	LastMod  time.Time          `bson:"last_mod,omitempty" json:"last_mod,omitempty"`
	Content  primitive.Binary   `bson:"content,omitempty" json:"-"`
	IsMD     bool               `bson:"is_md,omitempty" json:"-"`
	IsLocal  bool               `bson:"is_local,omitempty" json:"-"`
	Mime     string             `bson:"mime,omitempty" json:"mime,omitempty"`
}

// Store reads the file's content from the given reader, stores it depending
// on its size and writes the file's metadata to the database.
//
// If the file's size is greater than MaxFileSize, the file's content is stored
// on the file system and the file's IsLocal field is set to true. Otherwise,
// the file's content is stored in the database and the file's IsLocal field is
// set to false.
//
// If the file already exists in the database, the previous file is overwritten.
//
// Assumes that the file's Path and Filesize fields are set and returns an error
// otherwise.
func (p *MongoFile) Store(reader io.Reader) error {
	// check fields
	if p.Path == "" || p.Filesize == 0 {
		return errors.New("file's Filesize, Path or LastMod field is not set")
	}
	if p.Filesize > MaxFileSize {
		log.Println("File is to big; contents will be stored on file system:", p.Path)
		// we must ensure that the file's directory exists
		err := os.MkdirAll(pathLib.Join(FilePathRoot, pathLib.Dir(p.Path)), os.ModePerm)
		if err != nil {
			return err
		}
		// create the file
		f, err := os.Create(pathLib.Join(FilePathRoot, p.Path))
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
		log.Println("File is small enough; contents will be stored in database:", p.Path)
		// read the file's content
		buf := bytes.Buffer{}
		_, err := io.Copy(&buf, reader)
		if err != nil {
			return err
		}
		p.Content = primitive.Binary{Data: buf.Bytes()}
		p.IsLocal = false
	}
	log.Println("Writing file to database:", p.Path)
	// set options to either insert or update the file
	opts := options.Update().SetUpsert(true)
	// update the file in the database
	res, err := fileCol.UpdateOne(ctx, bson.M{"name": p.Path}, bson.M{"$set": p}, opts)
	if err != nil {
		return err
	}
	// check result
	if res.MatchedCount == 1 {
		log.Println("Updated file:", p.Path)
	} else {
		log.Println("Inserted file:", p.Path)
		p.id = res.UpsertedID.(primitive.ObjectID)
	}
	return nil
}

// Open returns a reader for the file's content. If the file is stored locally,
// the file's content is read from the file system. Otherwise, the file's
// content is read from the database and a bytes.Reader is returned.
func (p *MongoFile) Open() (io.ReadCloser, error) {
	if p.IsLocal {
		log.Println("Opening file from file system:", p.Path)
		return os.Open(pathLib.Join(FilePathRoot, p.Path))
	}
	log.Println("Opening file from database:", p.Path)
	opts := options.FindOne().SetProjection(bson.M{"content": 1})
	err := fileCol.FindOne(ctx, bson.M{"path": p.Path}, opts).Decode(p)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(p.Content.Data)), nil
}

// ToPage parses the file's content as markdown and returns a Page. If the file
// is not a markdown file, ErrNotMD is returned. The file is fully read from the
// database. If the file is stored locally, the file's content is read from the
// file system.
func (p *MongoFile) ToPage() (Page, error) {
	log.Println("Parsing page:", p.Path)
	if !p.IsMD {
		return Page{}, ErrNotMD
	}
	err := fileCol.FindOne(ctx, bson.M{"path": p.Path}).Decode(p)
	if err != nil {
		return Page{}, err
	}
	if p.IsLocal {
		log.Println("Reading page content from file system:", p.Path)
		f, err := os.Open(pathLib.Join(FilePathRoot, p.Path))
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
		Title:   p.Path,
		Content: template.HTML(blackfriday.Run(p.Content.Data)),
		LastMod: p.LastMod,
		Year:    time.Now().Year(),
	}, nil
}

// Delete deletes the file from the database and file system if it exists
func (p *MongoFile) Delete() error {
	log.Println("Deleting file from database:", p.Path)
	// we only need to know whether the file is local
	opts := options.FindOneAndDelete().SetProjection(bson.M{"is_local": 1, "path": 1})
	err := fileCol.FindOneAndDelete(ctx, bson.M{"path": p.Path}, opts).Decode(p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return err
	}
	// delete file from file system if it exists
	if p.IsLocal {
		log.Println("Deleting file from file system:", p.Path)
		err := os.Remove(pathLib.Join(FilePathRoot, p.Path))
		if err != nil {
			return err
		}
	}
	return nil
}

/* Methods for implementing the os.FileInfo interface */

// Name returns the file's path or, if the file is a markdown file, the file's
// path without the extension changed to ".html"
func (p *MongoFile) Name() string {
	if p.IsMD {
		return p.Path[:len(p.Path)-len(pathLib.Ext(p.Path))] + ".html"
	}
	return p.Path
}
func (p *MongoFile) Size() int64        { return p.Filesize }
func (p *MongoFile) Mode() os.FileMode  { return os.ModePerm }
func (p *MongoFile) ModTime() time.Time { return p.LastMod }
func (p *MongoFile) IsDir() bool        { return false }
func (p *MongoFile) Sys() interface{}   { return nil }

// GetFromDB returns the file with the given path from the database. The file's
// content is not read.
func GetFromDB(path string) (MongoFile, error) {
	var file MongoFile
	opts := options.FindOne().SetProjection(bson.M{"content": 0})
	err := fileCol.FindOne(ctx, bson.M{"path": path}, opts).Decode(&file)
	// if the file is not found and the file is a html file, we search for the file
	// as a markdown file
	if errors.Is(ErrNotFound, err) && pathLib.Ext(path) == ".html" {
		path = path[:len(path)-len(pathLib.Ext(path))] + ".md"
		err = fileCol.FindOne(ctx, bson.M{"path": path}, opts).Decode(&file)
		if err != nil {
			return MongoFile{}, err
		}
		file.Path = path
		file.IsMD = true
	}
	if err != nil {
		return MongoFile{}, err
	}
	return file, nil
}

// ListAllFiles lists all files in the database except for MongoFile.Content
func ListAllFiles() ([]MongoFile, error) {
	opts := options.Find().SetProjection(bson.M{"content": 0})
	cursor, err := fileCol.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var files []MongoFile
	err = cursor.All(ctx, &files)
	if err != nil {
		return nil, err
	}
	return files, nil
}
