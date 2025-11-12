package zset

import "testing"

func TestNewZSet(t *testing.T) {
	zset := NewZSet()
	if zset == nil {
		t.Fatal("无法创建新的zset")
	}
	if zset.Encoding() != encodingListpack {
		t.Fatal("zset的encoding属性应该为encodingListpack")
	}
	if zset.Len() != 0 {
		t.Fatal("zset的长度应该为0")
	}

}
