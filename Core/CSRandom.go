package Core

import "math"

const (
	mbig  = 2147483647
	mseed = 161803398
)

// CSRandom 是 C# System.Random 的 1:1 Go 语言完美移植版
// 采用 Knuth 的减法伪随机数生成算法
type CSRandom struct {
	inext     int
	inextp    int
	seedArray [56]int
}

// NewCSRandom 使用指定的种子初始化新的 C# 兼容随机数生成器
func NewCSRandom(seed int32) *CSRandom {
	r := &CSRandom{}
	r.init(int(seed))
	return r
}

func (r *CSRandom) init(seed int) {
	var ii, mj, mk int
	subtraction := seed
	if subtraction < 0 {
		subtraction = math.MaxInt32 - subtraction
		// 防止 int32 溢出
		if subtraction < 0 {
			subtraction = math.MaxInt32
		}
	}
	mj = mseed - subtraction
	r.seedArray[55] = mj
	mk = 1
	for i := 1; i < 55; i++ {
		ii = (21 * i) % 55
		r.seedArray[ii] = mk
		mk = mj - mk
		if mk < 0 {
			mk += mbig
		}
		mj = r.seedArray[ii]
	}
	for k := 1; k < 5; k++ {
		for i := 1; i <= 55; i++ {
			r.seedArray[i] -= r.seedArray[1+(i+30)%55]
			if r.seedArray[i] < 0 {
				r.seedArray[i] += mbig
			}
		}
	}
	r.inext = 0
	r.inextp = 21
}

// internalSample 内部核心采样方法
func (r *CSRandom) internalSample() int {
	r.inext++
	if r.inext >= 56 {
		r.inext = 1
	}
	r.inextp++
	if r.inextp >= 56 {
		r.inextp = 1
	}
	retVal := r.seedArray[r.inext] - r.seedArray[r.inextp]
	if retVal == mbig {
		retVal--
	}
	if retVal < 0 {
		retVal += mbig
	}
	r.seedArray[r.inext] = retVal
	return retVal
}

// Next 对应 C# rng.Next()，返回一个大于等于0且小于最大Int32的整数
func (r *CSRandom) Next() int {
	return r.internalSample()
}

// NextUpperBound 对应 C# rng.Next(maxValue)，返回 0 到 maxValue-1 之间的整数
func (r *CSRandom) NextUpperBound(maxValue int) int {
	if maxValue < 0 {
		return 0
	}
	return int(float64(r.internalSample()) * (1.0 / mbig) * float64(maxValue))
}

// NextRange 对应 C# rng.Next(minValue, maxValue)，返回 minValue 到 maxValue-1 之间的整数
func (r *CSRandom) NextRange(minValue, maxValue int) int {
	if minValue > maxValue {
		return minValue
	}
	rangeVal := maxValue - minValue
	if rangeVal <= 1 {
		return minValue
	}
	return minValue + int(float64(r.internalSample())*(1.0/mbig)*float64(rangeVal))
}

// NextDouble 对应 C# rng.NextDouble()，返回 0.0 到 1.0 之间的浮点数
func (r *CSRandom) NextDouble() float64 {
	return float64(r.internalSample()) * (1.0 / mbig)
}
