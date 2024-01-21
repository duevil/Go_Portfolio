package files

import (
	"context"
	"encoding/json"
	"errors"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"os"
	"path"
	"path/filepath"
)

const ConfigFile = "config.json"

// ProcessDir processes all files in the given directory. Tries to read the
// ConfigFile in the directory and uses it to process the files. Uses
// filepath.WalkDir to walk the directory and handle each file by processing it
// with Config.Process. If the file is not in the config, it is added with
// default values.
//
// Returns Configs that is ensured to contain a Config for each
// processed file in the directory.
func ProcessDir(ctx context.Context, bucket *gridfs.Bucket, dir string) (Configs, error) {
	// read config file if it exists
	var configs Configs
	cf, err := os.Open(path.Join(dir, ConfigFile))
	exists := !errors.Is(err, os.ErrNotExist)
	if !exists && err != nil {
		return nil, err
	}
	if exists {
		defer cf.Close()
		err = json.NewDecoder(cf).Decode(&configs)
		if err != nil {
			return nil, err
		}
	}
	// walk dir to handle each file
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		return processFile(dir, p, d, err, configs, ctx, bucket)
	})
	return configs, err
}

// ProcessSingle processes a single file. A Config is created with default
// values and the file is processed with Config.Process.
func ProcessSingle(ctx context.Context, bucket *gridfs.Bucket, file *os.File) (Config, error) {
	// create and set config
	config := Config{
		Title: path.Base(file.Name()),
		Path:  file.Name(),
	}
	config.FileType.FromFile(path.Ext(file.Name()))
	config.AdjustURL()
	// process file
	return config, config.process(file, ctx, bucket)
}

// processFile is a helper function for ProcessDir that processes a single file.
// The file is processed with Config.Process. If the file is not in the config,
// it is added with default values.
func processFile(
	dir, p string,
	d os.DirEntry,
	err error,
	configs Configs,
	ctx context.Context,
	bucket *gridfs.Bucket,
) error {
	if err != nil {
		return err
	}
	if d.IsDir() {
		return nil
	}
	// check if file is in config for this, the path needs to be relative to the dir,
	// so we remove the dir from the path
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return err
	}
	// open file
	file, err := os.Open(p)
	if err != nil {
		return err
	}
	defer file.Close()
	// get config; if not in config, add it with default values
	config, ok := configs[rel]
	if !ok {
		c, err := ProcessSingle(ctx, bucket, file)
		if err != nil {
			configs[rel] = c
		}
		return err
	}
	return config.process(file, ctx, bucket)
}

// process processes the file based on its FileType, i.e:
//   - Asset & Markdown: upload to database using UploadToDB, delete afterward
//   - Static: upload to database using UploadToDB and move to StaticDir
//   - Templated: move to TemplateDir
func (c *Config) process(file *os.File, ctx context.Context, bucket *gridfs.Bucket) error {
	switch c.FileType {
	case Asset, Markdown:
		fi, err := file.Stat()
		if err != nil {
			return err
		}
		err = UploadToDB(ctx, bucket, *c, file, fi)
		if err != nil {
			return err
		}
		err = os.Remove(file.Name()) // delete file after upload
		return err
	case Static:
		fi, err := file.Stat()
		if err != nil {
			return err
		}
		err = UploadToDB(ctx, bucket, *c, file, fi)
		if err != nil {
			return err
		}
		fallthrough // static files are moved the same way as templated files
	case Templated:
		// move file to dir corresponding to URL
		err := os.MkdirAll(path.Dir(c.URL), os.ModePerm)
		if err != nil {
			return err
		}
		return os.Rename(file.Name(), c.URL)
	}
	return nil
}
