package main

import (
	"encoding/hex"
	"flag"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/layers"

	"bufio"
	"os"
)

var (
	db      = flag.String("db", "", "database path")
	from    = flag.Int("from", 0, "from layer")
	to      = flag.Int("to", 0, "to layer")
	batches = flag.Int("batches", 1, "number of batches")
	every   = flag.Int("every", 0, "every layer")
)

func loadATXListFromFile() []types.Address {
	file, err := os.Open("coinbase_list.txt")
	must(err)
	defer file.Close()

	var coinbaseList []types.Address
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		coinbase := scanner.Text()
		notHex, err := hex.DecodeString(coinbase)
		must(err)
		address := types.Address(notHex)
		coinbaseList = append(coinbaseList, address)
	}
	must(scanner.Err())
	return coinbaseList
}

func calculateInclusionRates(db *sql.Database, fromLayer, toLayer int) {
	var (
		included               float64
		included_coinbases     float64
		included_not_coinbases float64
		total                  float64
		total_coinbases        float64
		total_not_coinbases    float64
	)
	coinbaseList := loadATXListFromFile()
	coinbaseSet := make(map[string]struct{})
	for _, coinbase := range coinbaseList {
		coinbaseSet[coinbase.String()] = struct{}{}
	}
	for i := fromLayer; i <= toLayer; i++ {
		id, err := layers.GetApplied(db, types.LayerID(i))
		must(err)
		if id != types.EmptyBlockID {
			block, err := blocks.Get(db, id)
			must(err)
			included += float64(len(block.Rewards))
			for _, reward := range block.Rewards {
				atx, err := atxs.Get(db, reward.AtxID)
				must(err)
				coinbase := atx.Coinbase.String()
				if _, ok := coinbaseSet[coinbase]; ok {
					included_coinbases += 1
				} else {
					included_not_coinbases += 1
				}
			}
		}

		ballots, err := ballots.Layer(db, types.LayerID(i))
		must(err)
		for _, ballot := range ballots {
			if ballot.IsMalicious() {
				continue
			}
			total += 1
			atx, err := atxs.Get(db, ballot.AtxID)
			must(err)
			coinbase := atx.Coinbase.String()
			if _, ok := coinbaseSet[coinbase]; ok {
				total_coinbases += 1
			} else {
				total_not_coinbases += 1
			}
		}
	}
	fmt.Printf("from = %d to = %d average inclusion %f\n    coinbases %f (%d)\n    not coinbases %f(%d)\n", fromLayer, toLayer, included/total, included_coinbases/total_coinbases, uint64(total_coinbases), included_not_coinbases/total_not_coinbases, uint64(total_not_coinbases))
}

func main() {
	flag.Parse()
	db, err := sql.Open("file:" + *db)
	must(err)

	if *from <= 0 {
		*from = *from + *to
	}

	if *every > 0 {
		for i := *from; i <= *to; i += *every {
			calculateInclusionRates(db, i, i+*every-1)
		}
	} else {
		batchSize := (*to - *from + 1) / *batches
		for batch := 0; batch < *batches; batch++ {
			startLayer := *from + batch*batchSize
			endLayer := startLayer + batchSize - 1
			if batch == *batches-1 {
				endLayer = *to
			}
			calculateInclusionRates(db, startLayer, endLayer)
		}
	}

}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
