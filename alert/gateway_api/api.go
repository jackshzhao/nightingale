package gateway_api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/alert/aconf"
	"github.com/ccfos/nightingale/v6/alert/queue"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/toolkits/pkg/logger"
	"github.com/toolkits/pkg/str"
)

// 记录每个 PM 网关是否存在未恢复的异常，避免重复恢复事件入库
var pmHasActiveError = map[string]bool{}

// HandelPM 仿照 HandelSyslog 的事件构造逻辑，实现无公钥证书方式访问 PM 认证接口
// POST https://192.168.9.164:443/SendMaintenancePlatform
// Body: {"certificationMark":"pmRequestgw"}
// Resp: {"code":0,"message":"PM认证成功"}
// code==0 正常，不入队；否则异常，告警内容为 message
func HandelPM(serverIP string) {
	url := fmt.Sprintf("https://%s:443/SendMaintenancePlatform", serverIP)
	//url := fmt.Sprintf("http://%s:17000/v1/monitor/test2", serverIP)
	body := strings.NewReader(`{"certificationMark":"pmRequestgw"}`)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		logger.Errorf("网关api请求创建失败: %v", err)
		pmHasActiveError[serverIP] = true
		pushPM(serverIP, -1, "网关api请求创建失败: "+err.Error(), false)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// 网络不通或请求失败：记录日志并告警
		logger.Errorf("网关api请求失败: %v, url: %s", err, url)
		pmHasActiveError[serverIP] = true
		pushPM(serverIP, -1, "网关api请求失败: "+err.Error(), false)
		return
	}
	defer resp.Body.Close()

	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		logger.Errorf("网关api响应解析失败: %v", err)
		pmHasActiveError[serverIP] = true
		pushPM(serverIP, -1, "网关api响应解析失败: "+err.Error(), false)
		return
	}

	if r.Code != 0 {
		// 非 0 视为异常，按 syslog 风格入队
		pmHasActiveError[serverIP] = true
		pushPM(serverIP, r.Code, r.Message, false)
		return
	}

	// code==0 视为恢复：仅在之前出现过异常时入队一次
	if pmHasActiveError[serverIP] {
		pushPM(serverIP, 0, r.Message, true)
		pmHasActiveError[serverIP] = false
		logger.Infof("网关密码机认证成功: code=%d, message=%s", r.Code, r.Message)
	} else {
		logger.Debugf("PM %s 已处于恢复状态，跳过重复恢复事件", serverIP)
	}
}

// pushPMEvent 以与 HandelSyslog 相同的字段风格构造事件并入队
// pushPM 统一处理 PM 告警/恢复事件入队
func pushPM(serverIP string, code int, message string, recovered bool) {
	now := time.Now().Unix()
	hashKey := fmt.Sprintf("syslog_%s", serverIP)

	severity := 1
	if recovered {
		severity = 3
	}

	ruleName := fmt.Sprintf("网关异常: %s", message)
	if recovered {
		ruleName = fmt.Sprintf("网关恢复: %s", message)
	}

	event := &models.AlertCurEvent{
		Cate:             models.LOG,
		Cluster:          "",
		DatasourceId:     0,
		GroupId:          0,
		GroupName:        "syslog",
		Hash:             str.MD5(hashKey),
		RuleId:           0,
		RuleName:         ruleName,
		RuleNote:         "",
		RuleProd:         "syslog",
		RuleAlgo:         "",
		Severity:         severity,
		PromForDuration:  0,
		PromQl:           "",
		PromEvalInterval: 0,
		RunbookUrl:       "",
		NotifyRecovered:  0,
		TargetIdent:      serverIP,
		TargetNote:       "",
		TriggerTime:      now,
		TriggerValue:     fmt.Sprintf("code=%d; message=%s", code, message),
		IsRecovered:      recovered,
		LastEvalTime:     now,
		FirstTriggerTime: now,
		TagsJSON: []string{
			fmt.Sprintf("logType=%s", "pm"),
			fmt.Sprintf("serverIp=%s", serverIP),
			fmt.Sprintf("clientIp=%s", ""),
			fmt.Sprintf("operateType=%s", "密码机认证"),
			fmt.Sprintf("message=%s", strings.TrimSpace(message)),
			fmt.Sprintf("code=%d", code),
		},
		TagsMap: map[string]string{
			"logType":     "pm",
			"serverIp":    serverIP,
			"clientIp":    "",
			"operateType": "密码机认证",
			"message":     strings.TrimSpace(message),
			"code":        fmt.Sprintf("%d", code),
		},
		AnnotationsJSON: map[string]string{
			"raw":         message,
			"operator":    "",
			"operateTime": time.Unix(now, 0).Format("2006-01-02 15:04:05"),
		},
	}

	event.FE2DB()
	if !queue.EventQueue.PushFront(event) {
		if recovered {
			logger.Warningf("pm recovered push_queue err: queue is full: %s", event.Hash)
		} else {
			logger.Warningf("pm event push_queue err: queue is full: %s", event.Hash)
		}
	}
}

// 已整合至 pushPM，移除 pushPMRecovered 包装

// StartPMPolling 以固定周期轮询调用 HandelPM
func StartPMPolling(alert aconf.Alert) {
	ips := alert.PM.IPs
	if len(ips) == 0 {
		logger.Warningf("PM 轮询已启用，但未配置 IP 列表")
	}
	for {
		for _, ip := range ips {
			HandelPM(ip)
		}
		time.Sleep(time.Duration(alert.PM.Interval) * time.Second)
	}
}
