package logstorage

import (
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterRange matches the given range [minValue..maxValue].
//
// Example LogsQL: `fieldName:range(minValue, maxValue]`
type filterRange struct {
	fieldName string

	minValue float64
	maxValue float64

	stringRepr string
}

func (fr *filterRange) String() string {
	return quoteFieldNameIfNeeded(fr.fieldName) + fr.stringRepr
}

func (fr *filterRange) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fr.fieldName)
}

func (fr *filterRange) applyToBlockResult(br *blockResult, bm *bitmap) {
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fr.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchRange(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		bm.resetBits()
		return
	}

	switch c.valueType {
	case valueTypeString:
		values := c.getValues(br)
		bm.forEachSetBit(func(idx int) bool {
			v := values[idx]
			return matchRange(v, minValue, maxValue)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchRange(v, minValue, maxValue) {
				c = 1
			}
			bb.B = append(bb.B, c)
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := valuesEncoded[idx][0]
			return bb.B[n] == 1
		})
		bbPool.Put(bb)
	case valueTypeUint8:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := uint64(unmarshalUint8(v))
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeUint16:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := uint64(unmarshalUint16(v))
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeUint32:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := uint64(unmarshalUint32(v))
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeUint64:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := unmarshalUint64(v)
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeFloat64:
		if minValue > math.Float64frombits(c.maxValue) || maxValue < math.Float64frombits(c.minValue) {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			f := unmarshalFloat64(v)
			return f >= minValue && f <= maxValue
		})
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fr *filterRange) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchRange(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint8:
		matchUint8ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint16:
		matchUint16ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint32:
		matchUint32ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint64:
		matchUint64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeFloat64:
		matchFloat64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeIPv4:
		bm.resetBits()
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchFloat64ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	if minValue > math.Float64frombits(ch.maxValue) || maxValue < math.Float64frombits(ch.minValue) {
		bm.resetBits()
		return
	}

	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of floating-point number: got %d; want 8", bs.partPath(), len(v))
		}
		f := unmarshalFloat64(v)
		return f >= minValue && f <= maxValue
	})
}

func matchValuesDictByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchRange(v, minValue, maxValue) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchRange(v, minValue, maxValue)
	})
}

func matchUint8ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 1 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 1", bs.partPath(), len(v))
		}
		n := uint64(unmarshalUint8(v))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint16ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 2 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint16 number: got %d; want 2", bs.partPath(), len(v))
		}
		n := uint64(unmarshalUint16(v))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint32ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 4 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 4", bs.partPath(), len(v))
		}
		n := uint64(unmarshalUint32(v))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint64ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 8", bs.partPath(), len(v))
		}
		n := unmarshalUint64(v)
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchRange(s string, minValue, maxValue float64) bool {
	f, ok := tryParseNumber(s)
	if !ok {
		return false
	}
	return f >= minValue && f <= maxValue
}

func tryParseNumber(s string) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	f, ok := tryParseFloat64(s)
	if ok {
		return f, true
	}
	nsecs, ok := tryParseDuration(s)
	if ok {
		return float64(nsecs), true
	}
	bytes, ok := tryParseBytes(s)
	if ok {
		return float64(bytes), true
	}
	return 0, false
}

func toUint64Range(minValue, maxValue float64) (uint64, uint64) {
	minValue = math.Ceil(minValue)
	maxValue = math.Floor(maxValue)
	return toUint64Clamp(minValue), toUint64Clamp(maxValue)
}

func toUint64Clamp(f float64) uint64 {
	if f < 0 {
		return 0
	}
	if f > math.MaxUint64 {
		return math.MaxUint64
	}
	return uint64(f)
}
