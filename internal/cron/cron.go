// Package cron 提供定时任务支持。
// 使用 robfig/cron/v3 实现真正的 cron 表达式调度。
package cron

import (
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/super1207/langlang-go/internal/log"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	mu      sync.Mutex
	running bool
	jobs    []*Job
	cron    *cron.Cron
	entries map[string]cron.EntryID
}

// Job 定时任务
type Job struct {
	Name     string
	Spec     string // cron 表达式
	Func     func() error
	Disabled bool
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		entries: make(map[string]cron.EntryID),
	}
}

// AddJob 添加定时任务
func (s *Scheduler) AddJob(name, spec string, fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, &Job{
		Name: name,
		Spec: spec,
		Func: fn,
	})
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.cron = cron.New(cron.WithSeconds())

	log.Info("定时任务调度器启动", "job_count", len(s.jobs))

	for _, job := range s.jobs {
		if job.Disabled {
			continue
		}
		name := job.Name
		fn := job.Func
		id, err := s.cron.AddFunc(job.Spec, func() {
			log.Info("执行定时任务", "name", name)
			if err := fn(); err != nil {
				log.Error("定时任务执行失败", "name", name, "error", err)
			}
		})
		if err != nil {
			log.Warn("注册定时任务失败", "name", job.Name, "spec", job.Spec, "error", err)
			continue
		}
		s.entries[job.Name] = id
		log.Info("注册定时任务", "name", job.Name, "spec", job.Spec)
	}

	s.cron.Start()
	s.running = true
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
	}
	s.running = false
	log.Info("定时任务调度器已停止")
}

// RunNow 立即执行一个任务
func (s *Scheduler) RunNow(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, job := range s.jobs {
		if job.Name == name && !job.Disabled {
			return job.Func()
		}
	}
	return fmt.Errorf("任务 %s 未找到", name)
}
