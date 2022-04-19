package ipc_queue

//#include <semaphore.h>
//#include <string.h>
import "C"
import (
	"github.com/lil-tanjiro/syscalls"
	"sync/atomic"
	"unsafe"
)

const (
	QOk = iota
	QMiss
)

const (
	SemaphorePShared = 1
	SemaphoreValue   = 1
)

type Queue struct {
	// HEADER SECTION
	// semaphore
	Sem C.sem_t

	// config
	cap      uint64
	size     uint64
	itemSize uintptr

	// DATA SECTION
	// byte for get pointer on the data section
	Data byte
}

func getQueueSize() uintptr {
	return unsafe.Sizeof(Queue{}) - (unsafe.Sizeof(int64(0)) - unsafe.Sizeof(byte(0)))
}

func MakeQueue(file string, id int, cap uint64, msgSize uintptr) (*Queue, int, error) {
	var (
		q     *Queue
		addr  unsafe.Pointer
		shmId int
		err   error
	)

	// make shared mem
	shmId, err = syscalls.UnixShmGet(file, id, uint64(getQueueSize()+(uintptr(cap)*msgSize)))

	if err != nil {
		return nil, 0, err
	}

	// attach current pid
	addr, err = syscalls.UnixShmAttach(shmId)

	if err != nil {
		return nil, shmId, err
	}

	// init semaphore
	err = syscalls.UnixInitUnnamedSem(addr, SemaphorePShared, SemaphoreValue)

	if err != nil {
		return nil, shmId, err
	}

	q = (*Queue)(addr)

	q.cap = cap
	q.size = 0
	q.itemSize = msgSize

	return q, shmId, nil
}

func GetQueue(shmId int) (*Queue, error) {
	var (
		addr unsafe.Pointer
		err  error
	)

	addr, err = syscalls.UnixShmAttach(shmId)

	if err != nil {
		return nil, err
	}

	return (*Queue)(addr), nil
}

func (q *Queue) Push(addr unsafe.Pointer) (int, error) {
	var (
		res int
		err error
	)

	res = QMiss

	err = syscalls.UnixSemWait(unsafe.Pointer(&q.Sem))

	if err != nil {
		return QMiss, err
	}

	if q.size < q.cap {
		C.memcpy(unsafe.Pointer(uintptr(unsafe.Pointer(&q.Data))+(uintptr(q.size)*q.itemSize)), addr, C.size_t(q.itemSize))

		q.size++

		res = QOk
	}

	err = syscalls.UnixSemPost(unsafe.Pointer(&q.Sem))

	if err != nil {
		return QMiss, err
	}

	return res, nil
}

func (q *Queue) Pop(addr unsafe.Pointer) (int, error) {
	var (
		res int
		err error
	)

	res = QMiss

	err = syscalls.UnixSemWait(unsafe.Pointer(&q.Sem))

	if err != nil {
		return QMiss, err
	}

	if q.size != 0 {
		q.size--

		C.memcpy(addr, unsafe.Pointer(uintptr(unsafe.Pointer(&q.Data))+(uintptr(q.size)*q.itemSize)), C.size_t(q.itemSize))

		res = QOk
	}

	err = syscalls.UnixSemPost(unsafe.Pointer(&q.Sem))

	if err != nil {
		return QMiss, err
	}

	return res, nil
}

func (q *Queue) GetSize() uint64 {
	return atomic.LoadUint64(&q.size)
}
