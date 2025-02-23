// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package decoder

import (
	"sort"
	"time"

	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/rowcodec"
)

// Column contains the info and generated expr of column.
type Column struct {
	Col     *table.Column
	GenExpr expression.Expression
}

// RowDecoder decodes a byte slice into datums and eval the generated column value.
type RowDecoder struct {
	tbl         table.Table
	mutRow      chunk.MutRow
	colMap      map[int64]Column
	colTypes    map[int64]*types.FieldType
	defaultVals []types.Datum
	cols        []*table.Column
	pkCols      []int64
}

// NewRowDecoder returns a new RowDecoder.
func NewRowDecoder(tbl table.Table, cols []*table.Column, decodeColMap map[int64]Column) *RowDecoder {
	tblInfo := tbl.Meta()
	colFieldMap := make(map[int64]*types.FieldType, len(decodeColMap))
	for id, col := range decodeColMap {
		colFieldMap[id] = &col.Col.ColumnInfo.FieldType
	}

	tps := make([]*types.FieldType, len(cols))
	for _, col := range cols {
		// Even for changing column in column type change, we target field type uniformly.
		tps[col.Offset] = &col.FieldType
	}
	var pkCols []int64
	switch {
	case tblInfo.IsCommonHandle:
		pkCols = tables.TryGetCommonPkColumnIds(tbl.Meta())
	case tblInfo.PKIsHandle:
		pkCols = []int64{tblInfo.GetPkColInfo().ID}
	default: // support decoding _tidb_rowid.
		pkCols = []int64{model.ExtraHandleID}
	}
	return &RowDecoder{
		tbl:         tbl,
		mutRow:      chunk.MutRowFromTypes(tps),
		colMap:      decodeColMap,
		colTypes:    colFieldMap,
		defaultVals: make([]types.Datum, len(cols)),
		cols:        cols,
		pkCols:      pkCols,
	}
}

// DecodeAndEvalRowWithMap decodes a byte slice into datums and evaluates the generated column value.
func (rd *RowDecoder) DecodeAndEvalRowWithMap(ctx sessionctx.Context, handle kv.Handle, b []byte, decodeLoc, sysLoc *time.Location, row map[int64]types.Datum) (map[int64]types.Datum, error) {
	var err error
	if rowcodec.IsNewFormat(b) {
		row, err = tablecodec.DecodeRowWithMapNew(b, rd.colTypes, decodeLoc, row)
	} else {
		row, err = tablecodec.DecodeRowWithMap(b, rd.colTypes, decodeLoc, row)
	}
	if err != nil {
		return nil, err
	}
	row, err = tablecodec.DecodeHandleToDatumMap(handle, rd.pkCols, rd.colTypes, decodeLoc, row)
	if err != nil {
		return nil, err
	}
	for _, dCol := range rd.colMap {
		colInfo := dCol.Col.ColumnInfo
		val, ok := row[colInfo.ID]
		if ok || dCol.GenExpr != nil {
			rd.mutRow.SetValue(colInfo.Offset, val.GetValue())
			continue
		}
		if dCol.Col.ChangeStateInfo != nil {
			val, _, err = tables.GetChangingColVal(ctx, rd.cols, dCol.Col, row, rd.defaultVals)
		} else {
			// Get the default value of the column in the generated column expression.
			val, err = tables.GetColDefaultValue(ctx, dCol.Col, rd.defaultVals)
		}
		if err != nil {
			return nil, err
		}
		rd.mutRow.SetValue(colInfo.Offset, val.GetValue())
	}
	return rd.EvalRemainedExprColumnMap(ctx, sysLoc, row)
}

// BuildFullDecodeColMap builds a map that contains [columnID -> struct{*table.Column, expression.Expression}] from all columns.
func BuildFullDecodeColMap(cols []*table.Column, schema *expression.Schema) map[int64]Column {
	decodeColMap := make(map[int64]Column, len(cols))
	for _, col := range cols {
		decodeColMap[col.ID] = Column{
			Col:     col,
			GenExpr: schema.Columns[col.Offset].VirtualExpr,
		}
	}
	return decodeColMap
}

// CurrentRowWithDefaultVal returns current decoding row with default column values set properly.
// Please make sure calling DecodeAndEvalRowWithMap first.
func (rd *RowDecoder) CurrentRowWithDefaultVal() chunk.Row {
	return rd.mutRow.ToRow()
}

// DecodeTheExistedColumnMap is used by ddl column-type-change first column reorg stage.
// In the function, we only decode the existed column in the row and fill the default value.
// For changing column, we shouldn't cast it here, because we will do a unified cast operation latter.
// For generated column, we didn't cast it here too, because the eval process will depend on the changing column.
func (rd *RowDecoder) DecodeTheExistedColumnMap(ctx sessionctx.Context, handle kv.Handle, b []byte, decodeLoc *time.Location, row map[int64]types.Datum) (map[int64]types.Datum, error) {
	var err error
	if rowcodec.IsNewFormat(b) {
		row, err = tablecodec.DecodeRowWithMapNew(b, rd.colTypes, decodeLoc, row)
	} else {
		row, err = tablecodec.DecodeRowWithMap(b, rd.colTypes, decodeLoc, row)
	}
	if err != nil {
		return nil, err
	}
	row, err = tablecodec.DecodeHandleToDatumMap(handle, rd.pkCols, rd.colTypes, decodeLoc, row)
	if err != nil {
		return nil, err
	}
	for _, dCol := range rd.colMap {
		colInfo := dCol.Col.ColumnInfo
		val, ok := row[colInfo.ID]
		if ok || dCol.GenExpr != nil || dCol.Col.ChangeStateInfo != nil {
			rd.mutRow.SetValue(colInfo.Offset, val.GetValue())
			continue
		}
		// Get the default value of the column in the generated column expression.
		val, err = tables.GetColDefaultValue(ctx, dCol.Col, rd.defaultVals)
		if err != nil {
			return nil, err
		}
		// Fill the default value into map.
		row[colInfo.ID] = val
		rd.mutRow.SetValue(colInfo.Offset, val.GetValue())
	}
	// return the existed column map here.
	return row, nil
}

// EvalRemainedExprColumnMap is used by ddl column-type-change first column reorg stage.
// It is always called after DecodeTheExistedColumnMap to finish the generated column evaluation.
func (rd *RowDecoder) EvalRemainedExprColumnMap(ctx sessionctx.Context, sysLoc *time.Location, row map[int64]types.Datum) (map[int64]types.Datum, error) {
	keys := make([]int, 0, len(rd.colMap))
	ids := make(map[int]int, len(rd.colMap))
	for k, col := range rd.colMap {
		keys = append(keys, col.Col.Offset)
		ids[col.Col.Offset] = int(k)
	}
	sort.Ints(keys)
	for _, id := range keys {
		col := rd.colMap[int64(ids[id])]
		if col.GenExpr == nil {
			continue
		}
		// Eval the column value
		val, err := col.GenExpr.Eval(rd.mutRow.ToRow())
		if err != nil {
			return nil, err
		}
		val, err = table.CastValue(ctx, val, col.Col.ColumnInfo, false, true)
		if err != nil {
			return nil, err
		}

		if val.Kind() == types.KindMysqlTime && sysLoc != time.UTC {
			t := val.GetMysqlTime()
			if t.Type() == mysql.TypeTimestamp {
				err := t.ConvertTimeZone(sysLoc, time.UTC)
				if err != nil {
					return nil, err
				}
				val.SetMysqlTime(t)
			}
		}
		rd.mutRow.SetValue(col.Col.Offset, val.GetValue())

		row[int64(ids[id])] = val
	}
	// return the existed and evaluated column map here.
	return row, nil
}
