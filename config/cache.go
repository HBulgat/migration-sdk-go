package config

import (
	"log"
	"slices"
	"sync"
	"time"

	"github.com/HBulgat/migration-sdk-go/constdef"
	"github.com/HBulgat/migration-sdk-go/gray"
)

type CachedConfigClient struct { // extend ConfigClient
	delegate               ConfigClient
	StatusCache            sync.Map
	RulesCache             sync.Map
	Lock                   sync.RWMutex
	RegisteredKeys         []string
	RefreshIntervalSeconds int32
}

func NewCachedConfigClient(client ConfigClient, RefreshIntervalSeconds int32) *CachedConfigClient {
	c := &CachedConfigClient{
		delegate:               client,
		RefreshIntervalSeconds: RefreshIntervalSeconds,
		RegisteredKeys:         make([]string, 0),
	}
	c.schedule()
	return c
}

func (c *CachedConfigClient) exists(migrationKey string) bool {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	return slices.Contains(c.RegisteredKeys, migrationKey)
}

func (c *CachedConfigClient) RegistryKey(migrationKey string) {
	if c.exists(migrationKey) {
		return
	}
	c.Lock.Lock()
	defer c.Lock.Unlock()
	c.RegisteredKeys = append(c.RegisteredKeys, migrationKey)
}

func (c *CachedConfigClient) GetStatus(migrationKey string) (constdef.MigrationTaskStatus, error) {
	c.RegistryKey(migrationKey)
	if val, ok := c.StatusCache.Load(migrationKey); ok {
		if status, ok := val.(constdef.MigrationTaskStatus); ok {
			return status, nil
		}
	}
	status, err := c.delegate.GetStatus(migrationKey)
	if err != nil {
		return 0, err
	}

	c.StatusCache.Store(migrationKey, status)
	return status, nil
}

func (c *CachedConfigClient) GetGrayRules(migrationKey string) ([]gray.Rule, error) {
	if !c.exists(migrationKey) {
		c.RegistryKey(migrationKey)
	}
	if val, ok := c.RulesCache.Load(migrationKey); ok {
		// 类型断言
		if rules, ok := val.([]gray.Rule); ok {
			return rules, nil
		}
	}
	rules, err := c.delegate.GetGrayRules(migrationKey)
	if err != nil {
		return nil, err
	}

	c.RulesCache.Store(migrationKey, rules)
	return rules, nil
}

func (c *CachedConfigClient) schedule() {
	go func() {
		ticker := time.NewTicker(time.Duration(c.RefreshIntervalSeconds) * time.Second)
		for range ticker.C {
			log.Println("cron to update cache")
			for _, key := range c.RegisteredKeys {
				log.Printf("updating cache %s", key)
				status, err1 := c.delegate.GetStatus(key)
				log.Printf("updating: migration task status:%v\n", status)
				if err1 != nil {
					log.Printf("error getting status: %v", err1)
				} else {
					c.StatusCache.Store(key, status)
				}
				rules, err2 := c.delegate.GetGrayRules(key)
				log.Printf("updating: migration gray rules:%v\n", rules)
				if err2 != nil {
					log.Printf("error getting gray rules: %v", err2)
				} else {
					c.RulesCache.Store(key, rules)
				}
			}
		}
	}()
}
