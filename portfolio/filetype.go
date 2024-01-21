package main

import (
	"path"
	"strings"
)

type Filetype int

const (
	// ZIP is used for zip files containing other files
	ZIP Filetype = iota
	// Asset is used for generic files that stored in the database
	Asset
	// Markdown is used for markdown files that stored in the database and are
	// converted to HTML using blackfriday.Run()
	Markdown
	// Static is used for files stored in the static directory
	Static
	// Templated is used for files stored in the templates directory
	Templated
)

// filetypeFromFile returns the filetype of a file based on its extension, i.e:
//   - .zip: ZIP
//   - .md: Markdown
//   - .html, .css, .js, .ico, .svg: Static
//   - .gohtml, .tmpl: Templated
//   - else: Asset
func filetypeFromFile(f string) Filetype {
	switch strings.ToLower(path.Ext(f)) {
	case ".zip":
		return ZIP
	case ".md":
		return Markdown
	case ".html", ".css", ".js", ".ico", ".svg":
		return Static
	case ".gohtml", ".tmpl":
		return Templated
	default:
		return Asset
	}
}

/*
// MarshalJSON converts the Filetype to a JSON string
func (ft *Filetype) MarshalJSON() ([]byte, error) {
	switch *ft {
	case ZIP:
		return json.Marshal("zip")
	case Asset:
		return json.Marshal("asset")
	case Markdown:
		return json.Marshal("markdown")
	case Static:
		return json.Marshal("static")
	case Templated:
		return json.Marshal("templated")
	default:
		return nil, errors.New("invalid Filetype")
	}
}

// UnmarshalJSON converts a JSON string to a Filetype
func (ft *Filetype) UnmarshalJSON(b []byte) error {
	var tmp string
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	switch tmp {
	case "zip":
		*ft = ZIP
	case "asset":
		*ft = Asset
	case "markdown":
		*ft = Markdown
	case "static":
		*ft = Static
	case "templated":
		*ft = Templated
	default:
		return errors.New("invalid Filetype")
	}
	return nil
}

// MarshalBSON converts the Filetype to a BSON string
func (ft *Filetype) MarshalBSON() ([]byte, error) { return ft.MarshalJSON() }

// UnmarshalBSON converts a BSON string to a Filetype
func (ft *Filetype) UnmarshalBSON(b []byte) error { return ft.UnmarshalJSON(b) }
*/
