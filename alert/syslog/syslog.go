package syslog

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/toolkits/pkg/logger"
)

// SyslogEntry 表示解析出的字段
// 支持以 "<009>" 或 tab (\t) 作为字段分隔符。字段形式为 key=value。
type SyslogEntry struct {
	LogType       string
	OperateTime   string // 原始时间字符串
	OperateAt     time.Time
	Operator      string
	ClientIP      string
	OperateType   string
	OperateObject string
	OperateResult string
	ServerIP      string
	MMJErrorCode  string
	Raw           string
}

// ParseSyslogLine 将单行 syslog 文本解析为 SyslogEntry
func ParseSyslogLine(line string) (*SyslogEntry, error) {
	if line == "" {
		return nil, errors.New("empty line")
	}

	var delim string
	switch {
	case strings.Contains(line, "<009>"):
		delim = "<009>"
	case strings.Contains(line, "\t"):
		delim = "\t"
	default:
		delim = " "
	}

	parts := strings.Split(line, delim)
	m := map[string]string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if idx := strings.IndexByte(p, '='); idx >= 0 {
			k := strings.TrimSpace(p[:idx])
			if strings.Contains(k, ": logType") {
				k = "logType"
			}
			v := strings.TrimSpace(p[idx+1:])
			m[k] = v
		}
	}

	entry := &SyslogEntry{Raw: line}
	entry.LogType = m["logType"]
	entry.OperateTime = m["operateTime"]
	entry.Operator = m["operator"]
	entry.ClientIP = m["clientIp"]
	entry.OperateType = m["operateType"]
	entry.OperateObject = m["operateObject"]
	entry.OperateResult = m["operateResult"]
	entry.ServerIP = m["serverIp"]
	entry.MMJErrorCode = m["mmjErrorCode"]

	if entry.OperateTime != "" {
		layout := "20060102150405"
		raw := entry.OperateTime
		if len(raw) >= 14 {
			secPart := raw[:14]
			if t, err := time.ParseInLocation(layout, secPart, time.Local); err == nil {
				entry.OperateAt = t
			}
		}
	}
	return entry, nil
}

func (e *SyslogEntry) IsError() bool {
	obj := strings.ReplaceAll(e.OperateObject, " ", "")
	if obj != "" && (strings.Contains(obj, "身份认证失败") || strings.Contains(obj, "认证失败")) {
		return true
	}
	if e.MMJErrorCode != "" && e.MMJErrorCode != "0" && e.MMJErrorCode != "00000" {
		return true
	}
	return false
}

func (e *SyslogEntry) IsRecovery() bool {
	obj := strings.ReplaceAll(e.OperateObject, " ", "")
	if obj != "" && (strings.Contains(obj, "业务已恢复") || strings.Contains(obj, "已恢复")) {
		return true
	}
	if e.MMJErrorCode == "0" {
		return true
	}
	return false
}

func (e *SyslogEntry) Describe() string {
	kind := "UNKNOWN"
	if e.MMJErrorCode != "0" {
		kind = "ERROR"
	} else if e.IsRecovery() {
		kind = "RECOVERY"
	}
	return fmt.Sprintf("[%s] time=%s server=%s mmj=%s desc=%s", kind, e.OperateTime, e.ServerIP, e.MMJErrorCode, e.OperateObject)
}

// StartUDPServer 启动一个 UDP 监听器，用于接收 Syslog 日志（默认端口514或自定义端口）
// addr 形如 ":514" 或 "0.0.0.0:514"
func StartUDPServer(addr string, handler func(*SyslogEntry)) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("UDP listen failed: %v", err)
	}
	defer conn.Close()

	buffer := make([]byte, 8192)

	for {
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			logger.Warningf("read error: %v", err)
			continue
		}

		line := strings.TrimSpace(string(buffer[:n]))

		if !strings.Contains(line, "mmjErrorCode") {
			logger.Infof("不是网关告警日志: %s", line)
			continue
		}
		logger.Infof("syslog告警日志: %s", line)
		entry, perr := ParseSyslogLine(line)
		if perr != nil {
			logger.Errorf("parse error: %v", perr)
			continue
		}
		if handler != nil {
			handler(entry)
		}
	}
}
