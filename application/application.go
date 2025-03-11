package application

import (
	"fmt"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/ccfos/nightingale/v6/pushgw/router"
	"github.com/toolkits/pkg/logger"
	"time"
)

/*
target 表增加health_level健康度、alert_nums告警数、weight权重三个字段
busi_group表 增加health_level健康度、alert_nums字段
*/

func InitApplicationHealth(ctx *ctx.Context) {
	go LoopApplicationHealth(ctx)
	//LoopApplicationHealth(ctx)
}

func LoopApplicationHealth(ctx *ctx.Context) {
	for {
		err := UpdateAllTargetHealth(ctx)
		if err != nil {
			logger.Errorf("UpdateAllTargetHealth err: %v", err)
		}
		err = UpdateAllApplicationHealth(ctx)
		if err != nil {
			logger.Errorf("UpdateAllApplicationHealth err: %v", err)
		}
		err = WriteCurAlertCount(ctx)
		if err != nil {
			logger.Errorf("WriteCurAlertCount err: %v", err)
		}

		time.Sleep(time.Minute)
	}
}

func WriteCurAlertCount(ctx *ctx.Context) error {
	count, err := models.AlertCurEventGetCount(ctx)
	if err != nil {
		logger.Errorf("AlertCurEventGetCount err: %v", err)
		return err
	}
	err = WriteApplicationHealthTimeSeries("system_cur_alert_total", count)
	if err != nil {
		logger.Errorf("WriteApplicationHealthTimeSeries err: %v", err)
		return err
	}

	list, err := models.AlertCurEventCountGroupByGroupID(ctx)
	for _, item := range list {
		metric := fmt.Sprintf("application_alert_count_%d", item.GroupId)
		err = WriteApplicationHealthTimeSeries(metric, item.Count)
		if err != nil {
			logger.Errorf("WriteApplicationHealthTimeSeries err: %v", err)
			return err
		}
	}
	return nil
}

func UpdateAllApplicationHealth(ctx *ctx.Context) error {
	applicationList, err := models.BusiGroupGetAll(ctx)
	if err != nil {
		logger.Errorf("BusiGroupGetAll err: %v", err)
		return err
	}
	ApplicationHealthCount := 0
	for _, application := range applicationList {
		healthScore, alertNum, err := ComputeApplicationHealth(ctx, application.Id)
		if err != nil {
			logger.Errorf("ComputeApplicationHealth err: %v", err)
			continue
		}
		err = UpdateApplicationHealth(ctx, application.Id, healthScore, alertNum)
		if err != nil {
			logger.Errorf("UpdateApplicationHealth err: %v", err)
		}
		err = UpdateMidAndDBHealth(ctx, application.Id)
		if err != nil {
			logger.Errorf("UpdateMidAndDBHealth err: %v", err)
		}

		if healthScore >= 90 {
			ApplicationHealthCount += 1
		}
	}

	err = WriteApplicationHealthTimeSeries("application_health_count", ApplicationHealthCount)
	if err != nil {
		logger.Errorf("WriteApplicationHealthTimeSeries err: %v", err)
		return err
	}

	return nil
}

func UpdateApplicationHealth(ctx *ctx.Context, applicationID int64, healthScore float32, alertNum int) error {

	err := models.BusiGroupUpdateHealth(ctx, applicationID, healthScore, alertNum)
	if err != nil {
		logger.Errorf("BusiGroupUpdateHealth err: %v", err)
		return err
	}

	metricName := fmt.Sprintf("%s%v", "application_health_", applicationID)
	err = WriteApplicationHealthTimeSeries(metricName, healthScore)
	if err != nil {
		logger.Errorf("WriteApplicationHealthTimeSeries err: %v", err)
		return err
	}
	return nil
}

// UpdateMidAndDBHealth 记录该应用的中间件和数据库健康度
func UpdateMidAndDBHealth(ctx *ctx.Context, applicationID int64) error {
	err := ComputeAndWriteNodeTypeHealth(ctx, applicationID, "中间件")
	if err != nil {
		logger.Errorf("ComputeAndWriteNodeTypeHealth 中间件 err: %v", err)
		return err
	}
	err = ComputeAndWriteNodeTypeHealth(ctx, applicationID, "数据库")
	if err != nil {
		logger.Errorf("ComputeAndWriteNodeTypeHealth 数据库 err: %v", err)
		return err
	}
	return nil
}

func ComputeAndWriteNodeTypeHealth(ctx *ctx.Context, applicationID int64, nodeType string) error {

	dbHealthScore, err := ComputeNodeTypeHealth(ctx, applicationID, nodeType)
	if err != nil {
		logger.Errorf("ComputeNodeTypeHealth err: %v", err)
		return err
	}
	dbMetricName := fmt.Sprintf("%s%v", "application_mid_health_", applicationID)
	if nodeType == "数据库" {
		dbMetricName = fmt.Sprintf("%s%v", "application_db_health_", applicationID)
	}

	err = WriteApplicationHealthTimeSeries(dbMetricName, dbHealthScore)
	if err != nil {
		logger.Errorf("WriteApplicationHealthTimeSeries err: %v, dbMetricName: %s", err, dbMetricName)
		return err
	}
	return nil
}

func ComputeNodeTypeHealth(ctx *ctx.Context, applicationID int64, nodeType string) (float32, error) {
	var score float32 = 0
	targets, err := models.GetTargetsGroupIDAndType(ctx, applicationID, nodeType)
	if err != nil {
		logger.Errorf("GetTargetsGroupIDAndType err: %v", err)
		return 0, err
	}

	for _, target := range targets {
		score += target.HealthLevel
	}
	if len(targets) > 0 {
		score = score / float32(len(targets))
	} else {
		score = 100
	}
	return score, nil
}

//func UpdateGroupUsability() error {
//	// 设置 Prometheus 服务器地址
//	prometheusURL := "http://localhost:9090" // 请根据实际情况修改
//
//	client, err := api.NewClient(api.Config{
//		Address: prometheusURL,
//	})
//	if err != nil {
//		logger.Errorf("Error creating client: %v\n", err)
//	}
//
//	v1api := v1.NewAPI(client)
//
//	// 设置查询的时间范围，从1月1日到当前时间
//	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
//	currentTime := time.Now().UTC()
//
//	// 查询异常时间
//	query := `sum_over_time((health < 70)[1m])`
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//
//	queryRange := v1.Range{
//		Start: startDate,
//		End:   currentTime,
//		Step:  time.Minute,
//	}
//
//	result, warnings, err := v1api.QueryRange(ctx, query, queryRange)
//	if err != nil {
//		logger.Errorf("Error querying Prometheus: %v\n", err)
//	}
//	if len(warnings) > 0 {
//		log.Printf("Warnings: %v\n", warnings)
//	}
//
//	// 解析查询结果
//	exceptionMinutes := 0.0
//	if result.Type() == model.ValMatrix {
//		matrix := result.(model.Matrix)
//		for _, stream := range matrix {
//			for _, point := range stream.Values {
//				exceptionMinutes += float64(point.Value)
//			}
//		}
//	}
//
//	// 计算总时间
//	totalMinutes := currentTime.Sub(startDate).Minutes()
//
//	// 计算异常比例
//	availability := 1 - (exceptionMinutes / totalMinutes)
//
//	fmt.Printf("异常时间: %.0f 分钟\n", exceptionMinutes)
//	fmt.Printf("总时间: %.0f 分钟\n", totalMinutes)
//	fmt.Printf("应用可用性: %.2f%%\n", availability*100)
//	return nil
//}

func WriteApplicationHealthTimeSeries(metricName string, value interface{}) error {
	currentTime := time.Now().Unix() // 生成 Unix 时间戳（秒）
	err := router.RemoteWriteTimeSeries(metricName, value, currentTime)
	if err != nil {
		logger.Errorf("RemoteWriteTimeSeries err: %v", err)
		return err
	}

	return nil
}

func ComputeApplicationHealth(ctx *ctx.Context, applicationID int64) (float32, int, error) {
	var score float32 = 0
	alertNum := 0

	OrdinaryNodes, err := models.GetTargetsGroupIDAndWeight(ctx, applicationID, models.OrdinaryNode)
	if err != nil {
		logger.Errorf("GetTargetsGroupIDAndWeight err: %v", err)
		return 0, 0, err
	}
	KeyNodes, err := models.GetTargetsGroupIDAndWeight(ctx, applicationID, models.KeyNode)
	if err != nil {
		logger.Errorf("GetTargetsGroupIDAndWeight err: %v", err)
		return 0, 0, err
	}

	for _, target := range OrdinaryNodes {
		score += target.HealthLevel
		alertNum += target.AlertNum
	}
	if len(OrdinaryNodes) > 0 {
		score = score / float32(len(OrdinaryNodes))
	} else {
		score = 100
	}

	for _, target := range KeyNodes {
		score = score * (target.HealthLevel / 100)
		alertNum += target.AlertNum
	}

	return score, alertNum, nil
}
