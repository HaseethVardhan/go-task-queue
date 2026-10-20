package queue

import (
	"errors"
	"sync"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

var ErrQueueFull = errors.New("Queue is full")
var ErrCapacityZeroOrNeg = errors.New("Queue capacity given is less than or equals to zero")

type Queue struct {
	mu       sync.Mutex
	tasks    []task.Task
	capacity int
}

func New(capacity int) (*Queue, error) {
	if capacity <= 0 {
		return nil, ErrCapacityZeroOrNeg
	}

	return &Queue{
		tasks:    make([]task.Task, 0, capacity),
		capacity: capacity,
	}, nil
}

func (q *Queue) Enqueue(t task.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) >= q.capacity {
		return ErrQueueFull
	}

	q.tasks = append(q.tasks, t)
	return nil
}

func (q *Queue) Dequeue() (task.Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return task.Task{}, false
	}

	t := q.tasks[0]
	q.tasks[0] = task.Task{}
	q.tasks = q.tasks[1:]

	return t, true
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.tasks)
}