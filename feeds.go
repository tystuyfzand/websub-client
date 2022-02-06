package client

import (
    "errors"
    xpp "github.com/mmcdole/goxpp"
    "golang.org/x/net/html/charset"
    "io"
    "log"
)

func extractFeedLinks(r io.Reader) (string, string, error) {
    p := xpp.NewXMLPullParser(r, false, charset.NewReaderLabel)

    var hubUrl, selfUrl string

    feedTag := firstTag(p)

    if feedTag != "rss" && feedTag != "feed" && feedTag != "rdf" {
        return "", "", errors.New("unexpected feed type")
    }

    switch feedTag {
    case "rdf":
        fallthrough
    case "rss":
        skipUntil(p, "channel")

        if err := p.Expect(xpp.StartTag, "channel"); err != nil {
            return "", "", err
        }
    case "feed":
        break
    }

    for {
        event, err := p.NextTag()

        if err != nil {
            return "", "", err
        }

        log.Println(p.Name)

        if event == xpp.StartTag {
            if p.Name == "link" {
                // Handle
                switch p.Attribute("rel") {
                case "self":
                    selfUrl = p.Attribute("href")
                case "hub":
                    hubUrl = p.Attribute("href")
                }

                if selfUrl != "" && hubUrl != "" {
                    break
                }
            } else {
                p.Skip()
            }
        } else if event == xpp.EndTag {
            if p.Name == "channel" {
                break
            }
        } else if event == xpp.EndDocument {
            break
        }
    }

    return selfUrl, hubUrl, nil
}

// skipUntil skips tags from the parser until we have the expected tag
func skipUntil(p *xpp.XMLPullParser, tagName string) {
    for {
        event, err := p.NextTag()

        if err != nil || event == xpp.EndDocument {
            break
        }

        if event == xpp.StartTag {
            if p.Name == tagName {
                return
            }

            p.Skip()
        }
    }
}

// firstTag gets the first start tag
func firstTag(p *xpp.XMLPullParser) string {
    for {
        event, err := p.NextTag()

        if err != nil || event == xpp.EndDocument {
            break
        }

        if event == xpp.StartTag {
            return p.Name
        }
    }

    return ""
}
