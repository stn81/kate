package timerengine

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stn81/kate/taskengine"
	"go.uber.org/zap"
)

const (
	// RingSize define the ring buffer size for timer engine
	RingSize = 3600
)

type request struct {
	task           Task
	delayInSeconds int64
	result         chan *TimerTask
}

// TimerEngine define the timer engine
// nolint:maligned
type TimerEngine struct {
	name      string
	buckets   []*list.List
	requests  chan *request
	ticker    <-chan time.Time
	tickIndex uint32
	executors *taskengine.TaskEngine
	taskIDSeq uint64
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	logger    *zap.Logger
}

// New create a new TimerEngine
func New(name string, concurrencyLevel int, logger *zap.Logger) *TimerEngine {
	newctx, cancel := context.WithCancel(context.Background())

	te := &TimerEngine{
		name:      name,
		ticker:    time.Tick(time.Second),
		buckets:   make([]*list.List, RingSize),
		requests:  make(chan *request, 1024),
		executors: taskengine.New(newctx, name, concurrencyLevel, logger),
		ctx:       newctx,
		cancel:    cancel,
		logger:    logger.With(zap.String("timerengine", name)),
	}

	for i := 0; i < len(te.buckets); i++ {
		te.buckets[i] = list.New()
	}

	return te
}

// Name return the name of timer engine
func (te *TimerEngine) Name() string {
	return te.name
}

// Start the timer engine
func (te *TimerEngine) Start() {
	te.wg.Add(1)
	go te.loop()
}

// Stop the timer engine
func (te *TimerEngine) Stop() {
	te.cancel()
	te.wg.Wait()
}

func (te *TimerEngine) loop() {
	te.logger.Info("timer engine loop started")

	defer func() {
		if r := recover(); r != nil {
			te.logger.Fatal("panic", zap.Any("error", r), zap.Stack("stack"))
		}

		te.cancel()
		te.executors.Shutdown()
		te.wg.Done()
		te.logger.Info("main loop stopped")
	}()

	for {
		select {
		case <-te.ctx.Done():
			return
		case req := <-te.requests:
			{
				var (
					tickIndex   = te.getTickIndex()
					offset      = int(req.delayInSeconds) + int(tickIndex)
					cycleNum    = offset / RingSize
					bucketIndex = offset % RingSize
					bucket      = te.buckets[bucketIndex]
					task        = newTimerTask(te, cycleNum, req.task)
				)
				bucket.PushBack(task)

				req.result <- task
			}
		case <-te.ticker:
			{
				var (
					tickIndex = te.updateTickIndex()
					tasks     = te.buckets[tickIndex]
				)

				go func(tasks *list.List, tickIndex uint32) {
					var next *list.Element

					for te := tasks.Front(); te != nil; te = next {
						next = te.Next()
						task := te.Value.(*TimerTask)

						if task.ready() {
							task.dispose()
							tasks.Remove(te)
						}
					}
				}(tasks, tickIndex)
			}
		}
	}
}

func (te *TimerEngine) nextTaskID() uint64 {
	return atomic.AddUint64(&te.taskIDSeq, 1)
}

func (te *TimerEngine) updateTickIndex() uint32 {
	tickIndex := atomic.AddUint32(&te.tickIndex, 1)
	tickIndex %= RingSize
	return tickIndex
}

func (te *TimerEngine) getTickIndex() uint32 {
	tickIndex := atomic.LoadUint32(&te.tickIndex)
	tickIndex %= RingSize
	return tickIndex
}

func (te *TimerEngine) execute(f TaskFunc) {
	te.executors.Schedule(f)
}

// Schedule a timer task with delay
func (te *TimerEngine) Schedule(task Task, delay time.Duration) (timerTask *TimerTask) {
	if delay <= 0 {
		timerTask = newTimerTask(te, 0, task)
		timerTask.dispose()
		return
	}

	req := &request{
		task:           task,
		delayInSeconds: int64(delay.Seconds()),
		result:         make(chan *TimerTask, 1),
	}

	select {
	case <-te.ctx.Done():
		return
	case te.requests <- req:
	}

	select {
	case <-te.ctx.Done():
	case timerTask = <-req.result:
	}
	return
}
