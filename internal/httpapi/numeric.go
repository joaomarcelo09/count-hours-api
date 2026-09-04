package httpapi

import (
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

func floatFromNumeric(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', 2, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}