// Package crawler
package crawler

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var tagsToExtractText = map[string]bool{
	"p":       true,
	"div":     true,
	"span":    true,
	"a":       true,
	"h1":      true,
	"h2":      true,
	"h3":      true,
	"h4":      true,
	"h5":      true,
	"h6":      true,
	"li":      true,
	"th":      true,
	"td":      true,
	"article": true,
	"main":    true,
	"section": true,
	"pre":     true,
}

func ExtractTextAndLinks(body io.Reader, baseURL *url.URL) (string, []string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", nil, err
	}

	var textBuilder strings.Builder
	links := make(map[string]struct{})

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode && n.Parent != nil && tagsToExtractText[n.Parent.Data] {
			trimmedText := strings.TrimSpace(n.Data)
			if len(trimmedText) > 0 {
				textBuilder.WriteString(trimmedText)
				textBuilder.WriteString(" ")
			}
		}

		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					linkURL, err := baseURL.Parse(a.Val)
					if err == nil {
						links[linkURL.String()] = struct{}{}
					}
					break
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)

	uniqueLinks := make([]string, 0, len(links))
	for link := range links {
		uniqueLinks = append(uniqueLinks, link)
	}

	textContent := strings.Join(strings.Fields(textBuilder.String()), " ")

	return textContent, uniqueLinks, nil
}
