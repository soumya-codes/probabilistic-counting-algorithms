# Probabilistic Counting Algorithms Comparison

This project compares and contrasts few implementations of Probabilistic Counting Algorithms, specifically Flajolet-Martin (two versions) and HyperLogLog.

## 1. Flajolet-Martin (Version 1)

This version uses the _**arithmetic-mean**_ approach to estimate the cardinality of stream data.

### Observations:

1. Requires very high `NumHashFns` and `NumEstimates` for reasonable estimates, making it computationally intensive.
2. For similar `NumHashFns` and `NumEstimates`, the relative error is significantly higher compared to the harmonic mean version.
3. This implementation suffers from high variance in relative error across runs. 
4. Standard hash functions (e.g., MurmurHash3) provide better accuracy compared to custom hash functions but are exponentially more expensive computationally.

### Sample Outputs:

#### Low Settings (NumHashFns = 16, NumEstimates = 2)
```output
True Cardinality: 1000, Estimated Cardinality: 803.56, Relative Error: 19.64%
True Cardinality: 10000, Estimated Cardinality: 121650.65, Relative Error: 1116.51%
True Cardinality: 100000, Estimated Cardinality: 257830.19, Relative Error: 157.83%
True Cardinality: 1000000, Estimated Cardinality: 666022.89, Relative Error: 33.40%
```

#### High Settings (NumHashFns = 256, NumEstimates = 16)
```output
True Cardinality: 1000, Estimated Cardinality: 1167.51, Relative Error: 16.75%
True Cardinality: 10000, Estimated Cardinality: 13751.58, Relative Error: 37.52%
True Cardinality: 100000, Estimated Cardinality: 130673.02, Relative Error: 30.67%
True Cardinality: 1000000, Estimated Cardinality: 1287211.18, Relative Error: 28.72%
```

## 2. Flajolet-Martin (Version 2)

This version uses the _**harmonic-mean**_ approach to estimate the cardinality of stream data.

### Observations:

1. Requires very high `NumHashFns` and `NumEstimates` for close estimates(0% to 3% Relative Error in most cases), making it computationally intensive.
2. For similar `NumHashFns` and `NumEstimates`, the relative error is significantly lower compared to the arithmetic mean version.
3. At high settings the relative error is consistently low(low single digit), making it more reliable for accurate cardinality estimation.
4. Standard hash functions (e.g., MurmurHash3) provide better accuracy compared to custom hash functions but are exponentially more expensive computationally.

### Sample Outputs:

#### Low Settings (NumHashFns = 16, NumEstimates = 2)
```output
True Cardinality: 1000, Estimated Cardinality: 1278.98, Relative Error: 27.90%
True Cardinality: 10000, Estimated Cardinality: 10595.21, Relative Error: 5.95%
True Cardinality: 100000, Estimated Cardinality: 110334.55, Relative Error: 10.33%
True Cardinality: 1000000, Estimated Cardinality: 1105509.99, Relative Error: 10.55%
```

#### High Settings (NumHashFns = 256, NumEstimates = 16)
```output
True Cardinality: 1000, Estimated Cardinality: 1012.54, Relative Error: 1.25%
True Cardinality: 10000, Estimated Cardinality: 10046.82, Relative Error: 0.47%
True Cardinality: 100000, Estimated Cardinality: 97571.20, Relative Error: 2.43%
True Cardinality: 1000000, Estimated Cardinality: 984091.37, Relative Error: 1.59%
```

## 3. HyperLogLog

This implementation uses a more sophisticated approach that builds upon concepts from the Flajolet-Martin algorithm.

### Implementation Details:
1. The algorithm adjusts precision based on expected cardinality, providing more accurate estimates.
2. The algorithm can adaptively switch from sparse to dense representation based on memory usage and cardinality.
3. The algorithm supports multiple precision levels, adapting to different cardinality ranges.
4. The algorithm uses sparse representation(HashMap) for low cardinalities, saving memory.
5. The algorithm employs different estimation methods (linear counting, HyperLogLog, and a corrected version) based on the estimated cardinality.
6. Uses different precision levels for sparse and dense representations to balance memory usage and accuracy.


### Observations:
1. Requires higher precision (i.e., more memory) for accurate estimates. 
2. Precision of around 14 for dense representation, while a precision of 25 for sparse representation provides a good balance between accuracy and memory usage.
3. Low variance in relative error, ranging from low single digits to low double digits.
4. Can handle much larger cardinalities (up to 100 million in test cases) more efficiently than Flajolet-Martin implementations.
5. No need for multiple iterations(estimations), making it much less compute intensive than Flajolet-Martin implementations.
6. The implementation uses xxHash as the hash function, providing a good balance between speed and distribution quality.
7. The algorithm has high implementation complexity and has relatively high memory overhead, specifically for low cardinalities.

### Sample Outputs (Precision 14): 
```output
True Cardinality: 1000, Estimated Cardinality: 1002.19, Relative Error: 0.22%
True Cardinality: 10000, Estimated Cardinality: 10134.41, Relative Error: 0.34%
True Cardinality: 100000, Estimated Cardinality: 100607.30, Relative Error: 0.61%
True Cardinality: 1000000, Estimated Cardinality: 999304.67, Relative Error: 0.07%
True Cardinality: 10000000, Estimated Cardinality: 9986980.14, Relative Error: 0.13%
```

## Observations on Probabilistic Counting Algorithms
1. Harmonic vs. Arithmetic Mean:
    - Flajolet-Martin using the harmonic mean implementation provides more accurate estimates compared to the implementation using using the arithmetic mean.
    - Both the arithmetic and harmonic mean version requires higher `NumHashFns` and `NumEstimates` but offers better accuracy with lower relative error.
    - The arithmetic mean version has higher relative error.

2. Computational Complexity:
    - Standard hash functions (e.g., MurmurHash3) generally have higher computational complexity compared to custom implementations.
    - This complexity difference can exponentially impact performance, especially when processing large datasets or when used in time-sensitive applications.

3. Accuracy vs. Dataset Size:
    - Standard hash functions typically provide higher accuracy due to better random distribution.
    - However, this accuracy advantage is most noticeable with very large datasets.
    - For smaller to medium-sized datasets, the accuracy difference between standard vs well-designed custom hash functions may be negligible.

4. Custom Hash Function Optimization:
    - The custom hash function used in this project is specifically optimized for:
      a) Speed: Prioritizing fast execution to handle high-throughput data streams.
      b) Distribution quality: Providing sufficiently good distribution for accurate cardinality estimation.
    - This optimization strikes a balance between performance and accuracy.

5. Trade-off Analysis:
    - The selection of a hash function for probabilistic counting involves a careful trade-off between:
        - Computational efficiency
        - Memory usage
        - Accuracy of cardinality estimation
        - Scale of the dataset
    - Custom hash function provides a good balance for many practical applications, while standard hash functions may be preferable for more applications that need high level of accuracy.