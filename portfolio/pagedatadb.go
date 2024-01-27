package main

import (
	"errors"
	"github.com/russross/blackfriday/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"log"
	"path"
	"time"
)

// PageDataDB is the representation of a page that is stored in the database
type PageDataDB struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name    string             `bson:"name,omitempty" json:"name,omitempty"`
	Content primitive.Binary   `bson:"content,omitempty" json:"content,omitempty"`
	LastMod time.Time          `bson:"last_mod,omitempty" json:"last_mod,omitempty"`
}

// TrimExt trims the extension from the page's name
func (p *PageDataDB) TrimExt() {
	p.Name = p.Name[:len(p.Name)-len(path.Ext(p.Name))]
}

// WriteToDB writes the page to the database; if the page already exists, it is
// updated, otherwise, it is inserted
func (p *PageDataDB) WriteToDB() error {
	log.Println("Writing page to database:", p.Name)
	// set options to either insert or update the page
	opts := options.Update().SetUpsert(true)
	// set update to update the page's content and last modification date
	update := bson.M{
		"$set": bson.M{
			"content":  p.Content,
			"last_mod": p.LastMod,
		},
	}
	// update the page in the database; use the page's name as filter, ensuring
	// that there is only one page with the same name
	res, err := pageCol.UpdateOne(ctx, bson.M{"name": p.Name}, update, opts)
	if err != nil {
		return err
	}
	// check result
	if res.MatchedCount == 1 {
		log.Println("Updated page:", p.Name)
	} else {
		log.Println("Inserted page:", p.Name)
		p.ID = res.UpsertedID.(primitive.ObjectID)
	}
	return nil
}

// Delete deletes the page from the database
func (p *PageDataDB) Delete() error {
	log.Println("Deleting page from database:", p.Name)
	return pageCol.FindOneAndDelete(ctx, p).Err()
}

// ToPage reads the page data from the database and parses it to a Page instance
func (p *PageDataDB) ToPage() (Page, error) {
	// to get the data from the database, either the ID or the name must be set
	if p.ID.IsZero() && p.Name == "" {
		return Page{}, errors.New("invalid page data")
	}
	// read page data from database
	err := pageCol.FindOne(ctx, p).Decode(p)
	if err != nil {
		return Page{}, err
	}
	// parse page data
	return Page{
		Title:   p.Name,
		Content: template.HTML(blackfriday.Run(p.Content.Data)),
		LastMod: p.LastMod,
		Year:    time.Now().Year(),
	}, nil
}

// listAllPages lists all pages in the database except for PageDataDB.Content
func listAllPages() ([]PageDataDB, error) {
	opts := options.Find().SetProjection(bson.M{"content": 0})
	cursor, err := pageCol.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var pages []PageDataDB
	err = cursor.All(ctx, &pages)
	if err != nil {
		return nil, err
	}
	return pages, nil
}
