package utility

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// 定义错误
var (
	ErrPoolClosed = errors.New("worker pool is closed")
	ErrWorkerBusy = errors.New("worker is busy")
	ErrTimeout    = errors.New("operation timeout")
	ErrNoWorker   = errors.New("no available worker")
)

// TaskFunc 任务函数类型
type TaskFunc func(args ...interface{}) (interface{}, error)

// Task 任务定义
type Task struct {
	ID      string
	Fn      TaskFunc
	Args    []interface{}
	Result  chan<- *TaskResult
	Timeout time.Duration
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID string
	Value  interface{}
	Error  error
	Cost   time.Duration
}

// Worker 工作协程对象
type Worker struct {
	id         string
	pool       *WorkerPool
	taskChan   chan *Task
	quit       chan struct{}
	busy       int32 // 原子操作：0-空闲，1-忙碌
	lastActive time.Time
	ctx        context.Context
	cancel     context.CancelFunc
}

// WorkerPool 协程池
type WorkerPool struct {
	workers       []*Worker
	workerChan    chan *Worker // 空闲worker队列
	lock          sync.RWMutex
	closed        bool
	minWorkers    int           // 最小worker数
	maxWorkers    int           // 最大worker数
	idleTimeout   time.Duration // 空闲超时
	taskQueueSize int           // 任务队列大小

	// 监控指标
	totalTasks  uint64
	failedTasks uint64
	busyWorkers int32
}

// NewWorkerPool 创建Worker池
func NewWorkerPool(
	minWorkers int, // 最小worker数
	maxWorkers int, // 最大worker数
	idleTimeout time.Duration, // 空闲超时
	taskQueueSize int, // 任务队列大小
) *WorkerPool {
	if minWorkers < 1 {
		minWorkers = runtime.NumCPU()
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers * 10
	}
	if idleTimeout == 0 {
		idleTimeout = 1 * time.Minute
	}

	pool := &WorkerPool{
		workerChan:    make(chan *Worker, maxWorkers),
		minWorkers:    minWorkers,
		maxWorkers:    maxWorkers,
		idleTimeout:   idleTimeout,
		taskQueueSize: taskQueueSize,
	}

	// 初始化最小数量的worker
	pool.lock.Lock()
	for i := 0; i < minWorkers; i++ {
		pool.createWorker()
	}
	pool.lock.Unlock()

	// 启动监控协程
	go pool.monitor()

	return pool
}

// GetWorker 获取一个空闲的worker（带超时）
func (p *WorkerPool) GetWorker(timeout time.Duration) (*Worker, error) {
	if p.closed {
		return nil, ErrPoolClosed
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	select {
	case worker := <-p.workerChan:
		if atomic.CompareAndSwapInt32(&worker.busy, 0, 1) {
			atomic.AddInt32(&p.busyWorkers, 1)
			worker.lastActive = time.Now()
			return worker, nil
		}
		// worker被其他goroutine抢走了，重新尝试
		return p.GetWorker(timeout)

	case <-ctx.Done():
		// 尝试创建新的worker
		p.lock.Lock()
		if len(p.workers) < p.maxWorkers {
			worker := p.createWorker()
			p.lock.Unlock()
			if atomic.CompareAndSwapInt32(&worker.busy, 0, 1) {
				atomic.AddInt32(&p.busyWorkers, 1)
				return worker, nil
			}
		} else {
			p.lock.Unlock()
		}
		return nil, ErrNoWorker
	}
}

// Execute 在worker上执行任务并获取结果
func (w *Worker) Execute(task *Task) error {
	if atomic.LoadInt32(&w.busy) != 1 {
		return ErrWorkerBusy
	}

	select {
	case w.taskChan <- task:
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}

// ExecuteFunc 执行函数并返回结果（同步方式）
func (w *Worker) ExecuteFunc(fn TaskFunc, args ...interface{}) (interface{}, error) {
	resultChan := make(chan *TaskResult, 1)
	task := &Task{
		Fn:     fn,
		Args:   args,
		Result: resultChan,
	}

	if err := w.Execute(task); err != nil {
		return nil, err
	}

	result := <-resultChan
	return result.Value, result.Error
}

// ExecuteFuncAsync 异步执行函数
func (w *Worker) ExecuteFuncAsync(fn TaskFunc, callback func(interface{}, error), args ...interface{}) error {
	resultChan := make(chan *TaskResult, 1)
	task := &Task{
		Fn:     fn,
		Args:   args,
		Result: resultChan,
	}

	if err := w.Execute(task); err != nil {
		return err
	}

	// 异步处理结果
	go func() {
		result := <-resultChan
		callback(result.Value, result.Error)
	}()

	return nil
}

// Release 释放worker回池中
func (w *Worker) Release() {
	if atomic.CompareAndSwapInt32(&w.busy, 1, 0) {
		atomic.AddInt32(&w.pool.busyWorkers, -1)
		select {
		case w.pool.workerChan <- w:
			// 成功放回池中
		default:
			// 池已满，worker自动退出
			w.stop()
		}
	}
}

// createWorker 创建新的worker
func (p *WorkerPool) createWorker() *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	worker := &Worker{
		id:         fmt.Sprintf("worker-%d", len(p.workers)+1),
		pool:       p,
		taskChan:   make(chan *Task, p.taskQueueSize),
		quit:       make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
		lastActive: time.Now(),
	}

	p.workers = append(p.workers, worker)

	// 启动worker协程
	go worker.run()

	// 将worker放入空闲队列
	p.workerChan <- worker

	return worker
}

// run worker主循环
func (w *Worker) run() {
	defer func() {
		if r := recover(); r != nil {
			// 处理panic
			fmt.Printf("Worker %s panic: %v\n", w.id, r)
		}
		w.stop()
	}()

	for {
		select {
		case task := <-w.taskChan:
			w.executeTask(task)

		case <-w.quit:
			return

		case <-w.ctx.Done():
			return
		}
	}
}

// executeTask 执行具体任务
func (w *Worker) executeTask(task *Task) {
	start := time.Now()
	var result interface{}
	var err error

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panic: %v", r)
			atomic.AddUint64(&w.pool.failedTasks, 1)
		}

		if task.Result != nil {
			task.Result <- &TaskResult{
				TaskID: task.ID,
				Value:  result,
				Error:  err,
				Cost:   time.Since(start),
			}
		}

		atomic.AddUint64(&w.pool.totalTasks, 1)
		w.lastActive = time.Now()
	}()

	// 设置超时
	if task.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), task.Timeout)
		defer cancel()

		done := make(chan struct{})
		go func() {
			result, err = task.Fn(task.Args...)
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			err = ErrTimeout
			return
		}
	}

	result, err = task.Fn(task.Args...)
}

// stop 停止worker
func (w *Worker) stop() {
	w.cancel()
	close(w.quit)
}

// monitor 监控协程
func (p *WorkerPool) monitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdleWorkers()
			p.printStats()

		case <-time.After(1 * time.Minute):
			if p.closed {
				return
			}
		}
	}
}

// cleanupIdleWorkers 清理空闲worker
func (p *WorkerPool) cleanupIdleWorkers() {
	p.lock.Lock()
	defer p.lock.Unlock()

	if len(p.workers) <= p.minWorkers {
		return
	}

	activeWorkers := make([]*Worker, 0, len(p.workers))
	for _, worker := range p.workers {
		if time.Since(worker.lastActive) < p.idleTimeout ||
			atomic.LoadInt32(&worker.busy) == 1 {
			activeWorkers = append(activeWorkers, worker)
		} else {
			worker.stop()
		}
	}

	p.workers = activeWorkers
}

// Close 关闭协程池
func (p *WorkerPool) Close() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.closed = true
	close(p.workerChan)

	for _, worker := range p.workers {
		worker.stop()
	}

	p.workers = nil
}

// Stats 获取池状态
func (p *WorkerPool) Stats() map[string]interface{} {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return map[string]interface{}{
		"total_workers":   len(p.workers),
		"busy_workers":    atomic.LoadInt32(&p.busyWorkers),
		"idle_workers":    len(p.workers) - int(atomic.LoadInt32(&p.busyWorkers)),
		"total_tasks":     atomic.LoadUint64(&p.totalTasks),
		"failed_tasks":    atomic.LoadUint64(&p.failedTasks),
		"worker_chan_len": len(p.workerChan),
		"closed":          p.closed,
	}
}

func (p *WorkerPool) printStats() {
	stats := p.Stats()
	fmt.Printf("[WorkerPool Stats] Workers: %d/%d, Busy: %d, Tasks: %d/%d\n",
		stats["idle_workers"], stats["total_workers"],
		stats["busy_workers"],
		stats["failed_tasks"], stats["total_tasks"])
}
