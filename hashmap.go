package hashmap

import (
	"bytes"
	"errors"
	"math"
	"sync"

	"hash/fnv"
)

type IsValidAndUpdataFunc func(key interface{}, value interface{}) bool
type ReadFunc func(key interface{}, value interface{})
type UpdateFunc func(value interface{})
type EqualFunc func(v1, v2 interface{}) bool

type HM struct {
	slices      []*Pairs
	capacity    int
	mask_uint32 uint32
	mask_uint64 uint64
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
		mask_int:    capacity - 1,
		mask_uint32: uint32(capacity - 1),
		mask_uint64: uint64(capacity - 1),
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
	var pairs, _ = hm.getPairsUnsafe(key)
	if pairs != nil {
		return pairs.value
	}
	return nil
}

func (hm *HM) Put(key interface{}, value interface{}) {
	var pairs, hashIndex = hm.getPairs(key)
	if pairs == nil {
		var newPairs = &Pairs{
			key:   key,
			value: value,
			last:  nil,
			next:  nil,
		}
		hm.lock.Lock()
		defer hm.lock.Unlock()
		pairs = hm.slices[hashIndex]
		if pairs != nil {
			pairs.last.next = newPairs
			pairs.last = newPairs
		} else {
			newPairs.last = newPairs
			hm.slices[hashIndex] = newPairs
		}
		hm.count++
	} else {
		hm.lock.Lock()
		defer hm.lock.Unlock()
		pairs.value = value
	}
}

func (hm *HM) UpdateWithFunc(key interface{}, updateFunc UpdateFunc) {
	hm.lock.Lock()
	defer hm.lock.Unlock()
	var pairs, _ = hm.getPairsUnsafe(key)
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

func (hm *HM) RemoveUnsafe(key interface{}) {
	hm.removePairs(key, nil)
}

func (hm *HM) RemoveAndUpdate(key interface{}, updateFunc UpdateFunc) {
	hm.lock.Lock()
	defer hm.lock.Unlock()
	hm.removePairs(key, updateFunc)
}

func (hm *HM) Exists(key interface{}) bool {
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	var pairs, _ = hm.getPairsUnsafe(key)
	if pairs != nil {
		return true
	}
	return false
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
			if isValidAndUpdateFunc(pairs.key, pairs.value) == false {
				hm.removePairs(pairs.key, nil)
			}
			if nextPairs == nil {
				break
			}
			pairs = nextPairs
		}
	}
}

func (hm *HM) getPairs(key interface{}) (*Pairs, int) {
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	return hm.getPairsUnsafe(key)
}

func (hm *HM) getPairsUnsafe(key interface{}) (*Pairs, int) {
	var hashIndex, equal = hm.getHashIndexAndEqualFunc(key)
	var pairs = hm.slices[hashIndex]
	for {
		if pairs == nil {
			break
		}
		if equal(pairs.key, key) {
			return pairs, hashIndex
		}
		pairs = pairs.next
	}
	return nil, hashIndex
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
	var hashIndex, equal = hm.getHashIndexAndEqualFunc(key)
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

func (hm *HM) GetCount() int {
	hm.lock.RLock()
	defer hm.lock.RUnlock()
	return hm.count
}

func (hm *HM) getHashIndexAndEqualFunc(key interface{}) (int, EqualFunc) {
	switch key.(type) {
	case []uint8:
		var hash = fnv.New32()
		hash.Write(key.([]byte))
		return int(hash.Sum32() & hm.mask_uint32), bytesEqual
	case string:
		var hash = fnv.New32()
		hash.Write([]byte(key.(string)))
		return int(hash.Sum32() & hm.mask_uint32), stringEqual
	case int:
		return key.(int) & hm.mask_int, intEqual
	case uint64:
		return int(key.(uint64) & hm.mask_uint64), uint64Equal
	case uint32:
		return int(key.(uint32) & hm.mask_uint32), uint32Equal
	default:
		panic(errors.New("bad key type"))
	}
}

func bytesEqual(v1, v2 interface{}) bool {
	return bytes.Equal(v1.([]byte), v2.([]byte))
}

func stringEqual(v1, v2 interface{}) bool {
	return v1.(string) == v2.(string)
}

func intEqual(v1, v2 interface{}) bool {
	return v1.(int) == v2.(int)
}

func uint32Equal(v1, v2 interface{}) bool {
	return v1.(uint32) == v2.(uint32)
}

func uint64Equal(v1, v2 interface{}) bool {
	return v1.(uint64) == v2.(uint64)
}
