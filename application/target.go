package application

import (
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"log"
)

func UpdateAllTargetHealth(ctx *ctx.Context) error {
	allTargets, err := models.GetAllTargets(ctx)
	if err != nil {
		log.Fatalf("GetAllTargets err: %v", err)
		return err
	}

	for _, target := range allTargets {
		score, alertNum, err := computeTargetHealth(ctx, target.Ident)
		if err != nil {
			log.Fatalf("computeTargetHealth err: %v", err)
		}
		err = models.TargetUpdateHealth(ctx, target.Id, alertNum, score)
		if err != nil {
			log.Fatalf("TargetUpdateHealth err: %v", err)
		}
	}

	return nil
}

func computeTargetHealth(ctx *ctx.Context, targetIdent string) (float32, int, error) {
	score := float32(100)
	alerts, err := models.AlertCurEventGetByIdent(ctx, targetIdent)
	if err != nil {
		log.Fatalf("AlertCurEventGetByIdent err: %v", err)
		return score, 0, err
	}

	for _, alert := range alerts {
		if alert.Severity == 1 {
			score -= models.EmergencyScore
		} else if alert.Severity == 2 {
			score -= models.WarningScore
		} else if alert.Severity == 3 {
			score -= models.NoticeScore
		}
	}

	return score, len(alerts), nil
}
