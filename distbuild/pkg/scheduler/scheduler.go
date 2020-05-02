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
	pickedChan chan struct{}
	isPicked   bool
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

func (c *Scheduler) LocateWorkerWithDeps(deps []build.ID) (api.WorkerID, bool) {
	for _, depID := range deps {
		depJob, exists := c.scheduledJobsMap[depID]

		if !exists {
			continue
		}

		if _, exists := c.workersMap[depJob.workerID]; exists {
			return depJob.workerID, true
		}
	}
	return "", false
}

func (c *Scheduler) RegisterWorker(workerID api.WorkerID) {
	fmt.Println("pkg/scheduler/scheduler.go RegisterWorker", workerID)
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
	fmt.Printf("Scheduler OnJobComplete started\n")
	c.scheduledJobsMutex.Lock()
	defer c.scheduledJobsMutex.Unlock()

	scheduledJob, exists := c.scheduledJobsMap[jobID]
	if !exists {
		fmt.Printf("Scheduler OnJobComplete started: job not exists\n")
		c.scheduledJobsMap[jobID] = &ScheduledJob{
			workerID: workerID,
			pendingJob: &PendingJob{
				Result: res,
				Job: &api.JobSpec{
					Job: build.Job{
						ID: jobID,
					},
				},
			},
		}
		return false
	}

	scheduledJob.pendingJob.Result = res
	fmt.Printf("Scheduler OnJobComplete started: channel closed\n")
	close(scheduledJob.pendingJob.Finished)

	return true
}

func (c *Scheduler) ScheduleJob(job *api.JobSpec) *PendingJob {
	c.scheduledJobsMutex.Lock()
	defer c.scheduledJobsMutex.Unlock()

	if scheduledJob, exists := c.scheduledJobsMap[job.ID]; exists {
		fmt.Println("IF")
		c.scheduledJobsQueue <- job.ID
		return scheduledJob.pendingJob
	}

	pendingJob := &PendingJob{
		Finished: make(chan struct{}),
		Job:      job,
	}

	scheduledJob := &ScheduledJob{
		pendingJob: pendingJob,
		pickedChan: make(chan struct{}),
	}

	c.scheduledJobsMap[job.ID] = scheduledJob
	// TODO: если есть воркер, у которого в кэше артифакты этого job.ID, добавляем к нем у в очередь и возвращаем pendingJob

	// fmt.Println("job")
	// spew.Dump(job)

	fmt.Println("c.scheduledJobsMap")
	for key := range c.scheduledJobsMap {
		fmt.Println(">>", key)
	}

	workerID, found := c.LocateWorkerWithDeps(job.Deps)
	if !found {
		go func() {
			// c.scheduledJobsQueue <- job.ID
			fmt.Println("!found go 1")
			select {
			case <-timeAfter(c.config.CacheTimeout):
				fmt.Println("!found go 2")
				c.scheduledJobsQueue <- job.ID
			case <-timeAfter(c.config.DepsTimeout):
				fmt.Println("!found go 3")
				c.scheduledJobsQueue <- job.ID
			}
		}()
		return pendingJob
	}

	go func() {
		fmt.Println("go 1")
		c.workersMap[workerID].queue1 <- job.ID
		// fmt.Println("go 11")
		select {
		case <-timeAfter(c.config.CacheTimeout):
			fmt.Println("go 2")
			c.workersMap[workerID].queue2 <- job.ID
			select {
			case <-timeAfter(c.config.DepsTimeout):
				fmt.Println("go 3")
				c.scheduledJobsQueue <- job.ID
			case <-scheduledJob.pickedChan:
				fmt.Println("go 4")
				return
			}
		case <-scheduledJob.pickedChan:
			fmt.Println("go 5")
			return
		default:
			fmt.Println("go 6")
		}
	}()
	// go func() {
	// 	fmt.Println("ScheduleJob start gourutine")
	// 	select {
	// 	// case <-c.workerRegistered:
	// 	// c.scheduledJobsQueue <- job.ID
	// 	case <-timeAfter(c.config.CacheTimeout):
	// 		fmt.Println("ScheduleJob select CacheTimeout")
	// 		c.scheduledJobsQueue <- job.ID
	// 	case <-timeAfter(c.config.DepsTimeout):
	// 		fmt.Println("ScheduleJob select DepsTimeout")
	// 		c.scheduledJobsQueue <- job.ID
	// 	default:
	// 		fmt.Println("ScheduleJob select default, job:", job)
	// 		c.scheduledJobsQueue <- job.ID
	// 	}
	// 	fmt.Println("ScheduleJob goroutine exited")
	// }()

	return pendingJob
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	c.logger.Info("pkg/scheduler/scheduler.go PickJob", zap.String("workerID", workerID.String()))
	// PickJob - блокируется. ScheduleJob - нет.
	fmt.Println("PickJob > ", len(c.workersMap))
	queues, exists := c.workersMap[workerID]
	if !exists {
		c.RegisterWorker(workerID)
		queues = c.workersMap[workerID]
	}
	// fmt.Println(len(c.scheduledJobsQueue))
	// fmt.Println(len(queues.queue1))
	// fmt.Println(len(queues.queue2))

	// for {
	fmt.Println("for 1", len(c.scheduledJobsQueue), len(queues.queue1), len(queues.queue2))
	var pendingJobID build.ID
	select {
	case pendingJobID = <-c.scheduledJobsQueue:
	case pendingJobID = <-queues.queue1:
	case pendingJobID = <-queues.queue2:
	case <-ctx.Done():
		return nil
	}
	fmt.Println("for 2")
	c.scheduledJobsMutex.Lock()
	sheduledJob := c.scheduledJobsMap[pendingJobID]
	// if sheduledJob.isPicked {
	// 	fmt.Println("for 3")
	// 	c.scheduledJobsMutex.Unlock()
	// 	continue
	// }
	sheduledJob.workerID = workerID
	sheduledJob.isPicked = true
	// close(sheduledJob.pickedChan)

	c.scheduledJobsMutex.Unlock()
	fmt.Println("for 4", pendingJobID)

	return sheduledJob.pendingJob
	// }
}
