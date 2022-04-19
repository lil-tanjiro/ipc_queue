package main

import (
	"fmt"
	"github.com/lil-tanjiro/ipc_queue"
	"github.com/lil-tanjiro/syscalls"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const (
	CountOfWorkers = 3
	MsgSize        = 1200
)

type Msg struct {
	T    int
	Data [MsgSize]byte
}

func worker(id int, shmId int) {
	var (
		msg *Msg
		q   *ipc_queue.Queue
		res int
		err error
	)

	q, err = ipc_queue.GetQueue(shmId)

	if err != nil {
		log.Println("Worker GETQUEUE", err)
		os.Exit(1)
	}

	msg = new(Msg)

	for {
		time.Sleep(time.Millisecond)

		res, err = q.Pop(unsafe.Pointer(msg))

		if err != nil {
			log.Println("Worker LOOP", err)
			os.Exit(1)
		}

		if res == ipc_queue.QOk {

			log.Println(fmt.Sprintf("Worker %d", id), string(msg.Data[:]))
		}
	}
}

func master(q *ipc_queue.Queue) {
	var (
		i   int
		msg *Msg
		res int
		err error
	)

	msg = new(Msg)
	i = 0

	for {
		time.Sleep(time.Millisecond)

		copy(msg.Data[:], fmt.Sprintf("test test %d", i))

		res, err = q.Push(unsafe.Pointer(msg))

		if err != nil {
			log.Println("MASTER", err)
			os.Exit(1)
		}

		if res != ipc_queue.QOk {
			log.Println("MASTER FILL")
		}

		i++
	}
}

func main() {
	if _, res := os.LookupEnv("DAEMON"); res {
		var (
			shmId int64
			wId   int64
		)

		shmId, err := strconv.ParseInt(os.Getenv("SHMID"), 10, 64)

		if err != nil {
			log.Println("WORKER", err)
			os.Exit(1)
		}

		wId, err = strconv.ParseInt(os.Getenv("WID"), 10, 64)

		if err != nil {
			log.Println("WORKER", err)
			os.Exit(1)
		}

		worker(int(wId), int(shmId))
	} else {
		var (
			pid   int
			pids  [CountOfWorkers]int
			q     *ipc_queue.Queue
			shmId int
			envs  [3]string
			err   error
		)

		q, shmId, err = ipc_queue.MakeQueue("./", 45, 12, MsgSize+unsafe.Sizeof(0))

		if err != nil {
			log.Println(err)

			syscalls.UnixShmRm(shmId)

			os.Exit(0)
		}

		pids = [CountOfWorkers]int{}
		envs = [3]string{}

		envs[0] = "DAEMON=true"
		envs[1] = fmt.Sprintf("SHMID=%d", shmId)

		for i := 0; i < CountOfWorkers; i++ {
			envs[2] = fmt.Sprintf("WID=%d", i)

			pid, err = syscall.ForkExec(os.Args[0], os.Args, &syscall.ProcAttr{
				Env:   append(os.Environ(), envs[:]...),
				Files: []uintptr{0, 1, 2},
			})

			pids[i] = pid
		}

		go listenSigs(pids[:], shmId, q)

		master(q)
	}
}

func listenSigs(pids []int, shmId int, q *ipc_queue.Queue) {
	ch := make(chan os.Signal, 1)

	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<-ch

	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	syscalls.UnixSemDestory(unsafe.Pointer(&q.Sem))

	syscalls.UnixShmRm(shmId)

	log.Println("Stoped")

	os.Exit(0)
}
