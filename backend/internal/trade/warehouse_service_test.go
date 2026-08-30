package trade

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidQuantityUsesFixedThreeDecimalPrecision(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"1", true}, {"0.001", true}, {"10.120", true}, {"0", false}, {"-1", false}, {"0.0001", false}, {"1000000000000000", false},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			if got := validQuantity(decimal.RequireFromString(tc.value)); got != tc.want {
				t.Fatalf("validQuantity(%s) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestMasterDataValidationAndTrimming(t *testing.T) {
	material := MaterialInput{Name: " 番茄 ", Category: " 作物 ", Unit: " kg "}
	if err := validateMaterial(&material); err != nil || material.Status != StatusActive {
		t.Fatalf("validate material: status=%q err=%v", material.Status, err)
	}
	warehouse := WarehouseInput{Name: " 成品仓 "}
	if err := validateWarehouse(&warehouse); err != nil || warehouse.Status != StatusActive {
		t.Fatalf("validate warehouse: status=%q err=%v", warehouse.Status, err)
	}
	invalid := MaterialInput{Name: "", Category: "作物", Unit: "kg"}
	if err := validateMaterial(&invalid); err != ErrInvalidInput {
		t.Fatalf("empty material name error = %v", err)
	}
}

func TestOutboundItemsHaveStableLockOrder(t *testing.T) {
	items := []OutboundItem{{WarehouseID: 2, MaterialID: 1}, {WarehouseID: 1, MaterialID: 9}, {WarehouseID: 1, MaterialID: 3}}
	ordered := orderedItems(items)
	if ordered[0].WarehouseID != 1 || ordered[0].MaterialID != 3 || ordered[1].MaterialID != 9 || ordered[2].WarehouseID != 2 {
		t.Fatalf("unexpected lock order: %+v", ordered)
	}
	if items[0].WarehouseID != 2 {
		t.Fatal("orderedItems mutated caller input")
	}
}
