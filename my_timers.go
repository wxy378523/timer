package mytimers

//Linux 2.6 Dynamic time wheel, only support false unregister tick node(TimerNode),tick.F nothing todo.
//add O(1),execute O(1),very high,Oh yeah.

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

const (
	TIME_NEAR_SHIFT = 8
	TIME_NEAR       = (1 << TIME_NEAR_SHIFT)
	TIME_NEAR_MASK  = (TIME_NEAR - 1)

	TIME_LEVEL_SHIFT = 6
	TIME_LEVEL       = (1 << TIME_LEVEL_SHIFT)
	TIME_LEVEL_MASK  = (TIME_LEVEL - 1)
)

//TimerNode gloabl
type TimerNode struct {
	ID       string
	expire   uint32               //期望到期时间,单位 10ms
	F        func(...interface{}) //回调
	Args     []interface{}        //参数
	Priority int32                //优先等级
}

//GTimer gloabl
type GTimer struct {
	Levels [4][TIME_LEVEL]*list.List //接近到期时间的256个tick集合
	Nears  [TIME_NEAR]*list.List     //四个等级 松散队列
	lock   sync.Mutex                //读写锁
	Times  uint32                    //滴答数
	Tick   time.Duration             //时间间隔
	Quit   chan struct{}             //退出的管道
}

// GolablTimer gloabl
var GolablTimer *GTimer = nil

// MyNew gloabl
func MyNew(tdur time.Duration) *GTimer {
	GT := &GTimer{
		Tick: tdur,
		Quit: make(chan struct{}),
	}

	for i := 0; i < 4; i++ {
		for j := 0; j < TIME_LEVEL; j++ {
			GT.Levels[i][j] = list.New()
		}
	}

	for i := 0; i < TIME_NEAR; i++ {
		GT.Nears[i] = list.New()
	}
	return GT
}

// Register d 单位 10ms
func (t *GTimer) Register(d uint32, f func(...interface{}), pri int32, args []interface{}) *TimerNode {
	node := &TimerNode{
		Args:     args,
		F:        f,
		Priority: pri,
	}

	t.lock.Lock()
	node.expire = uint32(d) + t.Times
	t.addNode(node)
	t.lock.Unlock()
	return node
}

func (t *GTimer) addNode(node *TimerNode) {
	curTime := t.Times
	expire := node.expire

	if (expire | TIME_NEAR_MASK) == (curTime | TIME_NEAR_MASK) {
		t.Nears[expire&TIME_NEAR_MASK].PushBack(node)
	} else {
		var mask uint32 = TIME_NEAR << TIME_LEVEL_SHIFT
		var level uint32
		for level := 0; level < 3; level++ {
			if (expire | (mask - 1)) == (curTime | (mask - 1)) {
				break
			}
			mask <<= TIME_LEVEL_SHIFT
		}
		t.Levels[level][((expire >> (TIME_NEAR_SHIFT + level*TIME_LEVEL_SHIFT)) & TIME_LEVEL_MASK)].PushBack(node)
	}
}

func (t *GTimer) cascade() {
	var mask uint32 = TIME_NEAR
	t.Times++
	ct := t.Times
	if ct == 0 {
		t.moveList(3, 0)
	} else {
		time := ct >> TIME_NEAR_SHIFT
		var i int = 0
		for (ct & (mask - 1)) == 0 {
			index := int(time & TIME_LEVEL_MASK)
			if index != 0 {
				t.moveList(i, index)
				break
			}
			mask <<= TIME_LEVEL_SHIFT
			time >>= TIME_LEVEL_SHIFT
			i++
		}
	}
}

func (t *GTimer) moveList(level, index int) {
	vec := t.Levels[level][index]
	front := vec.Front()
	vec.Init()
	for e := front; e != nil; e = e.Next() {
		node := e.Value.(*TimerNode)
		t.addNode(node)
	}
}

func (t *GTimer) execute() {
	index := t.Times & TIME_NEAR_MASK
	vec := t.Nears[index]
	if vec.Len() != 0 {
		f := vec.Front()
		vec.Init()
		t.lock.Unlock()
		dispatchList(f)
		t.lock.Lock()
		return
	}
}

//DispatchList golabl
func dispatchList(front *list.Element) {
	for element := front; element != nil; element = element.Next() {
		n := element.Value.(*TimerNode)
		n.F(n.Args)
	}
}

func (t *GTimer) timerUpdate() {
	t.lock.Lock()
	// try to dispatch timeout 0 (rare condition)
	t.execute()

	t.cascade()

	t.execute()
	t.lock.Unlock()
}

//Start golabl
func (t *GTimer) Start() {
	tick := time.NewTicker(t.Tick)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			t.timerUpdate()
		case <-t.Quit:
			return
		}
	}
}

//Stop GTimer golabl
func (t *GTimer) Stop() {
	close(t.Quit)
}

// StartTimerServer start timer server
func StartTimerServer() {
	GolablTimer = MyNew(time.Millisecond * 10)
	fmt.Println("golabl NB timer server start!")
	go GolablTimer.Start()
}
