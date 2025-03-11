package router

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/ccfos/nightingale/v6/prom"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/common/model"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

type getAppNodesRes struct {
	AllCount int        `json:"all_count"`
	MidCount int        `json:"mid_count"`
	DBCount  int        `json:"db_count"`
	NodesRes []nodesRes `json:"nodes_res"`
}

type nodesRes struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	CpuAvg string `json:"cpu_avg"`
	CpuMax string `json:"cpu_max"`
	MemAvg string `json:"mem_avg"`
	MemMax string `json:"mem_max"`
}

func (rt *Router) applicationSeriesMonth(c *gin.Context) {
	year := c.Query("year")
	month := c.Query("month")
	appName := c.Query("app_name")
	seriesType := c.Query("series_type") //application_health

	lst, err := GetMetricsSeriesOneMonth(rt.Ctx, rt.PromClients, year, month, appName, seriesType)
	if err != nil {
		logger.Errorf("GetMetricsSeriesOneMonth err: %v", err)
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(lst, err)
}

func GetMetricsSeriesOneMonth(ctx *ctx.Context, pc *prom.PromClientMap, year, month, appName, prefix string) (lst []model.Value, err error) {
	queryStr, err := GetAppHealthQuery(ctx, prefix, appName)
	if err != nil {
		logger.Errorf("GetAppHealthQuery err: %v", err)
		return
	}
	f, err := GetApplicationHealthSeriesReq(year, month, queryStr)
	if err != nil {
		logger.Errorf("GetApplicationHealthSeriesReq err: %v", err)
		return
	}

	lst, err = PromBatchQueryRange(pc, ctx, *f)
	if err != nil {
		logger.Errorf("PromBatchQueryRange err: %v", err)
		return
	}
	return
}

func GetAppHealthQuery(ctx *ctx.Context, prefix, appName string) (string, error) {
	group, err := models.BusiGroupGet(ctx, "name=?", appName)
	if err != nil {
		return "", err
	}
	if group == nil {
		return "", fmt.Errorf("应用不存在")
	}

	queryStr := fmt.Sprintf("%s_%d", prefix, group.Id)

	return queryStr, nil
}

func GetApplicationHealthSeriesReq(year, month, queryStr string) (*BatchQueryForm, error) {
	yearInt, err := strconv.Atoi(year)
	if err != nil {
		return nil, fmt.Errorf("参数year请传数字格式")
	}
	monthInt, err := strconv.Atoi(month)
	if err != nil {
		return nil, fmt.Errorf("参数month请传数字格式")
	}

	start, end, err := GetMonthStartAndEndTime(yearInt, monthInt)
	if err != nil {
		return nil, err
	}

	queries := make([]QueryFormItem, 0)
	query := QueryFormItem{
		Start: start,
		End:   end,
		Step:  10800,
		Query: queryStr,
	}
	queries = append(queries, query)

	return &BatchQueryForm{
		DatasourceId: 1,
		Queries:      queries,
	}, nil
}

func GetMonthStartAndEndTime(year int, month int) (int64, int64, error) {
	if month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("invalid month: %d", month)
	}

	// 获取指定月份的第一天
	startTime := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)

	// 获取当前时间
	now := time.Now()

	var endTime time.Time
	if year == now.Year() && month == int(now.Month()) {
		// 如果是当前月份，结束时间为当前时间
		endTime = now
	} else {
		// 否则获取该月的最后一天
		nextMonth := startTime.AddDate(0, 1, 0)
		endTime = nextMonth.Add(-time.Second)
	}

	return startTime.Unix(), endTime.Unix(), nil
}

func (rt *Router) getAppConnection(c *gin.Context) {
	appName := c.Query("app_name")

	group, err := models.BusiGroupGet(rt.Ctx, "name=?", appName)
	if err != nil {
		logger.Errorf("BusiGroupGet err: %v", err)
		ginx.NewRender(c).Message(err)
		return
	}
	if group == nil {
		logger.Errorf("BusiGroupGet 应用不存在")
		ginx.NewRender(c).Message("应用不存在")
		return
	}

	timeNow := fmt.Sprintf("%d", time.Now().UnixMilli())
	if group.AdServiceName == "" {
		ginx.NewRender(c).Message("应用未集成负责均衡，没有连接数")
		return
	}

	appConRes, err := getAppConnection(group.AdServiceName, timeNow)
	//appConRes, err := getAppConnectionTest(group.AdServiceName, timeNow)
	if err != nil {
		logger.Errorf("getAppConnection err: %v", err)
		ginx.NewRender(c).Message(err.Error())
		return
	}
	appConRes.Name = group.Name

	ginx.NewRender(c).Data(appConRes, err)
}

func getAppConnectionTest(name, timeNow string) (getAppConnectionRes, error) {
	appConRes := getAppConnectionRes{
		Name:      name,
		StartTime: 1730098829,
		StepTime:  1440,
		Values:    []int{23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56, 23, 45, 67, 22, 56},
	}
	return appConRes, nil
}

// resp, _, err := cli.Query(context.Background(), item.Query, time.Unix(item.Time, 0))
// if err != nil {
// return lst, err
// }
func getMetricQueryInstant(pc *prom.PromClientMap, query string) (model.Value, error) {
	var datasourceId int64 = 1
	cli := pc.GetCli(datasourceId)
	if cli == nil {
		logger.Warningf("no such datasource id: %d", datasourceId)
		return nil, fmt.Errorf("no such datasource id: %d", datasourceId)
	}
	timeNow := time.Now().Unix()
	resp, _, err := cli.Query(context.Background(), query, time.Unix(timeNow, 0))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (rt *Router) getAppNodesMetrics(c *gin.Context) {
	appName := c.Query("app_name")

	targets, err := getAllNodeByApp(rt.Ctx, appName)
	if err != nil {
		logger.Errorf("getAllNodeByApp err: %v", err)
		ginx.NewRender(c).Message(err)
		return
	}

	appNodesRes := getAppNodesRes{}
	midCount := 0
	dbCount := 0
	nodesArr := make([]nodesRes, 0)
	for _, target := range targets {
		if strings.Contains(target.Tags, "中间件") {
			midCount += 1
		}
		if strings.Contains(target.Tags, "数据库") {
			dbCount += 1
		}
		nodeRes, err := getNodeMetrics(target, rt.PromClients)
		if err != nil {
			logger.Errorf("getNodeMetrics err: %v", err)
			ginx.NewRender(c).Message(err)
		}
		nodesArr = append(nodesArr, nodeRes)
	}

	appNodesRes.AllCount = len(targets)
	appNodesRes.MidCount = midCount
	appNodesRes.DBCount = dbCount
	appNodesRes.NodesRes = nodesArr

	ginx.NewRender(c).Data(appNodesRes, err)
}

func getModeValue(item model.Value) string {
	if vector, ok := item.(model.Vector); ok {
		for _, sample := range vector {
			return fmt.Sprintf("%.1f", float64(sample.Value))
		}
	}
	return ""
}

func getNodeMetrics(target *models.Target, pc *prom.PromClientMap) (nodesRes, error) {
	nodeInfo := nodesRes{}
	nodeInfo.Name = target.Ident

	if strings.Contains(target.Tags, "中间件") {
		nodeInfo.Type = "中间件"
	}
	if strings.Contains(target.Tags, "数据库") {
		nodeInfo.Type = "数据库"
	}
	cpuAvgQuery := fmt.Sprintf("avg_over_time(cpu_usage_active{ident='%s'}[30d])", nodeInfo.Name)
	cpuMaxQuery := fmt.Sprintf("max_over_time(cpu_usage_active{ident='%s'}[30d])", target.Ident)
	memAvgQuery := fmt.Sprintf("avg_over_time(mem_used_percent{ident='%s'}[30d])", nodeInfo.Name)
	memMaxQuery := fmt.Sprintf("max_over_time(mem_used_percent{ident='%s'}[30d])", nodeInfo.Name)
	cpuAvg, err := getMetricQueryInstant(pc, cpuAvgQuery)
	if err != nil {
		logger.Errorf("getMetricQueryInstant cpuAvg err: %v", err)
		return nodeInfo, err
	}
	cpuMax, err := getMetricQueryInstant(pc, cpuMaxQuery)
	if err != nil {
		logger.Errorf("getMetricQueryInstant cpuMax err: %v", err)
		return nodeInfo, err
	}
	memAvg, err := getMetricQueryInstant(pc, memAvgQuery)
	if err != nil {
		logger.Errorf("getMetricQueryInstant memAvg err: %v", err)
		return nodeInfo, err
	}
	memMax, err := getMetricQueryInstant(pc, memMaxQuery)
	if err != nil {
		logger.Errorf("getMetricQueryInstant memMax err: %v", err)
		return nodeInfo, err
	}
	nodeInfo.CpuAvg = getModeValue(cpuAvg)
	nodeInfo.CpuMax = getModeValue(cpuMax)
	nodeInfo.MemAvg = getModeValue(memAvg)
	nodeInfo.MemMax = getModeValue(memMax)

	return nodeInfo, nil
}

func getAllNodeByApp(ctx *ctx.Context, appName string) ([]*models.Target, error) {
	group, err := models.BusiGroupGet(ctx, "name=?", appName)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, fmt.Errorf("应用不存在")
	}
	targetList, err := models.GetTargetsGroupID(ctx, group.Id)
	if err != nil {
		return nil, err
	}

	return targetList, nil
}
