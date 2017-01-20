package downloader

import (
	"fmt"
	"github.com/quexer/utee"
	"github.com/songtianyi/rrframework/connector/redis"
	"github.com/songtianyi/rrframework/logs"
	"github.com/songtianyi/rrframework/storage"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	URL_CACHE_KEY = "DATA:IMAGE:DOWNLOADED:URLS" // Key for downloaded url cache
)

// Struct channel
type Url struct {
	v string
}

// Downloader get urls from redis SourceQueue
// and download them concurrently
// then save downloaded binary to storage
type Downloader struct {
	// exported
	ConcurrencyLimit int                      // max number of goroutines to download
	RedisConnStr     string                   // redis connection string
	SourceQueue      string                   // url queue
	Store            rrstorage.StorageWrapper // for saving downloaded binary
	UrlChannelFactor int

	// inner use
	chSema chan struct{}        // for concurrency-limiting
	chFlag chan struct{}        // stop flag
	chUrls chan Url             // url channel queue
	rc     *rrredis.RedisClient // redis client
}

// Start downloader
func (p *Downloader) Start() {
	err, rc := rrredis.GetRedisClient(p.RedisConnStr)
	utee.Chk(err)
	p.rc = rc

	// create channel
	p.chSema = make(chan struct{}, p.ConcurrencyLimit)
	p.chFlag = make(chan struct{})
	p.chUrls = make(chan Url, p.ConcurrencyLimit*p.UrlChannelFactor)

	go func() {
		p.schedule()
	}()

	tick := time.Tick(2 * time.Second)

loop2:
	for {
		select {
		case <-p.chFlag:
			// be stopped
			for url := range p.chUrls {
				// push back to redis queue
				if _, err := rc.RPush(p.SourceQueue, url.v); err != nil {
					logs.Error(err)
				}
			}
			// end downloader
			break loop2
		case p.chSema <- struct{}{}:
			// s.sema not full
			url, ok := <-p.chUrls
			if !ok {
				// channel closed
				logs.Error("Channel s.urls may be closed")
				// TODO what's the right way to deal this situation?
				break loop2
			}
			go func() {
				if err := p.download(url.v); err != nil {
					// download fail
					// push back to redis
					logs.Error("Download %s fail, %s", url.v, err)
					if _, err := rc.RPush(p.SourceQueue, url.v); err != nil {
						logs.Error("Push back to redis failed, %s", err)
					}
				} else {
					// download success
					// push downloaded url to cache
					if err := rc.HMSet(URL_CACHE_KEY, map[string]string{
						url.v: "1",
					}); err != nil {
						logs.Error("Push to cache failed, %s", err)
					}
				}
			}()
		case <-tick:
			// print this every 2 seconds
			logs.Info("In queue: %d, doing: %d", len(p.chUrls), len(p.chSema))
		}
	}

}

// Stop downloader
func (p *Downloader) Stop() {
	close(p.chFlag)
}

func (p *Downloader) WaitCloser() {
	tok := time.Tick(time.Second * 1)
	for {
		select {
		case <-tok:
			if len(p.chUrls) > 0 || len(p.chSema) > 1 {
				// TODO there is a chance that last url downloading process be interupted
				continue
			}
			if v, err := p.rc.LLen(p.SourceQueue); err != nil || v != 0 {
				if err != nil {
					logs.Error(err)
				}
				continue
			}
			return
		}
	}
}

func (p *Downloader) download(url string) error {
	defer func() { <-p.chSema }() // release
	exist, err := p.rc.HMExists(URL_CACHE_KEY, url)
	if err != nil {
		return err
	}
	if exist {
		logs.Info("%s downloaded", url)
		return nil
	}
	logs.Info("Downloading %s", url)
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) { return net.DialTimeout(network, addr, 3*time.Second) },
		},
	}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("StatusCode %d", response.StatusCode)
	}
	filename := func() {
		strs := strings.Split(url, "/")
		if len(strs) < 1 {
			utee.Chk(fmt.Errorf("Bad URL:", url))
		}
		return strs[len(strs)-1]
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if err := p.Store.Save(b, filename()); err != nil {
		return err
	}
	return nil
}

func (p *Downloader) schedule() {
	for {
		url, err := p.rc.LPop(p.SourceQueue)
		if err == rrredis.Nil {
			time.Sleep(5 * time.Second)
			continue
		}
		if err != nil {
			logs.Error(err)
			time.Sleep(10 * time.Second)
			continue
		}
		select {
		case <-p.chFlag:
			return
		case p.chUrls <- Url{v: url}:
		}
	}
}
