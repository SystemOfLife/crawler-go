package main

import (
	"fmt"
	"os"
	"regexp"

	crw "crawler-go/crawler"
)

func main() {
	createDataDir()

	filter := regexp.MustCompile("https://github.com/.+")

	crawler := crw.NewCrawler("https://github.com/axi0mX/ipwndfu/issues/141", filter, 2, 100) // with depth 3 need to add backoff

	crawler.Start()

	crawler.Wg.Wait()

	fmt.Printf("End of crawling. Number of visited links: %d\n", len(crawler.Visited))
}

func createDataDir() {
	if _, err := os.Stat("./data"); os.IsNotExist(err) {
		iErr := os.Mkdir("./data", os.ModePerm)
		if iErr != nil {
			fmt.Printf("error while trying to create data directory: %v", iErr)
			os.Exit(1)
		}
	}
}
