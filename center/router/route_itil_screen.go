package router

import (
	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

//	type AppTopology struct {
//		AppName         string                    `json:"app_name"`
//		AppID           int                       `json:"app_id"`
//		AppTopologyEdge []models.AppTopologyEdge  `json:"app_topology_edge"`
//		AppTopologyNode []*models.AppTopologyNode `json:"app_topology_node"`
//	}
type MonitorCount struct {
	ApCount     int64 `json:"app_count"`
	DeviceCount int64 `json:"device_count"`
}

// getAppHealthList ITIl大屏调用
func (rt *Router) getAppHealthList(c *gin.Context) {
	lst, err := models.BusiGroupSimpleGetAll(rt.Ctx)
	ginx.NewRender(c).Data(lst, err)
}

// getAppAndDeviceCount ITIl大屏调用
func (rt *Router) getAppAndDeviceCount(c *gin.Context) {

	appCount, err := models.BusiGroupSimpleCount(rt.Ctx)
	if err != nil {
		logger.Errorf("itil BusiGroupSimpleCount err: %v", err)
		ginx.NewRender(c).Message(err)
		return
	}

	targetCount, err := models.TargetTotalCount(rt.Ctx)
	if err != nil {
		logger.Errorf("itil TargetTotalCount err: %v", err)
		ginx.NewRender(c).Message(err)
		return
	}

	res := MonitorCount{
		ApCount:     appCount,
		DeviceCount: targetCount,
	}
	ginx.NewRender(c).Data(res, err)
}
