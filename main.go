package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kelseyhightower/envconfig"
	"github.com/korylprince/ruckus-config-backup/git"
	"github.com/korylprince/ruckus-config-backup/snmp"
	"github.com/korylprince/ruckus-config-backup/tftp"
)

func run(config *Config) error {
	repo, err := git.New(config.LocalRepo)
	if err != nil {
		return fmt.Errorf("could not create repo: %w", err)
	}

	t := tftp.New()

	log.Println("INFO: Fetching Ruckus configurations")
	snmp.DefaultConfig.DownloadConfigs(config.Hosts, 16, t)

	hashes := t.Shutdown()

	// convert hashes to hostname files
	configs := make(map[string][]byte)
	for _, h := range config.Hosts {
		sum := md5.Sum([]byte(h))
		hash := hex.EncodeToString(sum[:])
		if buf, ok := hashes[hash]; ok {
			configs[h+".conf"] = buf
		}
	}

	log.Println("INFO: Updating local repo")
	if err = repo.UpdateFiles(configs, &object.Signature{Name: config.CommitUsername, Email: config.CommitEmail, When: time.Now()}); err != nil {
		return fmt.Errorf("could not create update files: %w", err)
	}

	log.Println("INFO: Pushing changes to remote repo")
	if err = repo.PushRemote(config.RemoteRepo, &http.BasicAuth{Username: config.RemoteUsername, Password: config.RemotePasswd}); err != nil {
		return fmt.Errorf("could not push remote: %w", err)
	}

	return nil
}

func main() {
	config := new(Config)
	if err := envconfig.Process("", config); err != nil {
		log.Fatalln("ERROR: Could not process configuration:", err)
	}

	snmp.DefaultConfig.Username = config.SNMPUsername
	snmp.DefaultConfig.AuthPassword = config.SNMPAuthPasswd
	snmp.DefaultConfig.PrivPassword = config.SNMPPrivPasswd

	for {
		if err := run(config); err != nil {
			log.Println("ERROR: Could not finish run:", err)
		}
		time.Sleep(config.RunInterval)
	}
}
