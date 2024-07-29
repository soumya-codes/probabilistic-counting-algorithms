package main

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"os"
	"time"
	"unsafe"

	"github.com/cespare/xxhash"
)

const (
	minPrecision            = 2
	maxPrecision            = 18
	sparsePrecision         = 25
	linearCountingThreshold = 5.0
	twoTo32                 = 1 << 32
	estimateThreshold       = twoTo32 / 30.0
)

type HyperLogLog struct {
	p         uint8            // Precision Value
	m         uint32           // Number of Registers = 2^p
	alpha     float64          // bias correction constant
	sparse    map[uint32]uint8 // Sparse representation: maps register index to its value
	registers []uint8          // Dense representation: array of register values
}

func NewHyperLogLog(precision uint8, isSparse bool) (*HyperLogLog, error) {
	if precision < minPrecision || precision > maxPrecision {
		return nil, fmt.Errorf("precision must be between %d and %d", minPrecision, maxPrecision)
	}

	m := uint32(1 << precision)
	hll := &HyperLogLog{
		p:     precision,
		m:     m,
		alpha: getAlpha(m),
	}

	if isSparse {
		hll.sparse = make(map[uint32]uint8)
	} else {
		hll.registers = make([]uint8, m)
	}

	return hll, nil
}

// getAlpha calculates the bias correction factor based on the number of registers.
// These bias correction factor values are well known and arrived upon by statistical analysis and extensive empirical
// testing of the HyperLogLog algorithm.
func getAlpha(m uint32) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1 + 1.079/float64(m))
	}
}

// Add potentially adds a new value to the HyperLogLog estimation data-structure for each stream value.
func (hll *HyperLogLog) Add(value string) {
	hash := xxhash.Sum64([]byte(value))
	if hll.sparse != nil {
		hll.addSparse(hash)
	} else {
		hll.addDense(hash)
	}
}

// addSparse adds a value to sparse-representation(map of ρ's) of HyperLogLog.
// Triggers a transition to dense representation if the sparse map becomes too large.
func (hll *HyperLogLog) addSparse(hash uint64) {
	// find the register(index) based on precision
	idx := hash >> (64 - sparsePrecision)
	// find the position of the leftmost 1-bit in the hash
	rho := uint8(bits.LeadingZeros64(hash<<sparsePrecision|1)) + 1

	// By using the maximum ρ value for each register, capture information about the rarest events, which are
	// most informative about the total number of unique elements.
	if oldRho, found := hll.sparse[uint32(idx)]; !found || rho > oldRho {
		hll.sparse[uint32(idx)] = rho
	}

	// Transition to dense based precision if the number of indexes having non-zero values becomes high
	if hll.shouldTransitionToDense() {
		hll.toDense()
	}
}

// dynamic threshold based on precision to switch from sparse to dense mode
//func (hll *HyperLogLog) shouldTransitionToDense() bool {
//	threshold := float64(hll.m) * (1 - math.Pow(0.9, float64(hll.p)))
//	return float64(len(hll.sparse)) > threshold
//}

// memory-based threshold to switch from sparse to dense mode
func (hll *HyperLogLog) shouldTransitionToDense() bool {
	sparseMemoryUsage := len(hll.sparse) * int(unsafe.Sizeof(uint32(0))+unsafe.Sizeof(uint8(0)))
	denseMemoryUsage := int(hll.m) * int(unsafe.Sizeof(uint8(0)))
	return sparseMemoryUsage >= denseMemoryUsage
}

// addDense adds a value to the dense-representation(array) of HyperLogLog.
func (hll *HyperLogLog) addDense(hash uint64) {
	// find the register(index) based on precision
	idx := hash >> (64 - hll.p)
	// find the position of the leftmost 1-bit in the hash
	rho := uint8(bits.LeadingZeros64(hash<<hll.p|1)) + 1

	// By using the maximum ρ value for each register, it captures information about the rarest events, which are
	// most informative about the total number of unique elements.
	if rho > hll.registers[idx] {
		hll.registers[idx] = rho
	}
}

// convert HyperLogLog from sparse to dense representation.
func (hll *HyperLogLog) toDense() {
	hll.registers = make([]uint8, hll.m)
	for idx, rho := range hll.sparse {
		i := idx >> (sparsePrecision - hll.p)
		if rho > hll.registers[i] {
			hll.registers[i] = rho
		}
	}

	hll.sparse = nil // Free the memory used by the sparse representation
}

func (hll *HyperLogLog) Estimate() float64 {
	if hll.sparse != nil {
		return hll.estimateSparse()
	}
	return hll.estimateDense()
}

func (hll *HyperLogLog) estimateSparse() float64 {
	sum := 0.0
	// The sum of 2^(-ρ) across all registers provides a statistically sound basis for estimating the total number
	// of unique elements needed to produce such a distribution of ρ values
	for _, rho := range hll.sparse {
		sum += math.Pow(2, -float64(rho))
	}

	// v is the number of registers with a non-zero value
	// v^2 / sum is the harmonic mean of the 2^ρ values and alpha is a correction factor
	v := float64(len(hll.sparse))
	estimate := hll.alpha * v * v / sum

	// For low cardinalities(sparse data, when many registers are still zero), use linear counting to provide better accuracy.
	if estimate <= linearCountingThreshold*float64(hll.m) {
		return hll.linearCounting(int(v))
	}
	return estimate
}

func (hll *HyperLogLog) estimateDense() float64 {
	sum := 0.0
	zeros := 0
	for _, val := range hll.registers {
		// The sum of 2^(-ρ) across all registers provides a statistically sound basis for estimating the total number
		// of unique elements needed to produce such a distribution of ρ values
		sum += math.Pow(2, -float64(val))
		if val == 0 {
			zeros++
		}
	}

	// m^2 / sum is the harmonic mean of the 2^ρ values and alpha is a correction factor
	estimate := hll.alpha * float64(hll.m*hll.m) / sum

	if estimate <= linearCountingThreshold*float64(hll.m) {
		// For low cardinalities, use linear counting to provide better accuracy when many registers are still zero.
		return hll.linearCounting(zeros)
	} else if estimate <= estimateThreshold {
		// For normal cardinalities, use linear counting to provide better accuracy when many ρ value registers are still zero.
		return estimate
	} else {
		// For very high cardinalities (approaching 2^32), apply an additional correction to account for the loss of
		// precision in floating-point calculations.
		return -twoTo32 * math.Log(1-estimate/twoTo32)
	}
}

// linearCounting cardinality estimate more accurate than HyperLogLog for small cardinalities.
func (hll *HyperLogLog) linearCounting(zeros int) float64 {
	// avoid log(0)
	if zeros == 0 {
		return float64(hll.m)
	}

	return float64(hll.m) * math.Log(float64(hll.m)/float64(zeros))
}

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	cardinalities := []int{1000, 10000, 100000, 1000000, 10000000, 100000000}
	for _, trueCardinality := range cardinalities {
		precision := getPrecisionForCardinality(trueCardinality)

		// toggle the second parameter to true for sparse and false for dense representation
		hll, err := NewHyperLogLog(precision, false)
		if err != nil {
			fmt.Printf("Error creating HyperLogLog: %v\n", err)
			os.Exit(1)
		}

		set := make(map[string]struct{})

		for len(set) < trueCardinality {
			val := fmt.Sprintf("%d", rand.Intn(trueCardinality*100))
			hll.Add(val)
			if _, ok := set[val]; !ok {
				set[val] = struct{}{}
			}
		}

		estimatedCardinality := hll.Estimate()
		relativeError := math.Abs(float64(trueCardinality)-estimatedCardinality) / float64(trueCardinality) * 100

		//fmt.Printf("Length of stream: %d\n", len(stream))
		fmt.Printf("True Cardinality: %d, Estimated Cardinality: %.2f, Relative Error: %.2f%%\n",
			trueCardinality, estimatedCardinality, relativeError)
	}
}

// getPrecisionForCardinality returns appropriate precision value based on the expected cardinality.
func getPrecisionForCardinality(cardinality int) uint8 {
	switch {
	case cardinality <= 10000:
		return 12 // Medium precision for medium-sized sets
	case cardinality <= 1000000:
		return 13 // Medium precision for medium-sized sets
	default:
		return 14 // Lower precision for large sets to save memory
	}
}
