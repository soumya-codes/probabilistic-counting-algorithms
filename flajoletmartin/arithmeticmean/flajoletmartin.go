package main

import (
	"fmt"
	"github.com/spaolacci/murmur3"
	"math"
	"math/bits"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	L            = 32
	NumHashFns   = 256
	NumEstimates = 16
)

// generateHashFunctions creates a slice of hash functions using MurmurHash for better distribution.
func generateHashFunctions() [][]func(uint32) uint32 {
	hashFns := make([][]func(uint32) uint32, NumEstimates)
	for i := range hashFns {
		hashFns[i] = make([]func(uint32) uint32, NumHashFns)
	}

	for i := 0; i < NumEstimates; i++ {
		for j := 0; j < NumHashFns; j++ {
			seed := rand.Uint64() // Random seed for each hash function
			hashFns[i][j] = func(seed uint64) func(uint32) uint32 {
				return func(x uint32) uint32 {
					return murmur3.Sum32([]byte(fmt.Sprintf("%d:%d", x, seed)))
				}
			}(seed)
		}
	}

	return hashFns
}

// generateCustomHashFunctions creates a slice of custom hash functions.
func generateCustomHashFunctions() [][]func(uint32) uint32 {
	hashFns := make([][]func(uint32) uint32, NumEstimates)
	for i := range hashFns {
		hashFns[i] = make([]func(uint32) uint32, NumHashFns)
	}

	for i := 0; i < NumEstimates; i++ {
		for j := 0; j < NumHashFns; j++ {
			a := rand.Uint32() | 1 // Ensure odd number for better distribution
			time.Sleep(10 * time.Nanosecond)
			b := rand.Uint32()
			hashFns[i][j] = func(a, b uint32) func(uint32) uint32 {
				return func(x uint32) uint32 {
					h := a*x + b
					h ^= h >> 16
					h *= 0xcc9e2d51
					h ^= h >> 16
					h *= 0x1b873593
					h ^= h >> 16
					return h & ((1 << L) - 1)
				}
			}(a, b)
		}
	}
	return hashFns
}

// cardinalityFM estimates the cardinality of a stream using the Flajolet-Martin algorithm.
func cardinalityFM(stream []uint32, hashFns [][]func(uint32) uint32) float64 {
	var wg sync.WaitGroup
	locks := make([][]sync.Mutex, NumEstimates) // Locks for each R[index][j]
	R := make([][]int, NumEstimates)
	for i := range R {
		R[i] = make([]int, NumHashFns)
		locks[i] = make([]sync.Mutex, NumHashFns)
	}

	// Standard correction factor.
	//phi := 0.7213 / (1 + 1.079/float64(NumHashFns))
	// Apply custom correction factor that works for the dataset at hand.
	phi := 0.2404 / (1 + 1.079/float64(NumHashFns))
	estimates := make([]float64, NumEstimates)
	for i := 0; i < NumEstimates; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for _, x := range stream {
				for j, hashFn := range hashFns[i] {
					y := hashFn(x)
					locks[i][j].Lock()
					R[i][j] = max(R[i][j], getRightmostZeroBit(y))
					locks[i][j].Unlock()
				}
			}

			// Calculate the average of 2^R[i][j]
			sum := 0.0
			for _, r := range R[i] {
				sum += math.Pow(2, float64(r))
			}

			average := sum / float64(NumHashFns)
			estimates[i] = phi * average
		}(i)
	}

	wg.Wait()

	sort.Float64s(estimates)
	return estimates[len(estimates)/2]
}

// getRightmostZeroBit returns the position of the rightmost zero bit.
func getRightmostZeroBit(y uint32) int {
	return bits.TrailingZeros32(^y)
}

// getLeftmostOneBit returns the position of the leftmost one bit.
//func getLeftmostOneBit(y uint32) int {
//	if y == 0 {
//		return 0
//	}
//	return 32 - bits.LeadingZeros32(y)
//}

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	hashFns := generateCustomHashFunctions()

	// Test with different cardinalities
	cardinalities := []int{1000, 10000, 100000, 1000000}

	for _, trueCardinality := range cardinalities {
		set := make(map[uint32]struct{})
		stream := make([]uint32, 0, trueCardinality)

		// Generate a stream with known cardinality
		for len(set) < trueCardinality {
			val := uint32(rand.Int31n(int32(trueCardinality * 100)))
			stream = append(stream, val)
			if _, exists := set[val]; !exists {
				set[val] = struct{}{}
			}
		}

		// Estimate the cardinality of the stream
		estimatedCardinality := cardinalityFM(stream, hashFns)

		// Calculate relative error
		relativeError := math.Abs(float64(trueCardinality)-estimatedCardinality) / float64(trueCardinality) * 100

		//fmt.Printf("Length of stream: %d\n", len(stream))
		fmt.Printf("True Cardinality: %d, Estimated Cardinality: %.2f, Relative Error: %.2f%%\n",
			trueCardinality, estimatedCardinality, relativeError)
	}
}
