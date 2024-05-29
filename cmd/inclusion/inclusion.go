package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/layers"

	"bufio"
)

var (
	dbPath  = flag.String("db", "", "database path")
	from    = flag.Int("from", 0, "from layer")
	to      = flag.Int("to", 0, "to layer")
	batches = flag.Int("batches", 1, "number of batches")
	every   = flag.Int("every", 0, "every layer")
	toFile  = flag.String("toFile", "result.csv", "output file")
)

type Result struct {
	fromLayer          int
	toLayer            int
	averageInclusion   float64
	inclusionFromList  float64
	totalCoinbases     uint64
	inclusionNotInList float64
	totalNotCoinbases  uint64
}

func loadATXListFromFile(fpath string) ([]types.Address, error) {
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var coinbaseList []types.Address
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		coinbase := scanner.Text()
		notHex, err := hex.DecodeString(coinbase)
		var address types.Address
		if err != nil {
			address, err = types.StringToAddress(coinbase)
			if err != nil {
				return nil, err
			}
		} else {
			address = types.Address(notHex)
		}
		coinbaseList = append(coinbaseList, address)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return coinbaseList, nil
}

func calculateInclusionRates(db *sql.Database, fromLayer, toLayer int) (Result, error) {
	var (
		included             float64
		includedCoinbases    float64
		includedNotCoinbases float64
		total                float64
		totalCoinbases       float64
		totalNotCoinbases    float64
	)
	coinbaseList, err := loadATXListFromFile("coinbase_list.txt")
	if err != nil {
		return Result{}, err
	}
	coinbaseSet := make(map[string]struct{})
	for _, coinbase := range coinbaseList {
		coinbaseSet[coinbase.String()] = struct{}{}
	}
	for i := fromLayer; i <= toLayer; i++ {
		id, err := layers.GetApplied(db, types.LayerID(i))
		if err != nil {
			return Result{}, err
		}
		if id != types.EmptyBlockID {
			block, err := blocks.Get(db, id)
			if err != nil {
				return Result{}, err
			}
			included += float64(len(block.Rewards))
			for _, reward := range block.Rewards {
				atx, err := atxs.Get(db, reward.AtxID)
				if err != nil {
					return Result{}, err
				}
				coinbase := atx.Coinbase.String()
				if _, ok := coinbaseSet[coinbase]; ok {
					includedCoinbases++
				} else {
					includedNotCoinbases++
				}
			}
		}

		ballots, err := ballots.Layer(db, types.LayerID(i))
		if err != nil {
			return Result{}, err
		}
		for _, ballot := range ballots {
			if ballot.IsMalicious() {
				continue
			}
			total++
			atx, err := atxs.Get(db, ballot.AtxID)
			if err != nil {
				return Result{}, err
			}
			coinbase := atx.Coinbase.String()
			if _, ok := coinbaseSet[coinbase]; ok {
				totalCoinbases++
			} else {
				totalNotCoinbases++
			}
		}
	}
	fmt.Printf("from = %d to = %d average inclusion %f\n    from list %f (%d)\n    not in list %f (%d)\n", fromLayer, toLayer, included/total, includedCoinbases/totalCoinbases, uint64(totalCoinbases), includedNotCoinbases/totalNotCoinbases, uint64(totalNotCoinbases))
	return Result{
		fromLayer:          fromLayer,
		toLayer:            toLayer,
		averageInclusion:   included / total,
		inclusionFromList:  includedCoinbases / totalCoinbases,
		totalCoinbases:     uint64(totalCoinbases),
		inclusionNotInList: includedNotCoinbases / totalNotCoinbases,
		totalNotCoinbases:  uint64(totalNotCoinbases),
	}, nil
}

func writeCsv(result Result) error {
	file, err := os.OpenFile(*toFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	_, err = writer.WriteString(fmt.Sprintf("%d,%d,%f,%f,%d,%f,%d\n", result.fromLayer, result.toLayer, result.averageInclusion, result.inclusionFromList, result.totalCoinbases, result.inclusionNotInList, result.totalNotCoinbases))
	return err
}

func main() {
	flag.Parse()
	db, err := sql.Open("file:" + *dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if *from <= 0 {
		*from += *to
	}

	if *every > 0 {
		for i := *from; i <= *to; i += *every {
			result, err := calculateInclusionRates(db, i, i+*every-1)
			if err != nil {
				log.Fatalf("failed to calculate inclusion rates: %v", err)
			}
			if err := writeCsv(result); err != nil {
				log.Fatalf("failed to write CSV: %v", err)
			}
		}
	} else {
		batchSize := (*to - *from + 1) / *batches
		for batch := 0; batch < *batches; batch++ {
			startLayer := *from + batch*batchSize
			endLayer := startLayer + batchSize - 1
			if batch == *batches-1 {
				endLayer = *to
			}
			result, err := calculateInclusionRates(db, startLayer, endLayer)
			if err != nil {
				log.Fatalf("failed to calculate inclusion rates: %v", err)
			}
			if err := writeCsv(result); err != nil {
				log.Fatalf("failed to write CSV: %v", err)
			}
		}
	}
}
