package spider

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"sync"
)

// Spider
type Spider struct {
	Url string // page that spider would deal with
	doc *goquery.Document
}

// Start spider
func NewSpider(url string) (*Spider, error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return nil, fmt.Errorf("url %s, error %s", url, err)
	}
	return &Spider{Url: url, doc: doc}, nil
}

func (p *Spider) GetHtml(selector string) ([]string, error) {
	var (
		res = make([]string, 0) //for leaf
		wg  sync.WaitGroup
		mu  sync.Mutex
	)

	p.doc.Find(selector).Each(func(ix int, sl *goquery.Selection) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			content, _ := sl.Html()
			mu.Lock()
			res = append(res, content)
			mu.Unlock()

		}()
	})
	wg.Wait()
	return res, nil
}

func (s *Spider) GetText(selector string) ([]string, error) {
	var (
		res = make([]string, 0) //for leaf
		wg  sync.WaitGroup
		mu  sync.Mutex
	)

	s.doc.Find(selector).Each(func(ix int, sl *goquery.Selection) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			res = append(res, sl.Text())
			mu.Unlock()
		}()
	})
	wg.Wait()
	return res, nil
}
