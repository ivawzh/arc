package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	client    *elastic.Client
}

func newClient(url, indexName, config string) (*elasticsearch, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
	}
	ctx := context.Background()

	// Initialize the client
	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic client: %v", err)
	}
	es := &elasticsearch{url, indexName, client}

	// Check if meta index already exists
	exists, err := client.IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named \"%s\" already exists, skipping ...\n", logTag, indexName)
		return es, nil
	}

	// Meta index doesn't exist, create one
	_, err = client.CreateIndex(indexName).
		Body(config).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named \"%s\"", indexName)
	}

	log.Printf("%s: successfully created index name \"%s\"", logTag, indexName)
	return es, nil
}

func (es *elasticsearch) indexRecord(record record) {
	_, err := es.client.
		Index().
		Index(es.indexName).
		Type("_doc").
		BodyJson(record).
		Do(context.Background())
	if err != nil {
		log.Printf("%s: error indexing logs record: %v", logTag, err)
		return
	}
}

func (es *elasticsearch) getRawLogs(from, size string, indices ...string) ([]byte, error) {
	offset, err := strconv.Atoi(from)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "from"`, from)
	}
	s, err := strconv.Atoi(size)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "size"`, size)
	}

	response, err := es.client.Search(es.indexName).
		From(offset).
		Size(s).
		Sort("timestamp", false).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	hits := []*elastic.SearchHit{}
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(*hit.Source, &source)
		if err != nil {
			return nil, err
		}
		rawIndices, ok := source["indices"]
		if !ok {
			log.Printf(`%s: unable to find "indices" in log record\n`, logTag)
		}
		logIndices, err := util.ToStringSlice(rawIndices)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			continue
		}

		if len(indices) == 0 {
			hits = append(hits, hit)
		} else if util.IsSubset(indices, logIndices) {
			hits = append(hits, hit)
		}
	}

	logs := make(map[string]interface{})
	logs["logs"] = hits
	logs["total"] = len(hits)
	logs["took"] = response.TookInMillis

	raw, err := json.Marshal(logs)
	if err != nil {
		return nil, err
	}

	return raw, nil
}