package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	elastic "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

const (
	DataDir      = "data/"
	ElasticIndex = "commits"
	ElasticURL   = "http://localhost:9200"
)

type Commit struct {
	Repository    string    `json:"repository"`
	When          time.Time `json:"committer_when"`
	Hash          string    `json:"hash"`
	Committer     string    `json:"committer_name"`
	CommiterEmail string    `json:"committer_email"`
	Message       string    `json:"message"`
}

func main() {
	files, _ := filepath.Glob(DataDir + "*.json")
	if len(files) < 1 {
		log.Fatal("No JSON files found in", DataDir)
	}
	ctx := context.Background()
	client, err := elastic.NewClient(
		elastic.SetURL(ElasticURL),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(false),
	)
	if err != nil {
		log.Fatal(err)
	}
	var (
		wg          sync.WaitGroup
		commitCount int
	)
	start := time.Now()
	for _, f := range files {
		wg.Add(1)
		go func(f string) {
			file, err := ioutil.ReadFile(f)
			if err != nil {
				log.Fatal(err)
			}
			var commits []Commit
			err = json.Unmarshal(file, &commits)
			if err != nil {
				log.Fatal(err)
			}
			repo := strings.TrimPrefix(strings.TrimSuffix(f, filepath.Ext(f)), DataDir)
			for i, c := range commits {
				c.Repository = repo
				_, err = client.Index().
					Index(ElasticIndex).
					BodyJson(c).
					Do(ctx)
				if err != nil {
					log.Fatal(err)
				}
				progress := fmt.Sprintf("[%d/%d] [%s]", i+1, len(commits), c.Repository)
				log.Printf("%s Imported commit %s to index\n", progress, c.Hash[0:7])
				commitCount++
			}
			wg.Done()
		}(f)
	}
	wg.Wait()
	defer func() {
		log.Printf("Imported %d repositories (%d commits) in %s\n", len(files), commitCount, time.Since(start).Round(time.Millisecond))
	}()
}
