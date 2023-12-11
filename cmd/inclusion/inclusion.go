package main

import (
	"flag"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
)

var (
	db      = flag.String("db", "", "database path")
	from    = flag.Int("from", 0, "from layer")
	to      = flag.Int("to", 0, "to layer")
	batches = flag.Int("batches", 1, "number of batches")
)

func calculateInclusionRates(db *sql.Database, fromLayer, toLayer int) {
	var (
		included float64
		total    float64
	)
	for i := fromLayer; i <= toLayer; i++ {
		id, err := layers.GetApplied(db, types.LayerID(i))
		must(err)
		if id != types.EmptyBlockID {
			block, err := blocks.Get(db, id)
			must(err)
			included += float64(len(block.Rewards))
		}
		ballots, err := ballots.Layer(db, types.LayerID(i))
		must(err)
		for _, ballot := range ballots {
			if ballot.IsMalicious() {
				continue
			}
			total += 1
		}
	}
	fmt.Printf("from = %d to = %d average inclusion %f\n", *from, *to, included/total)
}

func main() {
	flag.Parse()
	db, err := sql.Open("file:" + *db)
	must(err)

	if *from <= 0 {
		*from = *from + *to
	}

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

func must(err error) {
	if err != nil {
		panic(err)
	}
}
