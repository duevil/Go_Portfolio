package files

import (
	"errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

// ErrNotFound is returned when a file was not found in the database.
var ErrNotFound = errors.New("file not found")

// errNotFound checks whether the given error is a mongo.ErrNoDocuments or a
// gridfs.ErrFileNotFound and returns ErrNotFound joined with the given
// error. Otherwise, the given error is returned unchanged.
func errNotFound(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.Join(ErrNotFound, err)
	}
	if errors.Is(err, gridfs.ErrFileNotFound) {
		return errors.Join(ErrNotFound, err)
	}
	return err
}
