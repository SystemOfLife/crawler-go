package crawler

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"golang.org/x/net/html"
)

type Crawler struct {
	maxDepth   int
	maxVisits  int
	startPage  string
	filter     *regexp.Regexp
	Visited    map[string]bool // mb inteface, but than generate bool var every map check
	Wg         *sync.WaitGroup
	mutex      *sync.Mutex
	client     *http.Client
	grabClient *grab.Client
	workers    int
	reqch      chan *grab.Request
	respch     chan *grab.Response
}

func NewCrawler(startPage string, filter *regexp.Regexp, maxDepth, maxVisits, workers int) *Crawler {
	t := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,

		TLSHandshakeTimeout: 30 * time.Second,
	}
	c := grab.NewClient()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.3"
	c.UserAgent = ua

	defaultCLient := &http.Client{
		Transport: t,
	}

	return &Crawler{
		startPage:  startPage,
		maxDepth:   maxDepth,
		maxVisits:  maxVisits,
		filter:     filter,
		client:     defaultCLient,
		grabClient: c,
		mutex:      new(sync.Mutex),
		Wg:         new(sync.WaitGroup),
		Visited:    make(map[string]bool, maxVisits),
		workers:    workers,
		reqch:      make(chan *grab.Request),
		respch:     make(chan *grab.Response),
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

func (c *Crawler) parseHTMLContent(pageURL string, htmlContent []byte, depth int) {
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
			} else if token.Data == "link" || token.Data == "script" {
				var attrName, attrVal string
				for _, attr := range token.Attr {
					if attr.Key == "href" || attr.Key == "src" {
						attrName = attr.Key
						attrVal = attr.Val
					}
				}
				if attrName != "" && attrVal != "" {
					if strings.HasSuffix(attrVal, ".html") || strings.HasSuffix(attrVal, ".css") || strings.HasSuffix(attrVal, ".js") {
						req, _ := grab.NewRequest(fmt.Sprintf("./data/%s", attrVal), attrVal)
						c.grabClient.Do(req)
					}
				}
			}
		}
	}
}

func (c *Crawler) regexpExtract(content []byte, pageURL string, depth int) {
	re := regexp.MustCompile(`href="([^"]+)"|src="([^"]+)"`)

	matches := re.FindAllStringSubmatch(string(content), -1)

	var url string
	for _, match := range matches {
		if match[1] != "" {
			url = match[1]
		} else if match[2] != "" {
			url = match[2]
		}

		absLink, err := c.getAbsoluteURL(pageURL, url)
		if err != nil {
			fmt.Printf("Crawler.getAbsoluteURL: Error processing link %s: %s\n", url, err)
			continue
		}

		// TODO: need to parallerize download properly
		req, _ := grab.NewRequest(fmt.Sprintf("./data/%s", url), absLink)
		c.reqch <- req
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
	// reqch := make(chan *grab.Request)
	// respch := make(chan *grab.Response)

	for i := 0; i < c.workers; i++ {
		c.Wg.Add(1)
		go func() {
			c.grabClient.DoChannel(c.reqch, c.respch)
			c.Wg.Done()
		}()
	}

	c.Wg.Add(1)
	c.crawl(c.startPage, c.maxDepth)
}

func (c *Crawler) Close() {
	close(c.reqch)
	close(c.respch)
}

func (c *Crawler) isAllowed(URL string) bool {
	return c.filter.MatchString(URL)
}
