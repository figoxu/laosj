package main

import (
	"github.com/figoxu/laosj/downloader"
	"github.com/figoxu/laosj/spider"
	"github.com/songtianyi/rrframework/connector/redis"
	"github.com/songtianyi/rrframework/logs"
	"github.com/songtianyi/rrframework/storage"
	"regexp"
	"strconv"
	"sync"
)

func main() {
	d := &downloader.Downloader{
		ConcurrencyLimit: 10,
		UrlChannelFactor: 10,
		RedisConnStr:     "127.0.0.1:6379",
		SourceQueue:      "DATA:IMAGE:MZITU:XINGGAN",
		Store:            rrstorage.CreateLocalDiskStorage("/data/sexx/taiwan/"),
	}
	go func() {
		d.Start()
	}()

	// step1: find total index pages
	s, err := spider.NewSpider("http://www.mzitu.com/taiwan")
	if err != nil {
		logs.Error(err)
		return
	}
	rs, _ := s.GetText("div.main>div.main-content>div.postlist>nav.navigation.pagination>div.nav-links>a.page-numbers")
	max := spider.FindMaxInt(1, rs)

	// step2: for every index page, find every post entrance
	var wg sync.WaitGroup
	var mu sync.Mutex
	step2 := make([]string, 0)
	for i := 1; i <= max; i++ {
		wg.Add(1)
		go func(ix int) {
			defer wg.Done()
			ns, err := spider.NewSpider(s.Url + "/page/" + strconv.Itoa(ix))
			if err != nil {
				logs.Error(err)
				return
			}
			t, _ := ns.GetHtml("div.main>div.main-content>div.postlist>ul>li")
			mu.Lock()
			step2 = append(step2, t...)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	// parse url
	for i, v := range step2 {
		re := regexp.MustCompile("href=\"(\\S+)\"")
		m := re.FindStringSubmatch(v)
		if len(m) < 2 {
			continue
		}
		step2[i] = m[1]
	}

	for _, v := range step2 {
		// step3: step in entrance, find max pagenum
		ns1, err := spider.NewSpider(v)
		if err != nil {
			logs.Error(err)
			return
		}
		t1, _ := ns1.GetText("div.main>div.content>div.pagenavi>a")
		maxx := spider.FindMaxInt(1, t1)
		// step4: for every page
		for j := 1; j <= maxx; j++ {

			// step5: find img in this page
			ns2, err := spider.NewSpider(v + "/" + strconv.Itoa(j))
			if err != nil {
				logs.Error(err)
				return
			}
			t2, err := ns2.GetHtml("div.main>div.content>div.main-image>p>a")
			if len(t2) < 1 {
				// ignore this page
				continue
			}
			sub := regexp.MustCompile("src=\"(\\S+)\"").FindStringSubmatch(t2[0])
			if len(sub) != 2 {
				// ignore this page
				continue
			}
			err, rc := rrredis.GetRedisClient(d.RedisConnStr)
			if err != nil {
				logs.Error(err)
				return
			}
			key := d.SourceQueue
			if _, err := rc.RPush(key, sub[1]); err != nil {
				logs.Error(err)
				return
			}
		}
	}
	d.WaitCloser()
}
