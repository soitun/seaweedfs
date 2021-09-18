package bptree

import (
	"encoding/hex"
	"runtime/debug"
	"sort"
	"sync"
	"testing"

	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand"

)

var rand *mrand.Rand

func init() {
	seed := make([]byte, 8)
	if _, err := crand.Read(seed); err == nil {
		rand = ThreadSafeRand(int64(binary.BigEndian.Uint64(seed)))
	} else {
		panic(err)
	}
}

func randslice(length int) []byte {
	return RandSlice(length)
}

func randstr(length int) String {
	return String(RandStr(length))
}

type Strings []String

func (self Strings) Len() int {
	return len(self)
}

func (self Strings) Less(i, j int) bool {
	return self[i].Less(self[j])
}

func (self Strings) Swap(i, j int) {
	self[i], self[j] = self[j], self[i]
}

type record struct {
	key   String
	value ItemValue
}

type records []*record

func (self records) Len() int {
	return len(self)
}

func (self records) Less(i, j int) bool {
	return self[i].key.Less(self[j].key)
}

func (self records) Swap(i, j int) {
	self[i], self[j] = self[j], self[i]
}

func BenchmarkBpTree(b *testing.B) {
	b.StopTimer()

	recs := make(records, 100)
	ranrec := func() *record {
		return &record{randstr(20), randstr(20)}
	}

	for i := range recs {
		recs[i] = ranrec()
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		t := NewBpTree(23, nil)
		for _, r := range recs {
			t.Add(r.key, r.value)
		}
		for _, r := range recs {
			t.RemoveWhere(r.key, func(value ItemValue) bool { return true })
		}
	}
}

func TestAddHasCountFindIterateRemove(t *testing.T) {

	ranrec := func() *record {
		return &record{
			randstr(12),
			randstr(12),
		}
	}

	test := func(bpt *BpTree) {
		var err error
		recs := make(records, 128)
		new_recs := make(records, 128)
		for i := range recs {
			r := ranrec()
			recs[i] = r
			new_recs[i] = &record{r.key, randstr(12)}
			err = bpt.Add(r.key, r.value)
			if err != nil {
				t.Error(err)
			}
		}

		for i, r := range recs {
			if has := bpt.Has(r.key); !has {
				t.Error(bpt, "Missing key")
			}
			if has := bpt.Has(randstr(10)); has {
				t.Error("Table has extra key")
			}
			if count := bpt.Count(r.key); count != 1 {
				t.Error(bpt, "Missing key")
			}
			if count := bpt.Count(randstr(10)); count != 0 {
				t.Error("Table has extra key")
			}
			for k, v, next := bpt.Find(r.key)(); next != nil; k, v, next = next() {
				if !k.Equals(r.key) {
					t.Error(bpt, "Find Failed Key Error")
				}
				if !v.(String).Equals(r.value) {
					t.Error(bpt, "Find Failed Value Error")
				}
			}
			err = bpt.Replace(r.key, func(value ItemValue) bool { return true }, new_recs[i].value)
			if err != nil {
				t.Error(err)
			}
		}
		sort.Sort(recs)
		sort.Sort(new_recs)
		i := 0
		for k, v, next := bpt.Iterate()(); next != nil; k, v, next = next() {
			if !recs[i].key.Equals(k) {
				t.Error("iterate error wrong key")
			}
			if !new_recs[i].value.Equals(v.(String)) {
				t.Error("iterate error wrong value")
			}
			i++
		}
		i = len(recs) - 1
		for k, v, next := bpt.Backward()(); next != nil; k, v, next = next() {
			if !recs[i].key.Equals(k) {
				t.Error("iterate error wrong key")
			}
			if !new_recs[i].value.Equals(v.(String)) {
				t.Error("iterate error wrong value")
			}
			i--
		}
		i = 0
		for k, next := bpt.Keys()(); next != nil; k, next = next() {
			if !recs[i].key.Equals(k) {
				t.Error("iterate error wrong key")
			}
			i++
		}
		i = 7
		for k, v, next := bpt.Range(recs[i].key, recs[i+(len(recs)/2)].key)(); next != nil; k, v, next = next() {
			if !recs[i].key.Equals(k) {
				t.Error("iterate error wrong key")
			}
			if !new_recs[i].value.Equals(v.(String)) {
				t.Error("iterate error wrong value")
			}
			i++
		}
		for k, v, next := bpt.Range(recs[i].key, recs[7].key)(); next != nil; k, v, next = next() {
			if !recs[i].key.Equals(k) {
				t.Error("iterate error wrong key")
			}
			if !new_recs[i].value.Equals(v.(String)) {
				t.Error("iterate error wrong value", k, v, recs[i].value, new_recs[i].value)
			}
			i--
		}
		for i, r := range recs {
			if has := bpt.Has(r.key); !has {
				t.Error(bpt, "Missing key")
			}
			if count := bpt.Count(r.key); count != 1 {
				t.Error(bpt, "Missing key")
			}
			if err := bpt.RemoveWhere(r.key, func(value ItemValue) bool { return true }); err != nil {
				t.Fatal(bpt, err)
			}
			if has := bpt.Has(r.key); has {
				t.Error("Table has extra key")
			}
			for _, x := range recs[i+1:] {
				if has := bpt.Has(x.key); !has {
					t.Error(bpt, "Missing key", x.key)
				}
			}
		}
	}
	for i := 2; i < 64; i++ {
		test(NewBpTree(i, nil))
	}
}

func TestBpMap(t *testing.T) {

	ranrec := func() *record {
		return &record{
			randstr(12),
			randstr(12),
		}
	}

	test := func(table MapOperable) {
		recs := make(records, 400)
		for i := range recs {
			r := ranrec()
			recs[i] = r
			err := table.Put(r.key, String(""))
			if err != nil {
				t.Error(err)
			}
			err = table.Put(r.key, r.value)
			if err != nil {
				t.Error(err)
			}
		}

		for _, r := range recs {
			if has := table.Has(r.key); !has {
				t.Error(table, "Missing key")
			}
			if has := table.Has(randstr(12)); has {
				t.Error("Table has extra key")
			}
			if val, err := table.Get(r.key); err != nil {
				t.Error(err)
			} else if !(val.(String)).Equals(r.value) {
				t.Error("wrong value")
			}
		}

		for i, x := range recs {
			if val, err := table.Remove(x.key); err != nil {
				t.Error(err)
			} else if !(val.(String)).Equals(x.value) {
				t.Error("wrong value")
			}
			for _, r := range recs[i+1:] {
				if has := table.Has(r.key); !has {
					t.Error("Missing key")
				}
				if has := table.Has(randstr(12)); has {
					t.Error("Table has extra key")
				}
				if val, err := table.Get(r.key); err != nil {
					t.Error(err)
				} else if !(val.(String)).Equals(r.value) {
					t.Error("wrong value")
				}
			}
		}
	}

	test(NewBpMap(23, nil))
}

func Test_get_start(t *testing.T) {
	root := NewLeaf(2, nil)
	root, err := root.put(Int(1), Int(1))
	if err != nil {
		t.Error(err)
	}
	root, err = root.put(Int(5), Int(3))
	if err != nil {
		t.Error(err)
	}
	root, err = root.put(Int(3), Int(2))
	if err != nil {
		t.Error(err)
	}
	t.Log(root)
	t.Log(root.pointers[0])
	t.Log(root.pointers[1])
	i, n := root.get_start(Int(1))
	if n != root.pointers[0] {
		t.Error("wrong node from get_start")
	}
	if i != 0 {
		t.Error("wrong index from get_start")
	}
	i, n = root.get_start(Int(3))
	if n != root.pointers[0] {
		t.Error("wrong node from get_start")
	}
	if i != 1 {
		t.Error("wrong index from get_start")
	}
	i, n = root.get_start(Int(5))
	if n != root.pointers[1] {
		t.Error("wrong node from get_start")
	}
	if i != 0 {
		t.Error("wrong index from get_start")
	}
	i, n = root.get_start(Int(2))
	if n != root.pointers[0] {
		t.Error("wrong node from get_start")
	}
	if i != 1 {
		t.Error("wrong index from get_start")
	}
	i, n = root.get_start(Int(4))
	t.Log(n)
	if n != root.pointers[1] {
		t.Error("wrong node from get_start")
	}
	if i != 0 {
		t.Error("wrong index from get_start")
	}
	i, n = root.get_start(Int(0))
	if n != root.pointers[0] {
		t.Error("wrong node from get_start")
	}
	if i != 0 {
		t.Error("wrong index from get_start")
	}
	i, n = root.get_start(Int(5))
	if n != root.pointers[1] {
		t.Error("wrong node from get_start")
	}
	if i != 0 {
		t.Error("wrong index from get_start")
	}
}

func Test_get_end(t *testing.T) {
	root := NewLeaf(3, nil)
	root, err := root.put(Int(1), Int(1))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(4), Int(4))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(3), Int(3))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(8), Int(8))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(9), Int(9))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(10), Int(10))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(6), Int(6))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(7), Int(7))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.put(Int(5), Int(5))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(root)
	t.Log(root.pointers[0])
	t.Log(root.pointers[1])
	printTree(root, "")
}

func Test_put_no_root_split(t *testing.T) {
	a := NewLeaf(2, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	p, err := a.put(Int(1), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if !p.has(Int(1)) {
			t.Error("p didn't have the right keys", p)
		}
	}
	p, err = a.put(Int(1), Int(3))

	t.Log(a)
	printTree(a, "")

	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if !p.has(Int(1)) {
			t.Error("p didn't have the right keys", p)
		}
		t.Log(p)
		t.Log(p.getNext())
	}
}

func Test_put_root_split(t *testing.T) {
	a := NewLeaf(2, nil)
	p, err := a.put(Int(1), Int(1))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if !p.has(Int(1)) {
			t.Error("p didn't have the right keys", p)
		}
	}
	p, err = a.put(Int(3), Int(3))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if !p.has(Int(1)) || !p.has(Int(3)) {
			t.Error("p didn't have the right keys", p)
		}
	}
	p, err = a.put(Int(2), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p == a {
			t.Errorf("p == a")
		}
		if !p.has(Int(1)) || !p.has(Int(3)) {
			t.Error("p didn't have the right keys", p)
		}
		if len(p.pointers) != 2 {
			t.Error("p didn't have right number of pointers", p)
		}
		if !p.pointers[0].has(Int(1)) || !p.pointers[0].has(Int(2)) {
			t.Error("p.pointers[0] didn't have the right keys", p.pointers[0])
		}
		if !p.pointers[1].has(Int(3)) {
			t.Error("p.pointers[1] didn't have the right keys", p.pointers[1])
		}
		t.Log(p)
		t.Log(p.pointers[0])
		t.Log(p.pointers[1])
	}
}

func Test_internal_insert_no_split(t *testing.T) {
	a := NewInternal(3, nil)
	leaf := NewLeaf(1, nil)
	if err := leaf.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(1), leaf); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(5), nil); err != nil {
		t.Error(err)
	}
	p, q, err := a.internal_insert(Int(2), nil)
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q != nil {
			t.Errorf("q != nil")
		}
		if !p.has(Int(1)) || !p.has(Int(2)) || !p.has(Int(5)) {
			t.Error("p didn't have the right keys", p)
		}
	}
}

func Test_internal_insert_split_less(t *testing.T) {
	a := NewInternal(3, nil)
	leaf := NewLeaf(1, nil)
	if err := leaf.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(1), leaf); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(3), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(5), nil); err != nil {
		t.Error(err)
	}
	p, q, err := a.internal_insert(Int(2), nil)
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) || !p.has(Int(2)) {
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(3)) || !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

func Test_internal_split_less(t *testing.T) {
	a := NewInternal(3, nil)
	if err := a.put_kp(Int(1), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(3), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(5), nil); err != nil {
		t.Error(err)
	}
	p, q, err := a.internal_split(Int(2), nil)
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) || !p.has(Int(2)) {
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(3)) || !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

func Test_internal_split_equal(t *testing.T) {
	a := NewInternal(3, nil)
	if err := a.put_kp(Int(1), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(3), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(5), nil); err != nil {
		t.Error(err)
	}
	p, q, err := a.internal_split(Int(3), nil)
	if err == nil {
		t.Error("split succeeded should have failed", p, q)
	}
}

func Test_internal_split_greater(t *testing.T) {
	a := NewInternal(3, nil)
	if err := a.put_kp(Int(1), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(3), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(5), nil); err != nil {
		t.Error(err)
	}
	p, q, err := a.internal_split(Int(4), nil)
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) ||  !p.has(Int(3)) || !p.has(Int(4)){
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

func Test_leaf_insert_no_split(t *testing.T) {
	a := NewLeaf(3, nil)
	insert_linked_list_node(a, nil, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	p, q, err := a.leaf_insert(Int(2), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q != nil {
			t.Errorf("q != nil")
		}
		if !p.has(Int(1)) || !p.has(Int(2)) || !p.has(Int(3)) {
			t.Error("p didn't have the right keys", p)
		}
	}
}

// tests the defer to split logic
func Test_leaf_insert_split_less(t *testing.T) {
	a := NewLeaf(3, nil)
	insert_linked_list_node(a, nil, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(5), Int(5)); err != nil {
		t.Error(err)
	}
	p, q, err := a.leaf_insert(Int(2), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) || !p.has(Int(2)) {
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(3)) || !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

func Test_leaf_split_less(t *testing.T) {
	a := NewLeaf(3, nil)
	insert_linked_list_node(a, nil, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(5), Int(5)); err != nil {
		t.Error(err)
	}
	p, q, err := a.leaf_split(Int(2), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) || !p.has(Int(2)) {
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(3)) || !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

func Test_leaf_split_equal(t *testing.T) {
	a := NewLeaf(3, nil)
	insert_linked_list_node(a, nil, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(5), Int(5)); err != nil {
		t.Error(err)
	}
	p, q, err := a.leaf_split(Int(3), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) {
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(3)) || !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

func Test_leaf_split_greater(t *testing.T) {
	a := NewLeaf(3, nil)
	insert_linked_list_node(a, nil, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(5), Int(5)); err != nil {
		t.Error(err)
	}
	p, q, err := a.leaf_split(Int(4), Int(2))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil {
			t.Errorf("q == nil")
		}
		if !p.has(Int(1)) || !p.has(Int(3)) || !p.has(Int(4)) {
			t.Error("p didn't have the right keys", p)
		}
		if !q.has(Int(5)) {
			t.Error("q didn't have the right keys", q)
		}
	}
}

// tests the defer logic
func Test_pure_leaf_insert_split_less(t *testing.T) {
	a := NewLeaf(2, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(2, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(2, nil)
	insert_linked_list_node(d, c, nil)
	if err := a.put_kv(Int(3), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(2)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(4)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(5)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(6)); err != nil {
		t.Error(err)
	}
	if err := d.put_kv(Int(4), Int(6)); err != nil {
		t.Error(err)
	}
	p, q, err := a.leaf_insert(Int(2), Int(1))
	if err != nil {
		t.Error(err)
	} else {
		if q != a {
			t.Errorf("q != a")
		}
		if p == nil || len(p.keys) != 1 || !p.keys[0].Equals(Int(2)) {
			t.Errorf("p did not contain the right key")
		}
		if p.getPrev() != nil {
			t.Errorf("expected p.prev == nil")
		}
		if p.getNext() != a {
			t.Errorf("expected p.next == a")
		}
		if a.getPrev() != p {
			t.Errorf("expected a.prev == p")
		}
		if a.getNext() != b {
			t.Errorf("expected a.next == b")
		}
		if b.getPrev() != a {
			t.Errorf("expected b.prev == a")
		}
		if b.getNext() != c {
			t.Errorf("expected b.next == c")
		}
		if c.getPrev() != b {
			t.Errorf("expected c.prev == b")
		}
		if c.getNext() != d {
			t.Errorf("expected c.next == d")
		}
		if d.getPrev() != c {
			t.Errorf("expected d.prev == c")
		}
		if d.getNext() != nil {
			t.Errorf("expected d.next == nil")
		}
	}
}

func Test_pure_leaf_split_less(t *testing.T) {
	a := NewLeaf(2, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(2, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(2, nil)
	insert_linked_list_node(d, c, nil)
	if err := a.put_kv(Int(3), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(2)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(4)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(5)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(6)); err != nil {
		t.Error(err)
	}
	if err := d.put_kv(Int(4), Int(6)); err != nil {
		t.Error(err)
	}
	p, q, err := a.pure_leaf_split(Int(2), Int(1))
	if err != nil {
		t.Error(err)
	} else {
		if q != a {
			t.Errorf("q != a")
		}
		if p == nil || len(p.keys) != 1 || !p.keys[0].Equals(Int(2)) {
			t.Errorf("p did not contain the right key")
		}
		if p.getPrev() != nil {
			t.Errorf("expected p.prev == nil")
		}
		if p.getNext() != a {
			t.Errorf("expected p.next == a")
		}
		if a.getPrev() != p {
			t.Errorf("expected a.prev == p")
		}
		if a.getNext() != b {
			t.Errorf("expected a.next == b")
		}
		if b.getPrev() != a {
			t.Errorf("expected b.prev == a")
		}
		if b.getNext() != c {
			t.Errorf("expected b.next == c")
		}
		if c.getPrev() != b {
			t.Errorf("expected c.prev == b")
		}
		if c.getNext() != d {
			t.Errorf("expected c.next == d")
		}
		if d.getPrev() != c {
			t.Errorf("expected d.prev == c")
		}
		if d.getNext() != nil {
			t.Errorf("expected d.next == nil")
		}
	}
}

func Test_pure_leaf_split_equal(t *testing.T) {
	a := NewLeaf(2, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(2, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(2, nil)
	insert_linked_list_node(d, c, nil)
	if err := a.put_kv(Int(3), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(2)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(4)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(5)); err != nil {
		t.Error(err)
	}
	if err := d.put_kv(Int(4), Int(6)); err != nil {
		t.Error(err)
	}
	p, q, err := a.pure_leaf_split(Int(3), Int(1))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q != nil {
			t.Errorf("q != nil")
		}
		if a.getPrev() != nil {
			t.Errorf("expected a.prev == nil")
		}
		if a.getNext() != b {
			t.Errorf("expected a.next == b")
		}
		if b.getPrev() != a {
			t.Errorf("expected b.prev == a")
		}
		if b.getNext() != c {
			t.Errorf("expected b.next == c")
		}
		if c.getPrev() != b {
			t.Errorf("expected c.prev == b")
		}
		if c.getNext() != d {
			t.Errorf("expected c.next == d")
		}
		if d.getPrev() != c {
			t.Errorf("expected d.prev == c")
		}
		if d.getNext() != nil {
			t.Errorf("expected d.next == nil")
		}
	}
}

func Test_pure_leaf_split_greater(t *testing.T) {
	a := NewLeaf(2, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(2, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(2, nil)
	insert_linked_list_node(d, c, nil)
	if err := a.put_kv(Int(3), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(2)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(4)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(5)); err != nil {
		t.Error(err)
	}
	if err := d.put_kv(Int(5), Int(6)); err != nil {
		t.Error(err)
	}
	p, q, err := a.pure_leaf_split(Int(4), Int(1))
	if err != nil {
		t.Error(err)
	} else {
		if p != a {
			t.Errorf("p != a")
		}
		if q == nil || len(q.keys) != 1 || !q.keys[0].Equals(Int(4)) {
			t.Errorf("q != nil")
		}
		if a.getPrev() != nil {
			t.Errorf("expected a.prev == nil")
		}
		if a.getNext() != b {
			t.Errorf("expected a.next == b")
		}
		if b.getPrev() != a {
			t.Errorf("expected b.prev == a")
		}
		if b.getNext() != c {
			t.Errorf("expected b.next == c")
		}
		if c.getPrev() != b {
			t.Errorf("expected c.prev == b")
		}
		if c.getNext() != q {
			t.Errorf("expected c.next == q")
		}
		if q.getPrev() != c {
			t.Errorf("expected q.prev == c")
		}
		if q.getNext() != d {
			t.Errorf("expected q.next == d")
		}
		if d.getPrev() != q {
			t.Errorf("expected d.prev == q")
		}
		if d.getNext() != nil {
			t.Errorf("expected d.next == nil")
		}
	}
}

func Test_find_end_of_pure_run(t *testing.T) {
	a := NewLeaf(2, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(2, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(2, nil)
	insert_linked_list_node(d, c, nil)
	if err := a.put_kv(Int(3), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(2)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := b.put_kv(Int(3), Int(4)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(5)); err != nil {
		t.Error(err)
	}
	if err := c.put_kv(Int(3), Int(6)); err != nil {
		t.Error(err)
	}
	if err := d.put_kv(Int(4), Int(6)); err != nil {
		t.Error(err)
	}
	e := a.find_end_of_pure_run()
	if e != c {
		t.Errorf("end of run should have been block c %v %v", e, c)
	}
}

func Test_insert_linked_list_node(t *testing.T) {
	a := NewLeaf(1, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(3, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(4, nil)
	insert_linked_list_node(d, a, b)
	if a.getPrev() != nil {
		t.Errorf("expected a.prev == nil")
	}
	if a.getNext() != d {
		t.Errorf("expected a.next == d")
	}
	if d.getPrev() != a {
		t.Errorf("expected d.prev == a")
	}
	if d.getNext() != b {
		t.Errorf("expected d.next == b")
	}
	if b.getPrev() != d {
		t.Errorf("expected b.prev == d")
	}
	if b.getNext() != c {
		t.Errorf("expected b.next == c")
	}
	if c.getPrev() != b {
		t.Errorf("expected c.prev == b")
	}
	if c.getNext() != nil {
		t.Errorf("expected c.next == nil")
	}
}

func Test_remove_linked_list_node(t *testing.T) {
	a := NewLeaf(1, nil)
	insert_linked_list_node(a, nil, nil)
	b := NewLeaf(2, nil)
	insert_linked_list_node(b, a, nil)
	c := NewLeaf(3, nil)
	insert_linked_list_node(c, b, nil)
	d := NewLeaf(4, nil)
	insert_linked_list_node(d, a, b)
	if a.getPrev() != nil {
		t.Errorf("expected a.prev == nil")
	}
	if a.getNext() != d {
		t.Errorf("expected a.next == d")
	}
	if d.getPrev() != a {
		t.Errorf("expected d.prev == a")
	}
	if d.getNext() != b {
		t.Errorf("expected d.next == b")
	}
	if b.getPrev() != d {
		t.Errorf("expected b.prev == d")
	}
	if b.getNext() != c {
		t.Errorf("expected b.next == c")
	}
	if c.getPrev() != b {
		t.Errorf("expected c.prev == b")
	}
	if c.getNext() != nil {
		t.Errorf("expected c.next == nil")
	}
	remove_linked_list_node(d)
	if a.getPrev() != nil {
		t.Errorf("expected a.prev == nil")
	}
	if a.getNext() != b {
		t.Errorf("expected a.next == b")
	}
	if b.getPrev() != a {
		t.Errorf("expected b.prev == a")
	}
	if b.getNext() != c {
		t.Errorf("expected b.next == c")
	}
	if c.getPrev() != b {
		t.Errorf("expected c.prev == b")
	}
	if c.getNext() != nil {
		t.Errorf("expected c.next == nil")
	}
	remove_linked_list_node(a)
	if b.getPrev() != nil {
		t.Errorf("expected b.prev == nil")
	}
	if b.getNext() != c {
		t.Errorf("expected b.next == c")
	}
	if c.getPrev() != b {
		t.Errorf("expected c.prev == b")
	}
	if c.getNext() != nil {
		t.Errorf("expected c.next == nil")
	}
	remove_linked_list_node(c)
	if b.getPrev() != nil {
		t.Errorf("expected b.prev == nil")
	}
	if b.getNext() != nil {
		t.Errorf("expected b.next == nil")
	}
	remove_linked_list_node(b)
}

func Test_balance_leaf_nodes_with_dup(t *testing.T) {
	a := NewLeaf(3, nil)
	b := NewLeaf(3, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(2), Int(1)); err != nil {
		t.Error(err)
	}
	balance_nodes(a, b, Int(2))
	if !a.has(Int(1)) || a.has(Int(2)) {
		t.Error("a had wrong items", a)
	}
	if !b.has(Int(2)) || b.has(Int(1)) {
		t.Error("a had wrong items", b)
	}
}

func Test_balance_leaf_nodes(t *testing.T) {
	a := NewLeaf(7, nil)
	b := NewLeaf(7, nil)
	if err := a.put_kv(Int(1), Int(1)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(2), Int(2)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(3), Int(3)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(4), Int(4)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(5), Int(5)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(6), Int(6)); err != nil {
		t.Error(err)
	}
	if err := a.put_kv(Int(7), Int(7)); err != nil {
		t.Error(err)
	}
	balance_nodes(a, b, Int(5))
	for i, k := range a.keys {
		if int(k.(Int)) != i+1 {
			t.Errorf("k != %d", i+1)
		}
	}
	for i, k := range b.keys {
		if int(k.(Int)) != 5+i {
			t.Errorf("k != %d", 5+i)
		}
	}
	for i, v := range a.values {
		if int(v.(Int)) != i+1 {
			t.Errorf("k != %d", i+1)
		}
	}
	for i, v := range b.values {
		if int(v.(Int)) != 5+i {
			t.Errorf("v != %d", 5+i)
		}
	}
	t.Log(a)
	t.Log(b)
}

func Test_balance_internal_nodes(t *testing.T) {
	a := NewInternal(6, nil)
	b := NewInternal(6, nil)
	if err := a.put_kp(Int(1), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(2), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(3), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(4), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(5), nil); err != nil {
		t.Error(err)
	}
	if err := a.put_kp(Int(6), nil); err != nil {
		t.Error(err)
	}
	balance_nodes(a, b, Int(4))
	for i, k := range a.keys {
		if int(k.(Int)) != i+1 {
			t.Errorf("k != %d", i+1)
		}
	}
	for i, k := range b.keys {
		if int(k.(Int)) != 3+i+1 {
			t.Errorf("k != %d", 3+i+1)
		}
	}
	t.Log(a)
	t.Log(b)
}


// copied from

// ThreadSafeRand provides a thread safe version of math/rand.Rand using
// the same technique used in the math/rand package to make the top level
// functions thread safe.
func ThreadSafeRand(seed int64) *mrand.Rand {
	return mrand.New(&lockedSource{src: mrand.NewSource(seed).(mrand.Source64)})
}

// from: https://golang.org/src/math/rand/rand.go?s=8161:8175#L317
type lockedSource struct {
	lk  sync.Mutex
	src mrand.Source64
}

func (r *lockedSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.src.Int63()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Uint64() (n uint64) {
	r.lk.Lock()
	n = r.src.Uint64()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}

// seedPos implements Seed for a lockedSource without a race condiiton.
func (r *lockedSource) seedPos(seed int64, readPos *int8) {
	r.lk.Lock()
	r.src.Seed(seed)
	*readPos = 0
	r.lk.Unlock()
}

// read implements Read for a lockedSource without a race condition.
func (r *lockedSource) read(p []byte, readVal *int64, readPos *int8) (n int, err error) {
	r.lk.Lock()
	n, err = read(p, r.src.Int63, readVal, readPos)
	r.lk.Unlock()
	return
}

func read(p []byte, int63 func() int64, readVal *int64, readPos *int8) (n int, err error) {
	pos := *readPos
	val := *readVal
	for n = 0; n < len(p); n++ {
		if pos == 0 {
			val = int63()
			pos = 7
		}
		p[n] = byte(val)
		val >>= 8
		pos--
	}
	*readPos = pos
	*readVal = val
	return
}

// copied from https://sourcegraph.com/github.com/timtadh/data-structures@master/-/blob/test/support.go

type T testing.T

func (t *T) Assert(ok bool, msg string, vars ...ItemValue) {
	if !ok {
		t.Log("\n" + string(debug.Stack()))
		var objects []interface{}
		for _, t := range vars {
			objects = append(objects, t)
		}
		t.Fatalf(msg, objects...)
	}
}

func (t *T) AssertNil(errors ...error) {
	any := false
	for _, err := range errors {
		if err != nil {
			any = true
			t.Log("\n" + string(debug.Stack()))
			t.Error(err)
		}
	}
	if any {
		t.Fatal("assert failed")
	}
}

func RandSlice(length int) []byte {
	slice := make([]byte, length)
	if _, err := crand.Read(slice); err != nil {
		panic(err)
	}
	return slice
}

func RandHex(length int) string {
	return hex.EncodeToString(RandSlice(length / 2))
}

func RandStr(length int) string {
	return string(RandSlice(length))
}