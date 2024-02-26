package main

import (
	"fmt"
	"regexp"

	crw "crawler-go/crawler"
)

func main() {
	filter := regexp.MustCompile("https://github.com/.+")

	crawler := crw.NewCrawler("https://github.com/axi0mX/ipwndfu/issues/141", filter, 2, 100) // with depth 3 need to add backoff

	crawler.Start()

	crawler.Wg.Wait()

	fmt.Printf("End of crawling. Number of visited links: %d\n", len(crawler.Visited))
}
