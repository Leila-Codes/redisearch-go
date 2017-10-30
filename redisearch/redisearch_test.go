package redisearch_test

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/RedisLabs/redisearch-go/redisearch"
	"github.com/stretchr/testify/assert"
)

func createClient(indexName string) *redisearch.Client {
	value, exists := os.LookupEnv("REDISEARCH_TEST_HOST")
	host := "localhost:6379"
	if exists && value != "" {
		host = value
	}
	return redisearch.NewClient(host, indexName)
}

func TestClient(t *testing.T) {

	c := createClient("testung")

	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("foo"))
	c.Drop()
	if err := c.CreateIndex(sc); err != nil {
		t.Fatal(err)
	}

	docs := make([]redisearch.Document, 100)
	for i := 0; i < 100; i++ {
		docs[i] = redisearch.NewDocument(fmt.Sprintf("doc%d", i), float32(i)/float32(100)).Set("foo", "hello world")
	}

	if err := c.IndexOptions(redisearch.DefaultIndexingOptions, docs...); err != nil {
		t.Fatal(err)
	}

	// Test it again
	if err := c.IndexOptions(redisearch.DefaultIndexingOptions, docs...); err == nil {
		t.Fatal("Expected error for duplicate document")
	} else {
		if merr, ok := err.(redisearch.MultiError); !ok {
			t.Fatal("error not a multi error")
		} else {
			assert.Equal(t, 100, len(merr))
			assert.NotEmpty(t, merr)
			//fmt.Println("Got errors: ", merr)
		}
	}

	docs, total, err := c.Search(redisearch.NewQuery("hello world"))

	assert.Nil(t, err)
	assert.Equal(t, 100, total)
	assert.Equal(t, 10, len(docs))

	fmt.Println(docs, total, err)
}

func TestInfo(t *testing.T) {
	c := createClient("testung")

	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("foo")).
		AddField(redisearch.NewSortableNumericField("bar"))
	c.Drop()
	assert.Nil(t, c.CreateIndex(sc))

	info, err := c.Info()
	assert.Nil(t, err)
	fmt.Printf("%v\n", info)
}

func TestNumeric(t *testing.T) {
	c := createClient("testung")

	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("foo")).
		AddField(redisearch.NewSortableNumericField("bar"))
	c.Drop()
	assert.Nil(t, c.CreateIndex(sc))

	docs := make([]redisearch.Document, 100)
	for i := 0; i < 100; i++ {
		docs[i] = redisearch.NewDocument(fmt.Sprintf("doc%d", i), 1).Set("foo", "hello world").Set("bar", i)
	}

	assert.Nil(t, c.Index(docs...))

	docs, total, err := c.Search(redisearch.NewQuery("hello world @bar:[50 100]").SetFlags(redisearch.QueryNoContent | redisearch.QueryWithScores))
	assert.Nil(t, err)
	assert.Equal(t, 10, len(docs))
	assert.Equal(t, 50, total)

	docs, total, err = c.Search(redisearch.NewQuery("hello world @bar:[40 90]").SetSortBy("bar", false))
	assert.Nil(t, err)
	assert.Equal(t, 10, len(docs))
	assert.Equal(t, 51, total)
	assert.Equal(t, "doc90", docs[0].Id)
	assert.Equal(t, "doc89", docs[1].Id)
	assert.Equal(t, "doc81", docs[9].Id)

	docs, total, err = c.Search(redisearch.NewQuery("hello world @bar:[40 90]").
		SetSortBy("bar", true).
		SetReturnFields("foo"))
	assert.Nil(t, err)
	assert.Equal(t, 10, len(docs))
	assert.Equal(t, 51, total)
	assert.Equal(t, "doc40", docs[0].Id)
	assert.Equal(t, "hello world", docs[0].Properties["foo"])
	assert.Nil(t, docs[0].Properties["bar"])
	assert.Equal(t, "doc41", docs[1].Id)
	assert.Equal(t, "doc49", docs[9].Id)
	fmt.Println(docs)

	// Try "Explain"
	explain, err := c.Explain(redisearch.NewQuery("hello world @bar:[40 90]"))
	assert.Nil(t, err)
	assert.NotNil(t, explain)
	fmt.Println(explain)
}

func TestNoIndex(t *testing.T) {
	c := createClient("testung")
	c.Drop()

	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextFieldOptions("f1", redisearch.TextFieldOptions{Sortable: true, NoIndex: true, Weight: 1.0})).
		AddField(redisearch.NewTextField("f2"))

	err := c.CreateIndex(sc)
	assert.Nil(t, err)

	props := make(map[string]interface{})
	props["f1"] = "MarkZZ"
	props["f2"] = "MarkZZ"

	err = c.Index(redisearch.Document{Id: "doc1", Properties: props})
	assert.Nil(t, err)

	props["f1"] = "MarkAA"
	props["f2"] = "MarkAA"
	err = c.Index(redisearch.Document{Id: "doc2", Properties: props})
	assert.Nil(t, err)

	_, total, err := c.Search(redisearch.NewQuery("@f1:Mark*"))
	assert.Nil(t, err)
	assert.Equal(t, 0, total)

	_, total, err = c.Search(redisearch.NewQuery("@f2:Mark*"))
	assert.Equal(t, 2, total)

	docs, total, err := c.Search(redisearch.NewQuery("@f2:Mark*").SetSortBy("f1", false))
	assert.Equal(t, 2, total)
	assert.Equal(t, "doc1", docs[0].Id)

	docs, total, err = c.Search(redisearch.NewQuery("@f2:Mark*").SetSortBy("f2", true))
	assert.Equal(t, 2, total)
	assert.Equal(t, "doc2", docs[0].Id)
}

func TestHighlight(t *testing.T) {
	c := createClient("testung")

	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("foo")).
		AddField(redisearch.NewTextField("bar"))
	c.Drop()
	assert.Nil(t, c.CreateIndex(sc))

	docs := make([]redisearch.Document, 100)
	for i := 0; i < 100; i++ {
		docs[i] = redisearch.NewDocument(fmt.Sprintf("doc%d", i), 1).Set("foo", "hello world").Set("bar", "hello world foo bar baz")
	}
	c.Index(docs...)

	q := redisearch.NewQuery("hello").Highlight([]string{"foo"}, "[", "]")
	docs, _, err := c.Search(q)
	assert.Nil(t, err)

	assert.Equal(t, 10, len(docs))
	for _, d := range docs {
		assert.Equal(t, "[hello] world", d.Properties["foo"])
		assert.Equal(t, "hello world foo bar baz", d.Properties["bar"])
	}

	q = redisearch.NewQuery("hello world baz").Highlight([]string{"foo", "bar"}, "{", "}")
	docs, _, err = c.Search(q)
	assert.Nil(t, err)

	assert.Equal(t, 10, len(docs))
	for _, d := range docs {
		assert.Equal(t, "{hello} {world}", d.Properties["foo"])
		assert.Equal(t, "{hello} {world} foo bar {baz}", d.Properties["bar"])
	}

	// test RETURN contradicting HIGHLIGHT
	q = redisearch.NewQuery("hello").Highlight([]string{"foo"}, "[", "]").SetReturnFields("bar")
	docs, _, err = c.Search(q)
	assert.Nil(t, err)

	assert.Equal(t, 10, len(docs))
	for _, d := range docs {
		assert.Equal(t, nil, d.Properties["foo"])
		assert.Equal(t, "hello world foo bar baz", d.Properties["bar"])
	}

	c.Drop()
}

func TestSammurize(t *testing.T) {
	c := createClient("testung")

	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("foo")).
		AddField(redisearch.NewTextField("bar"))
	c.Drop()
	assert.Nil(t, c.CreateIndex(sc))

	docs := make([]redisearch.Document, 10)
	for i := 0; i < 10; i++ {
		docs[i] = redisearch.NewDocument(fmt.Sprintf("doc%d", i), 1).
			Set("foo", "There are two sub-commands commands used for highlighting. One is HIGHLIGHT which surrounds matching text with an open and/or close tag; and the other is SUMMARIZE which splits a field into contextual fragments surrounding the found terms. It is possible to summarize a field, highlight a field, or perform both actions in the same query.").Set("bar", "hello world foo bar baz")
	}
	c.Index(docs...)

	q := redisearch.NewQuery("commands fragments fields").Summarize("foo")
	docs, _, err := c.Search(q)
	assert.Nil(t, err)

	assert.Equal(t, 10, len(docs))
	for _, d := range docs {
		assert.Equal(t, "sub-commands commands used... field into contextual fragments surrounding the found terms. It is possible to summarize a field, highlight a field... ", d.Properties["foo"])
		assert.Equal(t, "hello world foo bar baz", d.Properties["bar"])
	}

	q = redisearch.NewQuery("commands fragments fields").
		Highlight([]string{"foo"}, "[", "]").
		SummarizeOptions(redisearch.SummaryOptions{
			Fields:       []string{"foo"},
			Separator:    "\r\n",
			FragmentLen:  10,
			NumFragments: 5},
		)
	docs, _, err = c.Search(q)
	assert.Nil(t, err)

	assert.Equal(t, 10, len(docs))
	for _, d := range docs {
		assert.Equal(t, "are two sub-[commands] [commands] used for highlighting. One is\r\na [field] into contextual [fragments] surrounding the found terms. It is possible to summarize a [field], highlight a [field], or\r\n", d.Properties["foo"])
		assert.Equal(t, "hello world foo bar baz", d.Properties["bar"])
	}
}

func ExampleClient() {

	// Create a client. By default a client is schemaless
	// unless a schema is provided when creating the index
	c := createClient("myIndex")

	// Create a schema
	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("body")).
		AddField(redisearch.NewTextFieldOptions("title", redisearch.TextFieldOptions{Weight: 5.0, Sortable: true})).
		AddField(redisearch.NewNumericField("date"))

	// Drop an existing index. If the index does not exist an error is returned
	c.Drop()

	// Create the index with the given schema
	if err := c.CreateIndex(sc); err != nil {
		log.Fatal(err)
	}

	// Create a document with an id and given score
	doc := redisearch.NewDocument("doc1", 1.0)
	doc.Set("title", "Hello world").
		Set("body", "foo bar").
		Set("date", time.Now().Unix())

	// Index the document. The API accepts multiple documents at a time
	if err := c.IndexOptions(redisearch.DefaultIndexingOptions, doc); err != nil {
		log.Fatal(err)
	}

	// Searching with limit and sorting
	docs, total, err := c.Search(redisearch.NewQuery("hello world").
		Limit(0, 2).
		SetReturnFields("title"))

	fmt.Println(docs[0].Id, docs[0].Properties["title"], total, err)
	// Output: doc1 Hello world 1 <nil>
}
