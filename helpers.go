package main

import (
	"bytes"

	"golang.org/x/net/html"
)

// ExtractLinks parses HTML content and extracts links.
func ExtractLinks(htmlContent []byte) ([]string, error) {
	links := make([]string, 0)

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
					if attr.Key == "href" && attr.Val != "" {
						if string(attr.Val)[0:1] != "#" {
							links = append(links, attr.Val)
						}
					}
				}
			}
		}
	}

	return links, nil
}
