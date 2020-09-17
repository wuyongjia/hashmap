# Golang hashmap

## Example


```go
package main

import (
	"fmt"
    "time"
    
    "github.com/wuyongjia/hashmap"
)

type Item struct {
	value int
	str   string
}

var capacity = 5
var hm1 *hashmap.HM
var hm2 *hashmap.HM
var hm3 *hashmap.HM

func main() {
	// #1
	hm1 = hashmap.New(capacity)

	hm1.Put(1, "value1")
	hm1.Put(101, 100001)
	hm1.Put(102, 3.14)
	hm1.Put(123, float32(3.1415))
	hm1.Put(333, &Item{
		value: 123,
		str:   "abcdefghijk",
	})

	fmt.Println(hm1.Get(1).(string))
	fmt.Println(hm1.Get(101).(int))
	fmt.Println(hm1.Get(102).(float64))
	fmt.Println(hm1.Get(123).(float32))

	var item = hm1.Get(333).(*Item)
	fmt.Println("value:", item.value, ", str:", item.str)

	hm1.UpdateWithFunc(333, func(value interface{}) {
		var item = value.(*Item)
		item.value = 321
		item.str = "ABCDEFGHIJKLMN"
	})

	item = hm1.Get(333).(*Item)
	fmt.Println("value:", item.value, ", str:", item.str)

	fmt.Println("count: ", hm1.GetCount(), "\n\n")

	// #2
	hm2 = hashmap.New(capacity)
	for i := 1; i <= capacity; i++ {
		go func(idx int) {
			hm2.Put([]byte(fmt.Sprintf("key%d", idx)), idx)
		}(i)
	}

	time.Sleep(time.Second)

	hm2.Remove([]byte(fmt.Sprintf("key%d", 2)))
	hm2.Remove([]byte(fmt.Sprintf("key%d", 3)))

	hm2.Iterate(func(key interface{}, value interface{}) {
		fmt.Println("key:", string(key.([]byte)), ", value:", value.(int))
	})

	fmt.Println("count: ", hm2.GetCount(), "\n\n")

	// #3
	hm3 = hashmap.New(capacity)
	for i := 1; i <= capacity; i++ {
		go func(idx int) {
			hm3.Put(fmt.Sprintf("key%d", idx), fmt.Sprintf("value%d", idx))
		}(i)
	}

	hm3.Put("mykey123", "hello")

	time.Sleep(time.Second)

	hm3.Iterate(func(key interface{}, value interface{}) {
		fmt.Println("key:", string(key.(string)), ", value:", value.(string))
	})

	fmt.Println("count: ", hm3.GetCount())
}
```