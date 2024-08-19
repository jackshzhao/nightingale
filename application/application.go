package application

import (
	"fmt"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/ccfos/nightingale/v6/pushgw/router"
	"log"
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
			log.Fatalf("UpdateAllTargetHealth err: %v", err)
		}
		err = UpdateAllApplicationHealth(ctx)
		if err != nil {
			log.Fatalf("UpdateAllApplicationHealth err: %v", err)
		}
		time.Sleep(time.Minute)
	}
}

func UpdateAllApplicationHealth(ctx *ctx.Context) error {
	applicationList, err := models.BusiGroupGetAll(ctx)
	if err != nil {
		log.Fatalf("BusiGroupGetAll err: %v", err)
		return err
	}
	ApplicationHealthCount := 0
	for _, application := range applicationList {
		healthScore, alertNum, err := ComputeApplicationHealth(ctx, application.Id)
		if err != nil {
			log.Fatalf("ComputeApplicationHealth err: %v", err)
			continue
		}
		err = UpdateApplicationHealth(ctx, application.Id, healthScore, alertNum)
		if err != nil {
			log.Fatalf("UpdateApplicationHealth err: %v", err)
		}
		if healthScore >= 90 {
			ApplicationHealthCount += 1
		}
	}

	err = WriteApplicationHealthTimeSeries("count", ApplicationHealthCount)
	if err != nil {
		log.Fatalf("WriteApplicationHealthTimeSeries err: %v", err)
		return err
	}

	return nil
}

func UpdateApplicationHealth(ctx *ctx.Context, applicationID int64, healthScore float32, alertNum int) error {

	err := models.BusiGroupUpdateHealth(ctx, applicationID, healthScore, alertNum)
	if err != nil {
		log.Fatalf("BusiGroupUpdateHealth err: %v", err)
		return err
	}

	err = WriteApplicationHealthTimeSeries(applicationID, healthScore)
	if err != nil {
		log.Fatalf("WriteApplicationHealthTimeSeries err: %v", err)
		return err
	}
	return nil
}

//func UpdateGroupUsability() error {
//	// 设置 Prometheus 服务器地址
//	prometheusURL := "http://localhost:9090" // 请根据实际情况修改
//
//	client, err := api.NewClient(api.Config{
//		Address: prometheusURL,
//	})
//	if err != nil {
//		log.Fatalf("Error creating client: %v\n", err)
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
//		log.Fatalf("Error querying Prometheus: %v\n", err)
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

func WriteApplicationHealthTimeSeries(name, value interface{}) error {
	metricName := fmt.Sprintf("%s%v", "application_health_", name)
	currentTime := time.Now().Unix() // 生成 Unix 时间戳（秒）
	err := router.RemoteWriteTimeSeries(metricName, value, currentTime)
	if err != nil {
		log.Fatalf("RemoteWriteTimeSeries err: %v", err)
		return err
	}

	return nil
}

func ComputeApplicationHealth(ctx *ctx.Context, applicationID int64) (float32, int, error) {
	var score float32 = 0
	alertNum := 0

	OrdinaryNodes, err := models.GetTargetsGroupIDAndWeight(ctx, applicationID, models.OrdinaryNode)
	if err != nil {
		log.Fatalf("GetTargetsGroupIDAndWeight err: %v", err)
		return 0, 0, err
	}
	KeyNodes, err := models.GetTargetsGroupIDAndWeight(ctx, applicationID, models.KeyNode)
	if err != nil {
		log.Fatalf("GetTargetsGroupIDAndWeight err: %v", err)
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
