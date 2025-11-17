package router

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
	"io"
	"net/http"
	"time"
)

const (
	adServer              = "https://10.11.11.19"
	adVirtualServiceItems = "/api/lb/current-version/stat/slb/virtual-service"
	adVirtualServicePath  = "/api/lb/current-version/stat/slb/virtual-service/"
	adPoolItems           = "/api/lb/current-version/stat/slb/pool"
	adPoolPath            = "/api/lb/current-version/stat/slb/pool/"
)

type getAppItemsRes struct {
	Items       []AppItem `json:"items"`
	TotalPages  int       `json:"total_pages"`
	PageNumber  int       `json:"page_number"`
	PageSize    int       `json:"page_size"`
	TotalItems  int       `json:"total_items"`
	ItemsOffset int       `json:"items_offset"`
	ItemsLength int       `json:"items_length"`
}

type AppItem struct {
	Name string `json:"name"`
}

type getAppConnectionRes struct {
	Name      string `json:"name"`
	StartTime int    `json:"start_time"`
	Timestamp int    `json:"timestamp"`
	StepTime  int    `json:"step_time"`
	Values    []int  `json:"values"`
}

func (rt *Router) getAllAppsConnectionBak(c *gin.Context) {
	appConArr := make([]getAppConnectionRes, 0)
	appConRes := getAppConnectionRes{
		Name:      "测试应用",
		StartTime: 1730597760,
		StepTime:  1440,
		Values:    []int{23, 45, 67, 22, 56},
	}
	appConArr = append(appConArr, appConRes)
	appConRes2 := getAppConnectionRes{
		Name:   "测试应用2",
		Values: []int{2, 4, 8, 1, 8},
	}
	appConArr = append(appConArr, appConRes2)
	appConRes3 := getAppConnectionRes{
		Name:   "测试应用3",
		Values: []int{675, 345, 467, 567, 455},
	}
	appConArr = append(appConArr, appConRes3)
	appConRes4 := getAppConnectionRes{
		Name:   "测试应用4",
		Values: []int{123, 222, 145, 245, 123},
	}
	appConArr = append(appConArr, appConRes4)
	appConRes5 := getAppConnectionRes{
		Name:   "测试应用5",
		Values: []int{123, 145, 167, 122, 156},
	}
	appConArr = append(appConArr, appConRes5)
	appConRes6 := getAppConnectionRes{
		Name:   "测试应用6",
		Values: []int{223, 245, 267, 222, 256},
	}
	appConArr = append(appConArr, appConRes6)
	appConRes7 := getAppConnectionRes{
		Name:   "测试应用7",
		Values: []int{323, 345, 367, 322, 356},
	}
	appConArr = append(appConArr, appConRes7)
	appConRes8 := getAppConnectionRes{
		Name:   "测试应用8",
		Values: []int{454, 545, 433, 432, 123},
	}
	appConArr = append(appConArr, appConRes8)
	appConRes9 := getAppConnectionRes{
		Name:   "测试应用9",
		Values: []int{32, 43, 77, 90, 76},
	}
	appConArr = append(appConArr, appConRes9)
	appConRes10 := getAppConnectionRes{
		Name:   "测试应用10",
		Values: []int{23, 34, 67, 54, 65},
	}
	appConArr = append(appConArr, appConRes10)
	ginx.NewRender(c).Data(appConArr, nil)
}

func (rt *Router) getAllAppsConnection(c *gin.Context) {

	appItems, err := getAllApps()
	if err != nil {
		logger.Errorf("getAllApps err: %v", err)
		ginx.NewRender(c).Message(err.Error())
		return
	}

	appConArr := make([]getAppConnectionRes, 0)

	timeNow := fmt.Sprintf("%d", time.Now().UnixMilli())
	for _, item := range appItems.Items {
		appConRes, err := getAppConnection(item.Name, timeNow)
		if err != nil {
			logger.Errorf("getAppConnection err: %v", err)
			ginx.NewRender(c).Message(err.Error())
			return
		}
		appConRes.Name = item.Name
		appConArr = append(appConArr, appConRes)
	}

	ginx.NewRender(c).Data(appConArr, err)
}

func getAllApps() (*getAppItemsRes, error) {
	url := adServer + adVirtualServiceItems
	resBody, err := sendHttp("GET", url)
	if err != nil {
		logger.Errorf("getAllApps sendHttp err: %v", err)
		return nil, err
	}
	appItems := &getAppItemsRes{}
	json.Unmarshal(resBody, appItems)
	return appItems, nil
}

func getAppConnection(name, timeNow string) (getAppConnectionRes, error) {
	res := getAppConnectionRes{}
	url := adServer + adVirtualServicePath + name + "/server_connection?trend=last-month&netns=default&all_properties=true&_dc=" + timeNow
	resBody, err := sendHttp("GET", url)
	if err != nil {
		logger.Errorf("sendHttp err: %v", err)
		return res, err
	}
	json.Unmarshal(resBody, &res)
	return res, nil
}

func sendHttp(method, url string) ([]byte, error) {
	// 创建HTTP客户端并忽略证书校验
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		logger.Errorf("Error creating request: %v", err)
		return nil, err
	}

	// 设置Basic Auth
	req.SetBasicAuth("itil_monitor", "7ujm8ik,(OL>")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Error making request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 读取并打印响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("Error reading response body: %v", err)
		return nil, err
	}

	return body, nil
}
