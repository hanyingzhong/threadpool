/* Copyright (C) 2016 Philipp Benner
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package threadpool

/* -------------------------------------------------------------------------- */

//import "fmt"
import "sync"

/* -------------------------------------------------------------------------- */

type ThreadPool struct {
  threads  int
  bufsize  int
  channel  chan func(int, func() error) error
  errmtx  *sync.RWMutex
  errmsg   error
  wg       sync.WaitGroup
}

func NewThreadPool(threads, bufsize int) *ThreadPool {
  if threads < 1 {
    panic("invalid number of threads")
  }
  if bufsize < 1 {
    panic("invalid bufsize")
  }
  t := ThreadPool{}
  t.threads = threads
  t.bufsize = bufsize
  t.errmtx  = new(sync.RWMutex)
  t.errmsg  = nil
  t.Launch()
  return &t
}

func (t *ThreadPool) AddTask(task func(threadIdx int, erf func() error) error) {
  t.channel <- task
}

func (t *ThreadPool) AddRangeTask(iFrom, iTo int, task func(i, threadIdx int, erf func() error) error) {
  n := (iTo-iFrom)/t.NumberOfThreads()
  for j := iFrom; j < iTo; j += n {
    iFrom_ := j
    iTo_   := j+n
    if iTo_ > iTo {
      iTo_ = iTo
    }
    t.AddTask(func(threadIdx int, erf func() error) error {
      for i := iFrom_; i < iTo_; i++ {
        if err := task(i, threadIdx, erf); err != nil {
          return err
        }
      }
      return nil
    })
  }
}

func (t *ThreadPool) Wait() error {
LOOP:
  for {
    select {
    case task := <-t.channel :
      if err := task(0, t.getError); err != nil {
        t.setError(err)
      }
    default:
      break LOOP
    }
  }
  close(t.channel)
  t.wg.Done()
  t.wg.Wait()
  err := t.errmsg
  t.Launch()
  return err
}

func (t *ThreadPool) setError(err error) {
  t.errmtx.Lock()
  t.errmsg = err
  t.errmtx.Unlock()
}

func (t *ThreadPool) getError() error {
  t.errmtx.RLock()
  defer t.errmtx.RUnlock()
  return t.errmsg
}

func (t *ThreadPool) Launch() {
  n := t.NumberOfThreads()
  t.channel = make(chan func(int, func() error) error, t.bufsize)
  t.errmsg  = nil
  t.wg.Add(n)
  for i := 1; i < n; i++ {
    go func(i int) {
      defer t.wg.Done()
      for task := range t.channel {
        if err := task(i, t.getError); err != nil {
          t.setError(err)
        }
      }
    }(i)
  }
}

func (t *ThreadPool) NumberOfThreads() int {
  return t.threads
}
