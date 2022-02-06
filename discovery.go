package client

import (
    "github.com/PuerkitoBio/goquery"
    "github.com/tomnomnom/linkheader"
    "io"
    "net/http"
    "strings"
)

// Discover pulls
func (c *Client) Discover(topicURL string) (string, string, error) {
    req, err := http.NewRequest(http.MethodGet, topicURL, nil)

    if err != nil {
        return "", "", err
    }

    res, err := c.client.Do(req)

    if err != nil {
        return "", "", err
    }

    defer res.Body.Close()

    if linkHeaders := res.Header[http.CanonicalHeaderKey("Link")]; len(linkHeaders) > 0 {
        hubUrl, selfUrl := extractLinkHeader(linkHeaders)

        if hubUrl != "" && selfUrl != "" {
            return hubUrl, selfUrl, nil
        }
    }

    contentType := res.Header.Get("Content-Type")

    if idx := strings.Index(contentType, ";"); idx != -1 {
        contentType = contentType[0:idx]
    }

    switch contentType {
    case "text/xml", "application/xml", "application/rss+xml":
        return extractFeedLinks(res.Body)
    case "text/html":
        return extractHtmlLinks(res.Body)
    }

    return "", "", ErrNoHub
}

// extractLinkHeader extracts link headers for hub and self.
func extractLinkHeader(header []string) (string, string) {
    links := linkheader.ParseMultiple(header)

    if len(links) < 1 {
        return "", ""
    }

    self := links.FilterByRel("self")

    if len(self) < 1 {
        return "", ""
    }

    hub := links.FilterByRel("hub")

    if len(hub) < 1 {
        return "", ""
    }

    return self[0].URL, hub[0].URL
}

// extractHtmlLinks
func extractHtmlLinks(r io.Reader) (string, string, error) {
    doc, err := goquery.NewDocumentFromReader(r)

    if err != nil {
        return "", "", err
    }

    links := doc.Find("link")

    if links.Length() < 1 {
        return "", "", ErrNoHub
    }

    self := links.FilterFunction(func(_ int, s *goquery.Selection) bool {
        attr, exists := s.Attr("rel")

        if !exists {
            return false
        }

        return attr == "self"
    })

    if self.Length() < 1 {
        return "", "", ErrNoSelf
    }

    hub := links.FilterFunction(func(_ int, s *goquery.Selection) bool {
        attr, exists := s.Attr("rel")

        if !exists {
            return false
        }

        return attr == "hub"
    })

    if hub.Length() < 1 {
        return "", "", ErrNoHub
    }

    return self.AttrOr("href", ""), hub.AttrOr("href", ""), nil
}