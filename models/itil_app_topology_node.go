package models

import (
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type AppTopologyNode struct {
	Id           int     `json:"id"`
	AppID        int     `json:"app_id"`
	NodeID       string  `json:"node_id"`
	NodeType     string  `json:"node_type"`
	NodeShape    string  `json:"node_shape"`
	PositionX    int     `json:"position_x"`
	PositionY    int     `json:"position_y"`
	SizeWidth    int     `json:"size_width"`
	SizeHeight   int     `json:"size_height"`
	NodeName     string  `json:"node_name"`
	TargetID     int     `json:"target_id"`
	TargetIP     string  `gorm:"->" json:"target_ip"`
	TargetHealth float32 `gorm:"->" json:"target_health"`
	TargetIdent  string  `gorm:"->" json:"target_ident"`
	PortTopID    string  `json:"port_top_id"`
	PortBottomID string  `json:"port_bottom_id"`
	PortLeftID   string  `json:"port_left_id"`
	PortRightID  string  `json:"port_right_id"`
}

func (u *AppTopologyNode) TableName() string {
	return "app_topology_node"
}

func (u *AppTopologyNode) Add(ctx *ctx.Context) error {
	return Insert(ctx, u)
}

func NodeDelByAppIDs(ctx *ctx.Context, appID int) error {
	return DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", appID).Delete(&AppTopologyNode{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func GetNodesByAppIDs(ctx *ctx.Context, appID int) ([]*AppTopologyNode, error) {
	var appTopologyNodes []*AppTopologyNode
	err := DB(ctx).Where("app_id = ?", appID).Find(&appTopologyNodes).Error
	if err != nil {
		return appTopologyNodes, errors.WithMessage(err, "failed to query AppTopologyNode")
	}
	return appTopologyNodes, nil
}
