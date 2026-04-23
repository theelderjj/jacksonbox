package priceisright

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed products_snapshot.json
var productsSnapshotJSON []byte

var products = mustLoadProducts()

func mustLoadProducts() []Product {
	var out []Product
	if err := json.Unmarshal(productsSnapshotJSON, &out); err != nil {
		panic(fmt.Errorf("load price is right snapshot: %w", err))
	}
	if len(out) == 0 {
		panic("load price is right snapshot: empty product list")
	}
	return out
}
