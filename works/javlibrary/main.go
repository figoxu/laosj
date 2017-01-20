package main

import (
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/figoxu/laosj/spider"
	"github.com/quexer/utee"
	"github.com/songtianyi/rrframework/logs"
	"github.com/songtianyi/rrframework/storage"
	"github.com/songtianyi/rrframework/utils"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

var (
	cmd = flag.String("cmd", "", "choose what you want\n"+
		"-cmd list, get all av star videos list\n"+
		"-cmd get -art GS-064, get specified video torrent file\n")
	artOp   = flag.String("art", "", "specify one art")
	withImg = flag.Int("image", 0, "-image {1|0} whether show video image in list")
)

const (
	JAV_PREFIX = "http://www.javlibrary.com/cn/"
)

func parseStar(item string, artist string) {
	m := regexp.MustCompile("href=\"(\\S+)\"").FindStringSubmatch(item)
	if len(m) < 2 {
		utee.Chk(fmt.Errorf("Find href error, %s", "len(m) < 2"))
	}
	url := m[1]
	jvname := strings.Split(url, "=")[1]
	s, err := spider.NewSpider(JAV_PREFIX + url + "&mode=2")
	if err != nil {
		utee.Chk(err)
	}
	arts, _ := s.GetText("div.videothumblist>div.videos>div.video>a>div.id")
	artsrcs, _ := s.GetHtml("div.videothumblist>div.videos>div.video>a")

	if len(artsrcs) != len(arts) {
		logs.Error(s.Url, len(artsrcs), len(arts))
		logs.Debug(artsrcs)
		logs.Debug(arts)
		utee.Chk(fmt.Errorf(""))
	}
	for i, artsrc := range artsrcs {
		m1 := regexp.MustCompile("src=\"(\\S+)\"").FindStringSubmatch(artsrc)
		if len(m1) < 2 {
			utee.Chk(fmt.Errorf("Find href fail, %s", artsrc))
		}
		coverurl := m1[1]
		if *withImg == 0 {
			logs.Info(jvname, arts[i], artist, m1[1])
		} else {
			surl := strings.Split(coverurl, "/")
			filename := surl[len(surl)-1]
			logs.Info(jvname, arts[i], artist, filename)
		}
	}
}

func runList() {
	// for every prefix=?
	for i := 0; i < 26; i++ {
		// get page num
		s, err := spider.NewSpider(JAV_PREFIX + "star_list.php?prefix=" + string(rune(i+65)))
		if err != nil {
			logs.Error(err)
			continue
		}
		rs, _ := s.GetText("div.page_selector>a.page")
		max := spider.FindMaxInt(0, rs)
		// for every page
		var wg sync.WaitGroup
		for j := 1; j <= max; j++ {
			// find stars in this page
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				s1, err := spider.NewSpider(s.Url + "&page=" + string(rune(k)))
				if err != nil {
					logs.Error(err)
					return
				}
				rs1, _ := s1.GetHtml("div.searchitem")
				rs2, _ := s1.GetText("div.searchitem>a")
				if len(rs1) != len(rs2) {
					logs.Error("assert fail")
					return
				}
				for n := range rs1 {
					parseStar(rs1[n], rs2[n])
				}
			}(j)
		}
		wg.Wait()
	}
}

func runGet(art string) {
	store := rrstorage.CreateLocalDiskStorage(".")

	// get download url
	url := "https://sukebei.nyaa.se/?page=search&cats=8_30&filter=0&sort=4&term=" + art
	rul := "div.content>table.tlist>tbody>tr.tlistrow.trusted>td.tlistdownload"
	doc, err := goquery.NewDocument(url)
	if err != nil {
		logs.Error(err)
		return
	}
	doc.Find(rul).Each(func(ix int, sl *goquery.Selection) {
		uri, _ := sl.Find("a").Attr("href")
		uri = "https:" + uri

		logs.Info("downloading", uri)
		resp, err := http.Get(uri)
		if err != nil {
			logs.Error(err)
			return
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logs.Error(err)
			return
		}
		if err := store.Save(b, art+"-"+rrutils.NewV4().String()+".torrent"); err != nil {
			logs.Error(err)
			return
		}
	})
}

func main() {
	flag.Parse()
	if *cmd == "list" {
		runList()
	} else if *cmd == "get" {
		if *artOp == "" {
			logs.Error("wrong usage\n./jav -cmd get -art IENE-706")
			return
		}
		runGet(*artOp)
	} else {
	}
}
