package router

import (
	"context"
	"crypto/tls"
	"fmt"

	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	pkgprom "github.com/ccfos/nightingale/v6/pkg/prom"
	"github.com/ccfos/nightingale/v6/prom"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/common/model"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

type QueryFormItem struct {
	Start int64  `json:"start" binding:"required"`
	End   int64  `json:"end" binding:"required"`
	Step  int64  `json:"step" binding:"required"`
	Query string `json:"query" binding:"required"`
}

type BatchQueryForm struct {
	DatasourceId int64           `json:"datasource_id" binding:"required"`
	Queries      []QueryFormItem `json:"queries" binding:"required"`
}

func (rt *Router) promBatchQueryRange(c *gin.Context) {
	var f BatchQueryForm
	ginx.Dangerous(c.BindJSON(&f))

	lst, err := PromBatchQueryRange(rt.PromClients, rt.Ctx, f)
	ginx.NewRender(c).Data(lst, err)
}

func PromBatchQueryRange(pc *prom.PromClientMap, ctx *ctx.Context, f BatchQueryForm) ([]model.Value, error) {
	var lst []model.Value

	cli := pc.GetCli(f.DatasourceId)
	if cli == nil {
		return lst, fmt.Errorf("no such datasource id: %d", f.DatasourceId)
	}

	for _, item := range f.Queries {
		if item.Query == "system_cur_alert_total" { //获取当前活跃告警数量
			count, err := models.AlertCurEventGetCount(ctx)
			res := model.Matrix{
				&model.SampleStream{
					Metric: model.Metric{},
					Values: []model.SamplePair{model.SamplePair{
						Timestamp: 1,
						Value:     model.SampleValue(count),
					}},
				},
			}
			lst = append(lst, res)
			return lst, err
		}

		r := pkgprom.Range{
			Start: time.Unix(item.Start, 0),
			End:   time.Unix(item.End, 0),
			Step:  time.Duration(item.Step) * time.Second,
		}

		resp, _, err := cli.QueryRange(context.Background(), item.Query, r)
		if err != nil {
			return lst, err
		}

		lst = append(lst, resp)
	}
	return lst, nil
}

type BatchInstantForm struct {
	DatasourceId int64             `json:"datasource_id" binding:"required"`
	Queries      []InstantFormItem `json:"queries" binding:"required"`
}

type InstantFormItem struct {
	Time  int64  `json:"time" binding:"required"`
	Query string `json:"query" binding:"required"`
}

type MetricSimRes struct {
	MS    MetricSimple `json:"metric"`
	Value []int        `json:"value"`
}

type MetricSimple struct {
	Name    string `json:"__name__"`
	Ident   string `json:"ident"`
	Method  string `json:"method"`
	Product string `json:"product"`
	Target  string `json:"target"`
}

type AppHttpRes struct {
	Dat []AppHttpInfo `json:"dat"`
	Err string        `json:"err"`
}

type AppHttpInfo struct {
	Target       string  `json:"target"`
	ResultCode   float64 `json:"result_code"`
	ResponseCode float64 `json:"response_code"`
	ResponseTime float64 `json:"response_time"`
}

func (rt *Router) promQueryHttp(c *gin.Context) {
	appName := c.Query("app_name")
	//var f BatchInstantForm
	time := time.Now().Unix()
	resultCodeQuery := fmt.Sprintf("http_response_result_code{product='%s'}", appName)
	responseCodeQuery := fmt.Sprintf("http_response_response_code{product='%s'}", appName)
	responseTimeQuery := fmt.Sprintf("http_response_response_time{product='%s'}", appName)
	queries := []InstantFormItem{
		{
			Time:  time,
			Query: resultCodeQuery,
		},
		{
			Time:  time,
			Query: responseCodeQuery,
		},
		{
			Time:  time,
			Query: responseTimeQuery,
		},
	}
	f := BatchInstantForm{
		DatasourceId: 1,
		Queries:      queries,
	}

	metricArr := make([]AppHttpInfo, 0)
	appHttpInfo := AppHttpInfo{}

	lst, err := PromBatchQueryInstant(rt.PromClients, f)
	for _, vector := range lst {
		vectorResult, ok := vector.(model.Vector)
		if !ok {
			logger.Errorf("Result is not a Vector type")
		}
		if len(vectorResult) > 0 {
			metric := vectorResult[0].Metric.String()

			appHttpInfo.Target = FindTarget(metric)
			if strings.Contains(metric, "http_response_result_code") {
				appHttpInfo.ResultCode = float64(vectorResult[0].Value)
			}
			if strings.Contains(metric, "http_response_response_code") {
				appHttpInfo.ResponseCode = float64(vectorResult[0].Value)
			}
			if strings.Contains(metric, "http_response_response_time") {
				appHttpInfo.ResponseTime = float64(vectorResult[0].Value) * 1000
			}
		}
	}
	if appHttpInfo.Target != "" {
		metricArr = append(metricArr, appHttpInfo)
	}

	ginx.NewRender(c).Data(metricArr, err)
}

func FindTarget(str string) string {
	re := regexp.MustCompile(`target="([^"]+)"`)

	// 使用正则表达式查找匹配项
	match := re.FindStringSubmatch(str)
	if len(match) > 1 {
		target := match[1]
		return target
	}
	logger.Errorf("Target value not found, str = %s", str)
	return ""
}

func (rt *Router) promBatchQueryInstant(c *gin.Context) {
	var f BatchInstantForm
	ginx.Dangerous(c.BindJSON(&f))

	lst, err := PromBatchQueryInstant(rt.PromClients, f)
	ginx.NewRender(c).Data(lst, err)
}

func PromBatchQueryInstant(pc *prom.PromClientMap, f BatchInstantForm) ([]model.Value, error) {
	var lst []model.Value

	cli := pc.GetCli(f.DatasourceId)
	if cli == nil {
		logger.Warningf("no such datasource id: %d", f.DatasourceId)
		return lst, fmt.Errorf("no such datasource id: %d", f.DatasourceId)
	}

	for _, item := range f.Queries {
		resp, _, err := cli.Query(context.Background(), item.Query, time.Unix(item.Time, 0))
		if err != nil {
			return lst, err
		}

		lst = append(lst, resp)
	}
	return lst, nil
}

func (rt *Router) dsProxy(c *gin.Context) {
	dsId := ginx.UrlParamInt64(c, "id")
	ds := rt.DatasourceCache.GetById(dsId)

	if ds == nil {
		c.String(http.StatusBadRequest, "no such datasource")
		return
	}

	target, err := url.Parse(ds.HTTPJson.Url)
	if err != nil {
		c.String(http.StatusInternalServerError, "invalid  url: %s", ds.HTTPJson.Url)
		return
	}

	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		req.Header.Set("Host", target.Host)

		// fe request e.g. /api/n9e/proxy/:id/*
		arr := strings.Split(req.URL.Path, "/")
		if len(arr) < 6 {
			c.String(http.StatusBadRequest, "invalid url path")
			return
		}

		req.URL.Path = strings.TrimRight(target.Path, "/") + "/" + strings.Join(arr[5:], "/")
		if target.RawQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = target.RawQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = target.RawQuery + "&" + req.URL.RawQuery
		}

		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "")
		}

		if ds.AuthJson.BasicAuthUser != "" {
			req.SetBasicAuth(ds.AuthJson.BasicAuthUser, ds.AuthJson.BasicAuthPassword)
		}

		headerCount := len(ds.HTTPJson.Headers)
		if headerCount > 0 {
			for key, value := range ds.HTTPJson.Headers {
				req.Header.Set(key, value)
				if key == "Host" {
					req.Host = value
				}
			}
		}
	}

	errFunc := func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}

	transport, has := transportGet(dsId, ds.UpdatedAt)
	if !has {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: ds.HTTPJson.TLS.SkipTlsVerify},
			Proxy:           http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: time.Duration(ds.HTTPJson.DialTimeout) * time.Millisecond,
			}).DialContext,
			ResponseHeaderTimeout: time.Duration(ds.HTTPJson.Timeout) * time.Millisecond,
			MaxIdleConnsPerHost:   ds.HTTPJson.MaxIdleConnsPerHost,
		}
		transportPut(dsId, ds.UpdatedAt, transport)
	}

	modifyResponse := func(r *http.Response) error {
		if r.StatusCode == http.StatusUnauthorized {
			logger.Warningf("proxy path:%s unauthorized access ", c.Request.URL.Path)
			return fmt.Errorf("unauthorized access")
		}

		return nil
	}

	proxy := &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		ErrorHandler:   errFunc,
		ModifyResponse: modifyResponse,
	}

	proxy.ServeHTTP(c.Writer, c.Request)

}

var (
	transports     = map[int64]http.RoundTripper{}
	updatedAts     = map[int64]int64{}
	transportsLock = &sync.Mutex{}
)

func transportGet(dsid, newUpdatedAt int64) (http.RoundTripper, bool) {
	transportsLock.Lock()
	defer transportsLock.Unlock()

	tran, has := transports[dsid]
	if !has {
		return nil, false
	}

	oldUpdateAt, has := updatedAts[dsid]
	if !has {
		oldtran := tran.(*http.Transport)
		oldtran.CloseIdleConnections()
		delete(transports, dsid)
		return nil, false
	}

	if oldUpdateAt != newUpdatedAt {
		oldtran := tran.(*http.Transport)
		oldtran.CloseIdleConnections()
		delete(transports, dsid)
		delete(updatedAts, dsid)
		return nil, false
	}

	return tran, has
}

func transportPut(dsid, updatedat int64, tran http.RoundTripper) {
	transportsLock.Lock()
	transports[dsid] = tran
	updatedAts[dsid] = updatedat
	transportsLock.Unlock()
}
