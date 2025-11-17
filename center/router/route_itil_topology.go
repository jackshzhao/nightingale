package router

import (
	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/errorx"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
	"net/http"
	"strconv"
)

type AppTopology struct {
	AppName         string                    `json:"app_name"`
	AppID           int                       `json:"app_id"`
	AppTopologyEdge []models.AppTopologyEdge  `json:"app_topology_edge"`
	AppTopologyNode []*models.AppTopologyNode `json:"app_topology_node"`
}

func (rt *Router) saveAppTopology(c *gin.Context) {
	var appTopology *AppTopology
	ginx.BindJSON(c, &appTopology)

	if err := models.NodeDelByAppIDs(rt.Ctx, appTopology.AppID); err != nil {
		logger.Errorf("NodeDelByAppIDs app_id: %v, err: %v", appTopology.AppID, err)
		ginx.NewRender(c).Message(err)
		return
	}
	if err := models.EdgeDelByAppIDs(rt.Ctx, appTopology.AppID); err != nil {
		logger.Errorf("EdgeDelByAppIDs app_id: %v, err: %v", appTopology.AppID, err)
		ginx.NewRender(c).Message(err)
		return
	}

	for _, item := range appTopology.AppTopologyEdge {
		item.AppID = appTopology.AppID
		if err := item.Add(rt.Ctx); err != nil {
			logger.Errorf("edge add app_id: %v, err: %v", appTopology.AppID, err)
			ginx.NewRender(c).Message(err)
			return
		}
	}

	for _, item := range appTopology.AppTopologyNode {
		item.AppID = appTopology.AppID
		if err := item.Add(rt.Ctx); err != nil {
			logger.Errorf("node add app_id: %v, err: %v", appTopology.AppID, err)
			ginx.NewRender(c).Message(err)
			return
		}
	}

	ginx.NewRender(c).Message(nil)
}

func (rt *Router) getAppTopology(c *gin.Context) {
	appIDStr := c.Query("app_id")
	if appIDStr == "" {
		logger.Errorf("app_id为空")
		ginx.NewRender(c).Message("app_id为空")
		return
	}
	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		errorx.Bomb(http.StatusBadRequest, "cannot convert [%s] to int", appIDStr)
	}

	group, err := models.BusiGroupGet(rt.Ctx, "id = ?", appID)

	edges, err := models.GetEdgesByAppIDs(rt.Ctx, appID)
	if err != nil {
		logger.Errorf("GetEdgesByAppIDs app_id: %v, err: %v", appID, err)
		ginx.NewRender(c).Message(err)
		return
	}

	nodes, err := models.GetNodesByAppIDs(rt.Ctx, appID)
	if err != nil {
		logger.Errorf("GetNodesByAppIDs app_id: %v, err: %v", appID, err)
		ginx.NewRender(c).Message(err)
		return
	}
	for _, node := range nodes {
		if node.TargetID == 0 {
			continue
		}
		target, err := models.GetTargetByID(rt.Ctx, node.TargetID)
		if err != nil {
			logger.Errorf("GetTargetByID id: %v, err: %v", node.TargetID, err)
			ginx.NewRender(c).Message(err)
			return
		}
		node.TargetIdent = target.Ident
		node.TargetIP = target.HostIp
		node.TargetHealth = target.HealthLevel
	}
	res := AppTopology{
		AppName:         group.Name,
		AppID:           appID,
		AppTopologyEdge: edges,
		AppTopologyNode: nodes,
	}

	ginx.NewRender(c).Data(res, err)
}
