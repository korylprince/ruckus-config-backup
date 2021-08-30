package main

import "time"

type Config struct {
	RunInterval time.Duration `default:"30m"`

	SNMPUsername   string `required:"true"`
	SNMPAuthPasswd string `required:"true"`
	SNMPPrivPasswd string `required:"true"`

	CommitUsername string `default:"Ruckus Config Backup"`
	CommitEmail    string
	LocalRepo      string `required:"true"`
	RemoteRepo     string `required:"true"`
	RemoteUsername string `required:"true"`
	RemotePasswd   string `required:"true"`

	Hosts []string `required:"true"`
}
