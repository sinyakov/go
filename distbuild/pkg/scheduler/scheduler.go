// +build !solution

package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

var timeAfter = time.After

type PendingJob struct {
	Job      *api.JobSpec
	Finished chan struct{}
	Result   *api.JobResult
}

type Config struct {
	CacheTimeout time.Duration
	DepsTimeout  time.Duration
}

type Scheduler struct {
	logger             *zap.Logger
	config             Config
	scheduledJobsMutex *sync.Mutex
	scheduledJobsQueue chan build.ID
	scheduledJobsMap   map[build.ID]*ScheduledJob
	workersMap         map[api.WorkerID]*WorkerQueues
	// workerRegistered   chan struct{}
}

type ScheduledJob struct {
	pendingJob *PendingJob
	workerID   api.WorkerID
}

type WorkerQueues struct {
	queue1 chan build.ID
	queue2 chan build.ID
}

func NewScheduler(l *zap.Logger, config Config) *Scheduler {
	return &Scheduler{
		logger:             l,
		config:             config,
		scheduledJobsMutex: &sync.Mutex{},
		scheduledJobsMap:   make(map[build.ID]*ScheduledJob),
		scheduledJobsQueue: make(chan build.ID, 1000),
		workersMap:         make(map[api.WorkerID]*WorkerQueues),
		// workerRegistered:   make(chan struct{}),
	}
}

func (c *Scheduler) LocateArtifact(id build.ID) (api.WorkerID, bool) {
	panic("implement me")
}

func (c *Scheduler) RegisterWorker(workerID api.WorkerID) {
	if _, exists := c.workersMap[workerID]; exists {
		return
	}

	workerQueues := &WorkerQueues{
		queue1: make(chan build.ID, 1000),
		queue2: make(chan build.ID, 1000),
	}

	c.workersMap[workerID] = workerQueues
	// c.workerRegistered <- struct{}{}
}

func (c *Scheduler) OnJobComplete(workerID api.WorkerID, jobID build.ID, res *api.JobResult) bool {
	c.scheduledJobsMutex.Lock()
	defer c.scheduledJobsMutex.Unlock()

	scheduledJob, exists := c.scheduledJobsMap[jobID]
	if !exists {
		return false
	}

	scheduledJob.pendingJob.Result = res
	close(scheduledJob.pendingJob.Finished)

	return true
}

func (c *Scheduler) ScheduleJob(job *api.JobSpec) *PendingJob {
	c.scheduledJobsMutex.Lock()
	defer c.scheduledJobsMutex.Unlock()

	if scheduledJob, exists := c.scheduledJobsMap[job.ID]; exists {
		return scheduledJob.pendingJob
	}

	pendingJob := &PendingJob{
		Finished: make(chan struct{}),
		Job:      job,
	}

	scheduledJob := &ScheduledJob{
		pendingJob: pendingJob,
	}

	c.scheduledJobsMap[job.ID] = scheduledJob
	// TODO: если есть воркер, у которого в кэше артифакты этого job.ID, добавляем к нем у в очередь и возвращаем pendingJob

	go func() {
		fmt.Println("gooo")
		select {
		// case <-c.workerRegistered:
		// c.scheduledJobsQueue <- job.ID
		case <-timeAfter(c.config.CacheTimeout):
			fmt.Println("CacheTimeout")
			c.scheduledJobsQueue <- job.ID
		case <-timeAfter(c.config.DepsTimeout):
			fmt.Println("DepsTimeout")
			c.scheduledJobsQueue <- job.ID
		default:
			fmt.Println("default")
			c.scheduledJobsQueue <- job.ID
		}
	}()

	return pendingJob
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	// PickJob - блокируется. ScheduleJob - нет.

	queues := c.workersMap[workerID]
	// fmt.Println(len(c.scheduledJobsQueue))
	// fmt.Println(len(queues.queue1))
	// fmt.Println(len(queues.queue2))

	select {
	case pendingJobID := <-c.scheduledJobsQueue:
		c.scheduledJobsMutex.Lock()
		c.scheduledJobsMap[pendingJobID].workerID = workerID
		c.scheduledJobsMutex.Unlock()
		return c.scheduledJobsMap[pendingJobID].pendingJob
	case pendingJobID := <-queues.queue1:
		c.scheduledJobsMutex.Lock()
		c.scheduledJobsMap[pendingJobID].workerID = workerID
		c.scheduledJobsMutex.Unlock()
		return c.scheduledJobsMap[pendingJobID].pendingJob
	case pendingJobID := <-queues.queue2:
		c.scheduledJobsMutex.Lock()
		c.scheduledJobsMap[pendingJobID].workerID = workerID
		c.scheduledJobsMutex.Unlock()
		return c.scheduledJobsMap[pendingJobID].pendingJob
	case <-ctx.Done():
		return nil
	}

}
