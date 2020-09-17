package hashmap

import (
	"bytes"
	"errors"
	"math"
	"sync"

	"hash/fnv"
)

type IsValidAndUpdataFunc func(value interface{}) bool
type ReadFunc func(key interface{}, value interface{})
type UpdateFunc func(value interface{})

type HM struct {
	slices      []*Pairs
	capacity    int
	mask_uint32 uint32
	mask_int    int
	count       int
	lock        *sync.RWMutex
}

type Pairs struct {
	key   interface{}
	value interface{}
	last  *Pairs
	next  *Pairs
}

func New(capacity int) *HM {
	var defaultCapacity = 1 << 4
	if capacity < defaultCapacity {
		capacity = defaultCapacity
	} else {
		capacity = 1 << (int(math.Ceil(math.Log2(float64(capacity)))))
	}
	var hm = &HM{
		slices:      make([]*Pairs, capacity),
		capacity:    capacity,
		count:       0,
		mask_uint32: uint32(capacity - 1),
		mask_int:    capacity - 1,
		lock:        &sync.RWMutex{},
	}
	return hm
}

func (hm *HM) Expand(capacity int) *HM {
	if capacity <= hm.count {
		panic(errors.New("the capacity is less than the number of items in the list"))
	}
	var newhm = New(capacity)
	var firstPairs, pairs *Pairs
	hm.lock.Lock()
	defer hm.lock.Unlock()
	for _, firstPairs = range hm.slices {
		if firstPairs == nil {
			continue
		}
		pairs = firstPairs
		for {
			newhm.Put(pairs.key, pairs.value)
			if pairs.next != nil {
				pairs = pairs.next
			} else {
				firstPairs.last = pairs
				break
			}
		}
	}
	return newhm
}

func (hm *HM) Get(key interface{}) interface{} {
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	var pairs = hm.getPairsUnsafe(key)
	if pairs != nil {
		return pairs.value
	}
	return nil
}

func (hm *HM) Put(key interface{}, value interface{}) {
	var pairs = hm.getPairs(key)
	if pairs == nil {
		hm.addPairs(key, value)
	} else {
		hm.setValue(pairs, value)
	}
}

func (hm *HM) UpdateWithFunc(key interface{}, updateFunc UpdateFunc) {
	hm.lock.Lock()
	defer hm.lock.Unlock()
	var pairs = hm.getPairsUnsafe(key)
	if pairs != nil {
		updateFunc(pairs.value)
	} else {
		updateFunc(nil)
	}
}

func (hm *HM) Remove(key interface{}) {
	hm.lock.Lock()
	defer hm.lock.Unlock()
	hm.removePairs(key, nil)
}

func (hm *HM) RemoveAndUpdate(key interface{}, updateFunc UpdateFunc) {
	hm.lock.Lock()
	defer hm.lock.Unlock()
	hm.removePairs(key, updateFunc)
}

func (hm *HM) Iterate(readFunc ReadFunc) {
	var pairs *Pairs
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	for _, pairs = range hm.slices {
		if pairs == nil {
			continue
		}
		for {
			readFunc(pairs.key, pairs.value)
			if pairs.next == nil {
				break
			}
			pairs = pairs.next
		}
	}
}

func (hm *HM) IterateAndUpdate(isValidAndUpdateFunc IsValidAndUpdataFunc) {
	var pairs, nextPairs *Pairs
	hm.lock.Lock()
	defer hm.lock.Unlock()
	for _, pairs = range hm.slices {
		if pairs == nil {
			continue
		}
		for {
			nextPairs = pairs.next
			if isValidAndUpdateFunc(pairs.value) == false {
				hm.removePairs(pairs.key, nil)
			}
			if nextPairs == nil {
				break
			}
			pairs = nextPairs
		}
	}
}

func (hm *HM) getPairs(key interface{}) *Pairs {
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	return hm.getPairsUnsafe(key)
}

func (hm *HM) getPairsUnsafe(key interface{}) *Pairs {
	var pairs = hm.slices[hm.GetHashIndex(key)]
	for {
		if pairs == nil {
			break
		}
		if equal(pairs.key, key) {
			return pairs
		}
		pairs = pairs.next
	}
	return nil
}

func (hm *HM) addPairs(key interface{}, value interface{}) {
	var hashIndex = hm.GetHashIndex(key)
	var newPairs = &Pairs{
		key:   key,
		value: value,
		last:  nil,
		next:  nil,
	}
	hm.lock.Lock()
	defer hm.lock.Unlock()
	var pairs = hm.slices[hashIndex]
	if pairs != nil {
		pairs.last.next = newPairs
		pairs.last = newPairs
	} else {
		newPairs.last = newPairs
		hm.slices[hashIndex] = newPairs
	}
	hm.count++
}

func (hm *HM) setValue(pairs *Pairs, value interface{}) {
	hm.lock.Lock()
	defer hm.lock.Unlock()
	pairs.value = value
}

func (hm *HM) setPairsEmpty(pairs *Pairs) {
	switch pairs.key.(type) {
	case []uint8:
		pairs.key = pairs.key.([]byte)[:0]
	case string:
		pairs.key = nil
	}
	pairs.value = nil
	pairs.last = nil
	pairs.next = nil
}

func (hm *HM) removePairs(key interface{}, updateFunc UpdateFunc) {
	var hashIndex = hm.GetHashIndex(key)
	var prevPairs *Pairs = nil
	var pairs = hm.slices[hashIndex]
	var firstPairs = pairs
	for {
		if pairs == nil {
			break
		}
		if equal(pairs.key, key) {
			if prevPairs != nil {
				prevPairs.next = pairs.next
				if pairs.next == nil {
					firstPairs.last = prevPairs
				}
			} else {
				if pairs.next != nil {
					pairs.next.last = pairs.last
				}
				hm.slices[hashIndex] = pairs.next
			}
			if updateFunc != nil {
				updateFunc(pairs.value)
			}
			hm.setPairsEmpty(pairs)
			hm.count--
			break
		}
		prevPairs = pairs
		pairs = pairs.next
	}
}

func (hm *HM) GetHashIndex(key interface{}) int {
	switch key.(type) {
	case []uint8:
		var hash = fnv.New32()
		hash.Write(key.([]byte))
		return int(hash.Sum32() & hm.mask_uint32)
	case string:
		var hash = fnv.New32()
		hash.Write([]byte(key.(string)))
		return int(hash.Sum32() & hm.mask_uint32)
	case int:
		return key.(int) & hm.mask_int
	default:
		panic(errors.New("bad key type"))
	}
}

func (hm *HM) GetCount() int {
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	return hm.count
}

func equal(v1, v2 interface{}) bool {
	switch v1.(type) {
	case []uint8:
		return bytes.Equal(v1.([]byte), v2.([]byte))
	case string:
		return v1.(string) == v2.(string)
	case int:
		return v1.(int) == v2.(int)
	}
	return false
}
