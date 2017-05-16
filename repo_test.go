package repo

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"testing"

	"context"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/russross/blackfriday"
)

func TestAlphaAndEmbed(t *testing.T) {
	query := startQuery()

	query.Find("body > ul").Each(func(_ int, s *goquery.Selection) {
		testList(t, s)
	})
}

func TestDuplicatedLinks(t *testing.T) {
	query := startQuery()
	links := make(map[string]bool, 0)

	query.Find("body  a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			t.Errorf("expected '%s' to have href", s.Text())
		}

		if links[href] {
			t.Fatalf("duplicated link '%s'", href)
		}

		links[href] = true
	})
}

func testList(t *testing.T, list *goquery.Selection) {
	list.Find("ul").Each(func(_ int, items *goquery.Selection) {
		testList(t, items)
		items.RemoveFiltered("ul")
	})

	cat := list.Prev().Text()

	t.Run(fmt.Sprintf("order of [%s]", cat), func(t *testing.T) {
		checkAlphabeticOrder(t, list)
	})
	t.Run(fmt.Sprintf("embeds in [%s]", cat), func(t *testing.T) {
		checkEmbeds(t, list)
	})
}

func readme() []byte {
	input, err := ioutil.ReadFile("./README.md")
	if err != nil {
		panic(err)
	}
	html := append([]byte("<body>"), blackfriday.MarkdownCommon(input)...)
	html = append(html, []byte("</body>")...)
	return html
}

func startQuery() *goquery.Document {
	buf := bytes.NewBuffer(readme())
	query, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		panic(err)
	}

	return query
}

func checkAlphabeticOrder(t *testing.T, s *goquery.Selection) {
	items := s.Find("li > a:first-child").Map(func(_ int, li *goquery.Selection) string {
		return strings.ToLower(li.Text())
	})

	sorted := make([]string, len(items))
	copy(sorted, items)
	sort.Strings(sorted)

	for k, item := range items {
		if item != sorted[k] {
			t.Errorf("expected '%s' but actual is '%s'", sorted[k], item)
		}
	}
	if t.Failed() {
		t.Logf("expected order is:\n%s", strings.Join(sorted, "\n"))
	}
}

func checkEmbeds(t *testing.T, s *goquery.Selection) {
	s.Find("li > a:first-child").Each(func(idx int, li *goquery.Selection) {
		repourl, _ := li.Attr("href")
		if strings.Contains(repourl, "#") {
			return
		}
		t.Run(fmt.Sprintf("embed for [%s]", li.Text()), func(t *testing.T) {
			checkEmbed(t, li.Parent().Find("a").Next())
		})
	})
}

func checkEmbed(t *testing.T, embed *goquery.Selection) {
	if embed.Text() != "Embed" {
		t.Errorf("expected 'Embed' but actual is '%v'", embed)
	}
	url, ok := embed.Attr("href")
	if !ok {
		t.Fatalf("expected a url to an embed")
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("failed to make request to %s: %s", url, err.Error())
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	rsp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatalf("failed to perform request to %s: %s", url, err.Error())
	}
	conttype := rsp.Header.Get("Content-Type")
	if !strings.Contains(conttype, "text/css") {
		rsp.Body.Close()
		t.Fatalf("expected content type to contain 'text/css' but actual is '%s' ", conttype)
	}
	_, err = ioutil.ReadAll(rsp.Body)
	rsp.Body.Close()
	if err != nil {
		t.Fatalf("failed to obtain content: %s", err.Error())
	}
}
