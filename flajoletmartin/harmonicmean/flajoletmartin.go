package main

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/dgryski/go-farm"
)

// Constants
const (
	L            = 32
	NumHashFns   = 256 // Increased number of hash functions for better accuracy
	NumEstimates = 16  // Number of estimates to combine using median
)

// generateFarmHashFunctions creates a slice of hash functions using FarmHash for better distribution.
func generateFarmHashFunctions() [][]func(int) uint64 {
	hashFns := make([][]func(int) uint64, NumEstimates)
	for i := range hashFns {
		hashFns[i] = make([]func(int) uint64, NumHashFns)
	}

	for i := 0; i < NumEstimates; i++ {
		for j := 0; j < NumHashFns; j++ {
			seed := rand.Uint64() // Random seed for each hash function
			hashFns[i][j] = func(seed uint64) func(int) uint64 {
				return func(x int) uint64 {
					return farm.Hash64([]byte(fmt.Sprintf("%d:%d", x, seed)))
				}
			}(seed)
		}
	}

	return hashFns
}

// generateCustomHashFunctions creates a slice of custom hash functions.
func generateCustomHashFunctions() [][]func(int) uint64 {
	hashFns := make([][]func(int) uint64, NumEstimates)
	for i := range hashFns {
		hashFns[i] = make([]func(int) uint64, NumHashFns)
	}

	for i := 0; i < NumEstimates; i++ {
		for j := 0; j < NumHashFns; j++ {
			a := rand.Uint64() | 1 // Ensure odd number for better distribution
			time.Sleep(10 * time.Nanosecond)
			b := rand.Uint64()
			hashFns[i][j] = func(a, b uint64) func(int) uint64 {
				return func(x int) uint64 {
					h := a*uint64(x) + b
					h ^= h >> 33
					h *= 0xff51afd7ed558ccd
					h ^= h >> 33
					h *= 0xc4ceb9fe1a85ec53
					h ^= h >> 33
					return h & ((1 << L) - 1)
				}
			}(a, b)
		}
	}

	return hashFns
}

// getRightmostSetBit returns the position of the rightmost set bit.
func getRightmostSetBit(y uint64) int {
	return bits.TrailingZeros64(y)
}

// cardinalityFMParallel estimates the cardinality of a stream using the improved Flajolet-Martin algorithm.
func cardinalityFMParallel(stream []int, hashFns [][]func(int) uint64) float64 {
	R := make([][]int, NumEstimates)
	locks := make([][]sync.Mutex, NumEstimates) // Locks for each R[index][j]
	for i := range R {
		R[i] = make([]int, NumHashFns)
		locks[i] = make([]sync.Mutex, NumHashFns)
	}

	var wg sync.WaitGroup
	estimates := make([]float64, NumEstimates)
	alpha := 0.7213 / (1.0 + 1.079/float64(NumHashFns))
	for i := 0; i < NumEstimates; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for _, x := range stream {
				for j := range hashFns[index] {
					y := hashFns[index][j](x)
					if y != 0 { // Ensure y is not zero
						locks[index][j].Lock()
						R[index][j] = max(R[index][j], getRightmostSetBit(y)+1)
						locks[index][j].Unlock()
					}
				}
			}

			sum := 0.0
			for _, r := range R[index] {
				val := 1 << r
				sum += 1.0 / float64(val)
			}

			if sum > 0 {
				// The harmonic mean is less sensitive to large outliers in the data compared to the arithmetic mean.
				// This property makes it particularly suitable for averaging ratios or rates, and in this context,
				// it helps in aggregating the hash-based estimates of the number of distinct elements.
				harmonicMean := 1 / sum
				estimates[index] = alpha * float64(NumHashFns) * harmonicMean
			} else {
				estimates[index] = 0
			}
		}(i)
	}

	wg.Wait()

	sort.Float64s(estimates)
	return estimates[len(estimates)/2]
}

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	hashFns := generateCustomHashFunctions()
	//hashFns := generateFarmHashFunctions()

	// Test with different cardinalities
	cardinalities := []int{1000, 10000, 100000, 1000000, 10000000}

	for _, trueCardinality := range cardinalities {
		set := make(map[int]struct{})
		stream := make([]int, 0, trueCardinality)

		// Generate a stream with known cardinality
		for len(set) < trueCardinality {
			val := rand.Intn(trueCardinality * 10) // Adjust range to ensure unique elements
			stream = append(stream, val)
			if _, exists := set[val]; !exists {
				set[val] = struct{}{}
			}
		}

		// Estimate the cardinality of the stream
		estimatedCardinality := cardinalityFMParallel(stream, hashFns)

		// Calculate relative error
		relativeError := math.Abs(float64(trueCardinality)-estimatedCardinality) / float64(trueCardinality) * 100

		//fmt.Printf("Length of stream: %d\n", len(stream))
		fmt.Printf("True Cardinality: %d, Estimated Cardinality: %.2f, Relative Error: %.2f%%\n",
			trueCardinality, estimatedCardinality, relativeError)
	}
}
