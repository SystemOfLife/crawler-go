package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	c := &http.Client{}

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

// FetchPage just fetches the body Oo
func (c *Crawler) fetchPage(pageURL string) ([]byte, error) {
	resp, err := c.client.Get(pageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch page: %s", resp.Status)
	}

	// Check text content-type in links
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/") {
		// Skip non-text URLs
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Crawler) regexpExtract(content []byte, pageURL string, depth int) {
	re := regexp.MustCompile(`href="([^"]+)"|src="([^"]+)"`)

	matches := re.FindAllStringSubmatch(string(content), -1)

	var matchedUrl string
	for _, match := range matches {
		if match[1] != "" {
			matchedUrl = match[1]
		} else if match[2] != "" {
			matchedUrl = match[2]
		}

		absLink, err := c.getAbsoluteURL(pageURL, matchedUrl)
		if err != nil {
			fmt.Printf("Crawler.getAbsoluteURL: Error processing link %s: %s\n", matchedUrl, err)
			continue
		}

		c.mutex.Lock() // protect map on both read/write
		if c.Visited[absLink] {
			c.mutex.Unlock()
			continue
		}
		c.Visited[pageURL] = true
		c.mutex.Unlock()

		go c.downloadTextFile(pageURL)
		// check regex and go deeper
		if c.isAllowed(absLink) {
			c.Wg.Add(1)
			go c.crawl(absLink, depth-1)
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

	c.mutex.Lock() // protect map on both read/write
	if c.Visited[pageURL] {
		c.mutex.Unlock()
		return
	}
	c.Visited[pageURL] = true
	c.mutex.Unlock()

	fmt.Printf("Crawling: %s\n", pageURL)

	body, err := c.fetchPage(pageURL)
	if err != nil {
		fmt.Printf("error fetching page %s: %s\n", pageURL, err)
		return
	}
	go c.downloadTextFile(pageURL)
	c.regexpExtract(body, pageURL, depth)
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

func (c *Crawler) downloadTextFile(pageURL string) {
	c.Wg.Add(1)
	defer c.Wg.Done()

	// TODO need to delete second downloading for basic HTML in crawl method
	resp, err := c.client.Get(pageURL)
	if err != nil {
		fmt.Printf("error download page %s: %s\n", pageURL, err)
		return
	}
	defer resp.Body.Close()

	// Check text content-type in links
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/") {
		return
	}

	fileName := filepath.Base(pageURL)
	filePath := filepath.Join("./data/", fileName)
	out, err := os.Create(filePath)
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("Error saving file %s: %s\n", filePath, err)
	}
}
