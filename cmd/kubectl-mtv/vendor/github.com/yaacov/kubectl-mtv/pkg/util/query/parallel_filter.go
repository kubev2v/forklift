package query

import (
	"fmt"
	"runtime"
	"sync"
)

// CalculateBatchSize determines an appropriate batch size based on the total items
// and available CPU cores. If providedBatchSize > 0, it uses that instead.
func CalculateBatchSize(totalItems int, providedBatchSize int) int {
	if providedBatchSize > 0 {
		return providedBatchSize
	}

	// Get number of available CPU cores
	numCPU := runtime.NumCPU()

	// Target 4 batches per core for good parallelism and load balancing
	targetBatches := numCPU * 4
	batchSize := totalItems / targetBatches

	// Ensure reasonable minimum and maximum batch sizes
	if batchSize < 100 && totalItems > 100 {
		batchSize = 100
	} else if batchSize < 1 {
		batchSize = 1
	}

	return batchSize
}

// FilterItemsParallel filters items in parallel batches
func FilterItemsParallel(items []map[string]interface{}, queryOpts *QueryOptions, batchSize int) ([]map[string]interface{}, error) {
	totalItems := len(items)
	if totalItems == 0 {
		return []map[string]interface{}{}, nil
	}

	tree, err := ParseWhereClause(queryOpts.Where)
	if err != nil {
		return nil, err
	}

	effectiveBatchSize := CalculateBatchSize(totalItems, batchSize)
	numBatches := (totalItems + effectiveBatchSize - 1) / effectiveBatchSize

	// Create channels for results and errors
	resultsChan := make(chan []map[string]interface{}, numBatches)
	errorsChan := make(chan error, numBatches)
	var wg sync.WaitGroup

	// Process each batch in a goroutine
	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go func(batchIndex int) {
			defer wg.Done()

			// Calculate start and end indices for this batch
			start := batchIndex * effectiveBatchSize
			end := start + effectiveBatchSize
			if end > totalItems {
				end = totalItems
			}

			// Process the batch using ApplyFilter
			filteredBatch, err := ApplyFilter(items[start:end], tree, queryOpts.Select)
			if err != nil {
				errorsChan <- fmt.Errorf("error filtering batch %d: %v", batchIndex, err)
				return
			}

			resultsChan <- filteredBatch
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Check for any errors
	for err := range errorsChan {
		if err != nil {
			return nil, err
		}
	}

	// Collect and combine all results
	var allResults []map[string]interface{}
	for batchResult := range resultsChan {
		allResults = append(allResults, batchResult...)
	}

	return allResults, nil
}
