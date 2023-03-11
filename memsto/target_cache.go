package memsto

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/ccfos/nightingale/v6/storage"

	"github.com/pkg/errors"
	"github.com/toolkits/pkg/logger"
)

// 1. append note to alert_event
// 2. append tags to series
type TargetCacheType struct {
	statTotal       int64
	statLastUpdated int64
	ctx             *ctx.Context
	stats           *Stats
	redis           storage.Redis

	sync.RWMutex
	targets map[string]*models.Target // key: ident
}

func NewTargetCache(ctx *ctx.Context, stats *Stats, redis storage.Redis) *TargetCacheType {
	tc := &TargetCacheType{
		statTotal:       -1,
		statLastUpdated: -1,
		ctx:             ctx,
		stats:           stats,
		redis:           redis,
		targets:         make(map[string]*models.Target),
	}

	tc.SyncTargets()
	return tc
}

func (tc *TargetCacheType) Reset() {
	tc.Lock()
	defer tc.Unlock()

	tc.statTotal = -1
	tc.statLastUpdated = -1
	tc.targets = make(map[string]*models.Target)
}

func (tc *TargetCacheType) StatChanged(total, lastUpdated int64) bool {
	if tc.statTotal == total && tc.statLastUpdated == lastUpdated {
		return false
	}

	return true
}

func (tc *TargetCacheType) Set(m map[string]*models.Target, total, lastUpdated int64) {
	tc.Lock()
	tc.targets = m
	tc.Unlock()

	// only one goroutine used, so no need lock
	tc.statTotal = total
	tc.statLastUpdated = lastUpdated
}

func (tc *TargetCacheType) Get(ident string) (*models.Target, bool) {
	tc.RLock()
	defer tc.RUnlock()
	val, has := tc.targets[ident]
	return val, has
}

func (tc *TargetCacheType) GetMissHost(targets []*models.Target, ts int64) []string {
	tc.RLock()
	defer tc.RUnlock()
	var missHosts []string
	for _, target := range targets {
		target, exists := tc.targets[target.Ident]
		if !exists {
			missHosts = append(missHosts, target.Ident)
			continue
		}
		if target.UnixTime < ts {
			missHosts = append(missHosts, target.Ident)
		}
	}

	return missHosts
}

func (tc *TargetCacheType) GetOffsetHost(targets []*models.Target, ts int64) []string {
	tc.RLock()
	defer tc.RUnlock()
	var hosts []string
	for _, target := range targets {
		target, exists := tc.targets[target.Ident]
		if !exists {
			continue
		}

		if target.Offset > ts {
			hosts = append(hosts, target.Ident)
		}
	}

	return hosts
}

func (tc *TargetCacheType) SyncTargets() {
	err := tc.syncTargets()
	if err != nil {
		log.Fatalln("failed to sync targets:", err)
	}

	go tc.loopSyncTargets()
}

func (tc *TargetCacheType) loopSyncTargets() {
	duration := time.Duration(9000) * time.Millisecond
	for {
		time.Sleep(duration)
		if err := tc.syncTargets(); err != nil {
			logger.Warning("failed to sync targets:", err)
		}
	}
}

func (tc *TargetCacheType) syncTargets() error {
	start := time.Now()

	stat, err := models.TargetStatistics(tc.ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to call TargetStatistics")
	}

	if !tc.StatChanged(stat.Total, stat.LastUpdated) {
		tc.stats.GaugeCronDuration.WithLabelValues("sync_targets").Set(0)
		tc.stats.GaugeSyncNumber.WithLabelValues("sync_targets").Set(0)
		logger.Debug("targets not changed")
		return nil
	}

	lst, err := models.TargetGetsAll(tc.ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to call TargetGetsAll")
	}

	metaMap := tc.GetHostMetas(lst)

	m := make(map[string]*models.Target)
	for i := 0; i < len(lst); i++ {
		lst[i].FillTagsMap()
		if meta, ok := metaMap[lst[i].Ident]; ok {
			lst[i].FillMeta(meta)
		}
		m[lst[i].Ident] = lst[i]
	}

	tc.Set(m, stat.Total, stat.LastUpdated)

	ms := time.Since(start).Milliseconds()
	tc.stats.GaugeCronDuration.WithLabelValues("sync_targets").Set(float64(ms))
	tc.stats.GaugeSyncNumber.WithLabelValues("sync_targets").Set(float64(len(lst)))
	logger.Infof("timer: sync targets done, cost: %dms, number: %d", ms, len(lst))

	return nil
}

func (tc *TargetCacheType) GetHostMetas(targets []*models.Target) map[string]*models.HostMeta {
	metaMap := make(map[string]*models.HostMeta)
	if tc.redis == nil {
		return metaMap
	}
	var metas []*models.HostMeta
	num := 0
	var keys []string
	for i := 0; i < len(targets); i++ {
		keys = append(keys, targets[i].Ident)
		num++
		if num == 100 {
			vals := tc.redis.MGet(context.Background(), keys...).Val()
			for _, value := range vals {
				var meta models.HostMeta
				if value == nil {
					continue
				}
				err := json.Unmarshal([]byte(value.(string)), &meta)
				if err != nil {
					logger.Errorf("failed to unmarshal host meta: %s value:%v", err, value)
					continue
				}
				metaMap[meta.Hostname] = &meta
			}
			keys = keys[:0]
			metas = metas[:0]
			num = 0
		}
	}

	vals := tc.redis.MGet(context.Background(), keys...).Val()
	for _, value := range vals {
		var meta models.HostMeta
		if value == nil {
			continue
		}
		err := json.Unmarshal([]byte(value.(string)), &meta)
		if err != nil {
			continue
		}
		metaMap[meta.Hostname] = &meta
	}

	return metaMap
}