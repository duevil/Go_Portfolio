package files

import (
	"encoding/json"
	"path"
	"strings"
)

// FileType constants represent the type of the asset the Config object refers
// to.
const (
	Asset     FileType = iota + 1 // any file to be stored in the GridFS file system
	Markdown                      // a markdown file to be rendered as html
	Static                        // any file to be served statically (e.g. html, small images, css, js)
	Templated                     // a template file to be rendered as html or used to render markdown
)

// FileType represents the type of the asset the Config object refers to.
type FileType int

// MarshalBSON marshals the FileType object into BSON data using MarshalJSON.
func (ft *FileType) MarshalBSON() ([]byte, error) { return ft.MarshalJSON() }

// UnmarshalBSON unmarshals the BSON data into the FileType object using
// UnmarshalJSON.
func (ft *FileType) UnmarshalBSON(b []byte) error { return ft.UnmarshalJSON(b) }

// MarshalJSON marshals the FileType object into JSON data. Sets the FileType to
// its string representation.
func (ft *FileType) MarshalJSON() ([]byte, error) { return json.Marshal(ft.String()) }

// UnmarshalJSON unmarshals the JSON data into the FileType object. Parses the
// FileType from the string representation.
func (ft *FileType) UnmarshalJSON(b []byte) error {
	var tmp string
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	ft.fromString(tmp)
	return nil
}

// String returns the string representation of the FileType.
func (ft *FileType) String() string {
	switch *ft {
	case Asset:
		return "asset"
	case Markdown:
		return "markdown"
	case Static:
		return "static"
	case Templated:
		return "templated"
	default:
		return ""
	}
}

// FromFile sets the FileType based on the file extension.
func (ft *FileType) FromFile(f string) {
	switch strings.ToLower(path.Ext(f)) {
	case ".md":
		*ft = Markdown
	case ".gohtml":
		*ft = Templated
	// images like png, jpg, gif, etc. are assumed to be assets and thus not listed here
	case ".html", ".htm", ".css", ".js", ".svg", ".ico", ".webp":
		*ft = Static
	default:
		*ft = Asset
	}
}

// fromString sets the FileType to its constant representation.
func (ft *FileType) fromString(s string) {
	switch s {
	case "asset":
		*ft = Asset
	case "markdown":
		*ft = Markdown
	case "static":
		*ft = Static
	case "templated":
		*ft = Templated
	default:
	}
}
