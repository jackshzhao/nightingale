package aconf

import (
	"path"
)

type Alert struct {
	Disable     bool
	EngineDelay int64
	Heartbeat   HeartbeatConfig
	Alerting    Alerting
	Syslog      SyslogConfig
	PM          PMConfig
}

type SMTPConfig struct {
	Host               string
	Port               int
	User               string
	Pass               string
	From               string
	InsecureSkipVerify bool
	Batch              int
}

type HeartbeatConfig struct {
	IP         string
	Interval   int64
	Endpoint   string
	EngineName string
}

type Alerting struct {
	Timeout           int64
	TemplatesDir      string
	NotifyConcurrency int
}

type SyslogConfig struct {
	Enable bool
	Listen string
}

// PMConfig 支持多 IP 认证轮询
type PMConfig struct {
	Enable   bool     // 是否启用 PM 认证轮询
	Interval int64    // 轮询间隔，单位：秒
	IPs      []string // 需要轮询的网关 IP 列表
}

type CallPlugin struct {
	Enable     bool
	PluginPath string
	Caller     string
}

type RedisPub struct {
	Enable        bool
	ChannelPrefix string
	ChannelKey    string
}

func (a *Alert) PreCheck(configDir string) {
	if a.Alerting.TemplatesDir == "" {
		a.Alerting.TemplatesDir = path.Join(configDir, "template")
	}

	if a.Alerting.NotifyConcurrency == 0 {
		a.Alerting.NotifyConcurrency = 10
	}

	if a.Heartbeat.Interval == 0 {
		a.Heartbeat.Interval = 1000
	}

	if a.Heartbeat.EngineName == "" {
		a.Heartbeat.EngineName = "default"
	}

	if a.EngineDelay == 0 {
		a.EngineDelay = 30
	}

	// defaults for syslog
	if a.Syslog.Listen == "" {
		a.Syslog.Listen = ":514"
	}

	// defaults for PM
	if a.PM.Interval == 0 {
		a.PM.Interval = 30 // 默认 30s
	}
}
