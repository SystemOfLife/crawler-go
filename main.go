package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
)

// Crawler represents our web crawler.
type Crawler struct {
	MaxDepth  int
	MaxVisits int
	Visited   map[string]bool
	Mutex     sync.Mutex
}

func NewCrawler(domain string, maxDepth, maxVisits int) *Crawler {
	return &Crawler{
		MaxDepth:  maxDepth,
		MaxVisits: maxVisits,
		Visited:   make(map[string]bool),
	}
}

// FetchPage just fetches the body Oo
func (c *Crawler) FetchPage(pageURL string) ([]byte, error) {
	resp, err := http.Get(pageURL)
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

// Crawl recursively crawls the URL
func (c *Crawler) Crawl(pageURL string, depth int) {
	if depth <= 0 || len(c.Visited) == c.MaxVisits { // to stop recursion or limit
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

	body, err := c.FetchPage(pageURL)
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

	for _, link := range links {
		absLink, err := c.AbsoluteURL(pageURL, link)
		if err != nil {
			fmt.Printf("Error processing link %s: %s\n", link, err)
			continue
		}
		absLinks = append(absLinks, absLink)
	}

	for _, abs := range absLinks {
		c.Crawl(abs, depth-1)
	}
}

func (c *Crawler) AbsoluteURL(baseURL, link string) (string, error) {
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

func main() {
	crawler := NewCrawler("", 2, 100)

	rootURL := "https://github.com/axi0mX/ipwndfu/issues/141"
	crawler.Crawl(rootURL, crawler.MaxDepth)

	fmt.Println()
	fmt.Printf("End of crawling. Number of visited links: %d", len(crawler.Visited))
}
