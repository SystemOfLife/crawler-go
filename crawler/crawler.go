package crawler

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Crawler struct {
	maxDepth  int
	maxVisits int
	startPage string
	filter    *regexp.Regexp
	Visited   map[string]bool // mb inteface, but than generate bool var every map check
	Wg        *sync.WaitGroup
	mutex     *sync.Mutex
	client    *http.Client
}

func NewCrawler(startPage string, filter *regexp.Regexp, maxDepth, maxVisits int) *Crawler {
	t := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,

		TLSHandshakeTimeout: 30 * time.Second,
	}
	c := &http.Client{
		Transport: t,
	}
	return &Crawler{
		startPage: startPage,
		maxDepth:  maxDepth,
		maxVisits: maxVisits,
		filter:    filter,
		client:    c,
		mutex:     new(sync.Mutex),
		Wg:        new(sync.WaitGroup),
		Visited:   make(map[string]bool, maxVisits),
	}
}

// FetchPage just fetches the body Oo TODO: ue grab lib
func (c *Crawler) fetchPage(pageURL string) ([]byte, error) {
	resp, err := c.client.Get(pageURL)
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

func (c *Crawler) extractLinks(pageURL string, htmlContent []byte, depth int) {
	reader := bytes.NewReader(htmlContent)
	tokenizer := html.NewTokenizer(reader)

	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}

		if tokenType == html.StartTagToken {
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" && attr.Val != "" { // cut same URL before checking visited map
						if string(attr.Val)[0:1] != "#" { // TODO: is there a problem?
							absLink, err := c.getAbsoluteURL(pageURL, attr.Val)
							if err != nil {
								fmt.Printf("Crawler.getAbsoluteURL: Error processing link %s: %s\n", attr.Val, err)
								continue
							}

							// check regex and go deeper
							if c.isAllowed(absLink) {
								c.Wg.Add(1)
								go c.crawl(absLink, depth-1)
							}
						}
					}
				}
			}
		}
	}
}

// crawl recursively crawls the URL
func (c *Crawler) crawl(pageURL string, depth int) {
	defer c.Wg.Done()

	// to stop recursion or stop on limit. Ideally should use ctx.Done() on limit with gorutines
	if depth <= 0 || len(c.Visited) == c.maxVisits {
		return
	}

	c.mutex.Lock() // protect map
	if c.Visited[pageURL] {
		c.mutex.Unlock()
		return
	}
	c.Visited[pageURL] = true
	c.mutex.Unlock()

	fmt.Printf("Crawling: %s\n", pageURL)

	body, err := c.fetchPage(pageURL)
	if err != nil {
		fmt.Printf("Error fetching page %s: %s\n", pageURL, err)
		return
	}

	c.extractLinks(pageURL, body, depth)
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
	c.Wg.Add(1)
	c.crawl(c.startPage, c.maxDepth)
}

func (c *Crawler) isAllowed(URL string) bool {
	return c.filter.MatchString(URL)
}
