package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"
)

type URL struct {
	Value string `json:"url"`
}

type CreateSlugURLPairResponse struct {
	Result struct {
		InsertedID string `json:"InsertedID"`
	} `json:"result"`
	SlugURLPair struct {
		Slug     string `json:"Slug"`
		Url      string `json:"Url"`
		ExpireAt string `json:"ExpireAt"`
	} `json:"slugURLPair"`
}

type URLPair struct {
	OriginalUrl string `json:"originalUrl"`
	NewUrl      string `json:"newUrl"`
}

const CREATE_SLUG_URL_PAIR_ENDPOINT string = "https://dlgfy.xyz/createSlugURLPair"
const REDIRECT_ENDPOINT string = "https://dlgfy.xyz"

func makeOutput(slug string) string {
	return REDIRECT_ENDPOINT + "/" + slug
}

func standardOutput(outputSlice []string) string {
	return strings.Join(outputSlice, "\n")
}

func jsonOutput(originalURLs []string, shortenedURLs []string) (string, error) {
	urlPairs := make([]URLPair, 0, len(originalURLs))
	for i := 0; i < len(originalURLs); i++ {
		urlPair := URLPair{
			OriginalUrl: originalURLs[i],
			NewUrl:      shortenedURLs[i],
		}
		urlPairs = append(urlPairs, urlPair)
	}
	if jsonData, err := json.MarshalIndent(urlPairs, "", "  "); err != nil {
		return "", err
	} else {
		return string(jsonData), nil
	}
}

func dumpStringToFile(str string, path string) error {
	output := []byte(str)
	return os.WriteFile(path, output, 0644)
}

func defaultAction(ctx *cli.Context) {
	if ctx.Args().Len() > 0 {
		bulkConvert(ctx)
	} else {
		cli.ShowAppHelpAndExit(ctx, 0)
	}
}

func output(ctx *cli.Context, originalURLs []string, shortenedURLs []string) error {
	var output string
	var err error
	if ctx.Bool("json") {
		output, err = jsonOutput(originalURLs, shortenedURLs)
		if err != nil {
			return err
		}
	} else {
		output = standardOutput(shortenedURLs)
	}
	if outputPath := ctx.String("output"); outputPath != "" {
		dumpStringToFile(output, outputPath)
	} else {
		fmt.Fprintln(os.Stdout, output)
	}
	return nil
}

func bulkConvert(ctx *cli.Context) {
	urls := ctx.Args().Slice()
	var mu sync.Mutex
	var wg sync.WaitGroup
	shortenedURLs := make([]string, len(urls))
	for i, url := range urls {
		wg.Add(1)
		go func(url string, index int) {
			defer wg.Done()
			requestBody := URL{Value: url}
			jsonBody, err := json.Marshal(requestBody)
			if err != nil {
				log.Fatal(err)
			}
			resp, err := http.Post(CREATE_SLUG_URL_PAIR_ENDPOINT, "application/json", bytes.NewBuffer(jsonBody))
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var response CreateSlugURLPairResponse
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					log.Fatal(err)
				}
				mu.Lock()
				shortenedURLs[index] = makeOutput(response.SlugURLPair.Slug)
				fmt.Fprintln(os.Stderr, "\u2713  "+url)
				mu.Unlock()
			} else if resp.StatusCode == http.StatusTooManyRequests {
				fmt.Fprintln(os.Stderr, "Rate limit exceeded. Skipping url: "+url)
				mu.Lock()
				shortenedURLs[index] = ""
				mu.Unlock()
			}
		}(url, i)
	}
	wg.Wait() // wait until all goroutines are done
	if err := output(ctx, urls, shortenedURLs); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stderr, "Done!")
	if ctx.String("output") != "" {
		fmt.Fprintln(os.Stderr, "Saved output to: "+ctx.String("output"))
	}
	os.Exit(0)
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Value: false,
				Usage: "output in json format",
			},
			&cli.StringFlag{
				Name:  "output",
				Value: "",
				Usage: "output to file",
			},
		},
		Name:  "delongify",
		Usage: "Shrinks your url's",
		Action: func(ctx *cli.Context) error {
			defaultAction(ctx)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
