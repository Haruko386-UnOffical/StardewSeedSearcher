package Core

import (
	"encoding/binary"
	"github.com/pierrec/xxHash/xxHash32"
)

// HashHelper 哈希和随机种子计算辅助类，基于 UnderScore76的实现
type HashHelper struct{}

// GetHashFromString 获取字符串的哈希值
func GetHashFromString(value string) int32 {
	data := []byte(value)
	return GetHashFromBytes(data)
}

// GetHashFromArray 获取整数数组的固希值
func GetHashFromArray(values ...int32) int32 {
	data := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(data[i*4:], uint32(v))
	}
	return GetHashFromBytes(data)
}

// GetHashFromBytes 获取字节数组的哈希值
func GetHashFromBytes(value []byte) int32 {
	// 星露谷底层的 xxHash32 使用默认种子 0
	h := xxHash32.New(0)
	h.Write(value)
	// 强制转换为有符号的 int32，与 C# BitConverter.ToInt32 保持一致
	return int32(h.Sum32())
}

// GetRandomSeed 计算随机种子
// useLegacyRandom 是否使用旧随机
func GetRandomSeed(a, b, c, d, e int32, useLegacyRandom bool) int32 {
	const modValue int64 = 2147483647

	a %= int32(modValue)
	b %= int32(modValue)
	c %= int32(modValue)
	d %= int32(modValue)
	e %= int32(modValue)

	if useLegacyRandom {
		sum := int64(a) + int64(b) + int64(c) + int64(d) + int64(e)
		return int32(sum % modValue)
	}
	return GetHashFromArray(a, b, c, d, e)
}
