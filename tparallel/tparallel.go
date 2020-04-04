// +build !solution

package tparallel

type T struct {
	finished   chan bool
	barrier    chan bool
	isParallel bool
	parent     *T
	sub        []*T
}

func (t *T) Parallel() {
	if t.isParallel {
		panic("test is already parallel")
	}
	t.isParallel = true
	t.parent.sub = append(t.parent.sub, t)

	t.finished <- true
	<-t.parent.barrier
}

func (t *T) tRunner(subtest func(t *T)) {
	subtest(t)
	if len(t.sub) > 0 {
		close(t.barrier)

		for _, sub := range t.sub {
			<-sub.finished
		}
	}
	if t.isParallel {
		t.parent.finished <- true
	}
	t.finished <- true

}

func (t *T) Run(subtest func(t *T)) {
	subT := &T{
		finished: make(chan bool),
		barrier:  make(chan bool),
		parent:   t,
		sub:      make([]*T, 0),
	}
	go subT.tRunner(subtest)
	<-subT.finished
}

func Run(topTests []func(t *T)) {
	root := &T{
		finished: make(chan bool),
		barrier:  make(chan bool),
		parent:   nil,
		sub:      make([]*T, 0),
	}
	for _, fn := range topTests {
		root.Run(fn)
	}
	close(root.barrier)

	if len(root.sub) > 0 {
		<-root.finished
	}
}
