package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
)

type Crawler struct {
	MaxDepth  int
	MaxVisits int
	StartPage string
	Filter    *regexp.Regexp
	Visited   map[string]bool
	Mutex     *sync.Mutex
}

func NewCrawler(startPage string, filter *regexp.Regexp, maxDepth, maxVisits int) *Crawler {
	return &Crawler{
		StartPage: startPage,
		MaxDepth:  maxDepth,
		MaxVisits: maxVisits,
		Filter:    filter,
		Mutex:     new(sync.Mutex),
		Visited:   make(map[string]bool),
	}
}

// FetchPage just fetches the body Oo
func (c *Crawler) fetchPage(pageURL string) ([]byte, error) {
	resp, err := http.Get(pageURL) // TODO: mb add timeouts?
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch page: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// crawl recursively crawls the URL
func (c *Crawler) crawl(pageURL string, depth int) {
	if depth <= 0 || len(c.Visited) == c.MaxVisits { // to stop recursion or stop on limit. Ideally should use ctx.Done() on limit with gorutines
		return
	}

	c.Mutex.Lock() // protect map
	if c.Visited[pageURL] {
		c.Mutex.Unlock()
		return
	}
	c.Visited[pageURL] = true
	c.Mutex.Unlock()

	fmt.Printf("Crawling: %s\n", pageURL)

	body, err := c.fetchPage(pageURL)
	if err != nil {
		fmt.Printf("Error fetching page %s: %s\n", pageURL, err)
		return
	}

	links, err := ExtractLinks(body)
	if err != nil {
		fmt.Printf("Error extracting links from page %s: %s\n", pageURL, err)
		return
	}

	absLinks := make([]string, 0, len(links))

	for _, link := range links { // mb should invoke crawl not in loop
		absLink, err := c.getAbsoluteURL(pageURL, link)
		if err != nil {
			fmt.Printf("Crawler.getAbsoluteURL: Error processing link %s: %s\n", link, err)
			continue
		}
		if c.isAllowed(absLink) {
			absLinks = append(absLinks, absLink)
		}
	}

	for _, abs := range absLinks {
		c.crawl(abs, depth-1)
	}
}

func (c *Crawler) getAbsoluteURL(baseURL, link string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	rel, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	absURL := base.ResolveReference(rel)
	return absURL.String(), nil
}

func (c *Crawler) Start() {
	c.crawl(c.StartPage, c.MaxDepth)
}

func (c *Crawler) isAllowed(URL string) bool {
	return c.Filter.MatchString(URL)
}

func main() {
	filter := regexp.MustCompile("https://github.com/.+")

	crawler := NewCrawler("https://github.com/axi0mX/ipwndfu/issues/141", filter, 2, 100)

	crawler.Start()

	fmt.Printf("End of crawling. Number of visited links: %d\n", len(crawler.Visited))
}
