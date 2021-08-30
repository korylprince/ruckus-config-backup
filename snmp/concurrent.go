package snmp

import (
	"log"
	"sync"

	"github.com/korylprince/ruckus-config-backup/tftp"
)

func (c *Config) worker(in <-chan string, wg *sync.WaitGroup, svr *tftp.Server) {
	for host := range in {
		if err := c.DownloadConfig(host, svr); err != nil {
			log.Printf("WARNING: Could not get config from %s: %v\n", host, err)
		}
	}
	wg.Done()
}

func (c *Config) DownloadConfigs(hosts []string, workers int, svr *tftp.Server) {
	ch := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go c.worker(ch, wg, svr)
	}
	for _, h := range hosts {
		ch <- h
	}
	close(ch)
	wg.Wait()
}
