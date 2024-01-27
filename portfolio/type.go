package main

import "path"

// Type is the type of files handled by the server
type Type int

const (
	// Static files are files that have the following characteristics:
	// 		- they are stored on the server's file system in the StatDir
	// 		- they are served to the client statically and as-is
	// 		- they are collected when downloading the portfolio
	// 		- they are listed when listing all files
	Static Type = iota
	// Template files are files that have the following characteristics:
	// 		- they are stored on the server's file system in the TmplDir
	// 		- they are used to create the HTML representation of pages
	// 		- they are served by parsing them without any data
	// 		- they are not collected when downloading the portfolio
	// 		- they are not listed when listing all files
	Template
	// Markdown files ar page files that have the following characteristics:
	// 		- they are stored in the database using PageDataDB
	// 		- they are used to create the HTML representation of pages using the page template
	// 		- they are served by parsing them with the data from the database
	// 		- they are collected when downloading the portfolio
	// 		- they are listed when listing all files
	Markdown
	// Zipped files are files only used to upload other files to the server
	Zipped
)

// MatchesExt returns whether the given file extension matches the type
func (t Type) MatchesExt(name string) bool {
	return map[string]Type{
		".gohtml": Template,
		".tmpl":   Template,
		".md":     Markdown,
		".zip":    Zipped,
	}[path.Ext(name)] == t
}
