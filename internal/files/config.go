package files

import (
	"encoding/json"
	"path"
	"strings"
)

// Constants for the static and template directories.
const (
	StaticDir   = "/static"    // StaticDir is the directory used to store static files.
	TemplateDir = "/templates" // TemplateDir is the directory used to store template files.
)

// Constants for the url paths corresponding to the FileType.
const (
	AssetURL = "/asset" // AssetURL is the url path used to serve assets stored in the GridFS file system.
	PageURL  = "/page"  // PageURL is the url path used to serve pages rendered from Markdown files.

	// StaticURL is the url path used to serve static files (for convenience, equivalent to StaticDir).
	StaticURL = StaticDir
	// TemplateURL is the url path used to serve template files (for convenience, equivalent to TemplateDir).
	TemplateURL = TemplateDir
)

// Config contains the configuration for an asset.
type Config struct {
	Path          string   `json:"file"`                 // the file path of the asset
	Title         string   `json:"title"`                // title of the asset
	URL           string   `json:"url,omitempty"`        // the url path the asset is served at
	PredefinedURL bool     `json:"custom_url,omitempty"` // whether the URL was predefined or uses default values
	FileType      FileType `json:"type,omitempty"`       // the FileType of the asset
}

// Configs is a map of file paths to Config objects. It is used to simplify the
// appliance of the Config to the file by using the file path to find the
// corresponding Config.
type Configs map[string]Config

// UnmarshalBSON unmarshals the BSON data into the Config object using
// UnmarshalJSON.
func (c *Config) UnmarshalBSON(b []byte) error { return json.Unmarshal(b, c) }

// UnmarshalJSON unmarshals the JSON data into the Config object. If the Title
// is not set, it is set to the base name of the file. If the FileType is not
// set, it is parsed from the string representation. The URL is adjusted using
// AdjustURL.
func (c *Config) UnmarshalJSON(b []byte) error {
	type config Config // prevent recursion
	var tmp config
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	*c = Config(tmp)
	if c.Title == "" {
		c.Title = path.Base(c.Path)
	}
	if c.FileType == 0 {
		c.FileType.FromFile(c.Path)
	}
	c.AdjustURL()
	return nil
}

// AdjustURL sets the URL depending on the FileType and whether the URL is set,
// i.e. if the URL is not set, it is set to Config.Title in lowercase and is prepended
// with the appropriate URL prefix:
//   - AssetURL for Asset
//   - PageURL for Markdown
//   - StaticURL for Static
//   - TemplateURL for Templated
//
// If the URL is already set, it is left unchanged. The PredefinedURL field is set to reflect
// whether the URL was predefined or is set to use default values.
//
// # Example
//
//	Title = "My Page"
//	FileType = Markdown
//	URL = ""
//	AdjustURL() => URL = "/page/my-page"
//
//	Title = "My Page"
//	FileType = Asset
//	URL = "/path/to/my-page"
//	AdjustURL() => URL = "/path/to/my-page"
func (c *Config) AdjustURL() {
	if c.URL != "" {
		c.PredefinedURL = true
		return
	}
	c.URL = strings.ToLower(c.Title)
	switch c.FileType {
	case Asset:
		c.URL = path.Join(AssetURL, c.URL)
	case Markdown:
		c.URL = path.Join(PageURL, c.URL)
	case Static:
		c.URL = path.Join(StaticURL, c.URL)
	case Templated:
		c.URL = path.Join(TemplateURL, c.URL)
	}
	c.PredefinedURL = false
}

// UnmarshalJSON unmarshals the JSON data into the Configs object.
func (cs *Configs) UnmarshalJSON(b []byte) error {
	var tmp []Config
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	*cs = make(Configs)
	for _, c := range tmp {
		(*cs)[c.Path] = c
	}
	return nil
}
