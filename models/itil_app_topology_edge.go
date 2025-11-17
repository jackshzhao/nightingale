package models

import (
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type AppTopologyEdge struct {
	Id             int    `json:"id"`
	AppID          int    `json:"app_id"`
	EdgeID         string `json:"edge_id"`
	SourceNodeID   string `json:"source_node_id"`
	SourceNodePort string `json:"source_node_port"`
	TargetNodeID   string `json:"target_node_id"`
	TargetNodePort string `json:"target_node_port"`
}

func (u *AppTopologyEdge) TableName() string {
	return "app_topology_edge"
}

func (u *AppTopologyEdge) Add(ctx *ctx.Context) error {
	return Insert(ctx, u)
}

func EdgeDelByAppIDs(ctx *ctx.Context, appID int) error {
	return DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", appID).Delete(&AppTopologyEdge{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func GetEdgesByAppIDs(ctx *ctx.Context, appID int) ([]AppTopologyEdge, error) {
	var appTopologyEdges []AppTopologyEdge
	err := DB(ctx).Where("app_id = ?", appID).Find(&appTopologyEdges).Error
	if err != nil {
		return appTopologyEdges, errors.WithMessage(err, "failed to query AppTopologyEdge")
	}
	return appTopologyEdges, nil
}
