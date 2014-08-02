package indicators

import (
	"container/list"
	"errors"
	"github.com/thetruetrade/gotrade"
	"math"
)

// A Kaufman Adaptive Moving Average Indicator (Kama), no storage, for use in other indicators
type KamaWithoutStorage struct {
	*baseIndicator
	*baseFloatBounds

	// private variables
	periodTotal          float64
	periodHistory        *list.List
	periodCounter        int
	constantMax          float64
	constantDiff         float64
	sumROC               float64
	periodROC            float64
	previousClose        float64
	previousKama         float64
	valueAvailableAction ValueAvailableActionFloat
	timePeriod           int
}

// NewKamaWithoutStorage creates a Kaufman Adaptive Moving Average Indicator (Kama) without storage
func NewKamaWithoutStorage(timePeriod int, valueAvailableAction ValueAvailableActionFloat) (indicator *KamaWithoutStorage, err error) {

	// an indicator without storage MUST have a value available action
	if valueAvailableAction == nil {
		return nil, ErrValueAvailableActionIsNil
	}

	// the minimum timeperiod for this indicator is 2
	if timePeriod < 2 {
		return nil, errors.New("timePeriod is less than the minimum (2)")
	}

	// check the maximum timeperiod
	if timePeriod > MaximumLookbackPeriod {
		return nil, errors.New("timePeriod is greater than the maximum (100000)")
	}

	lookback := timePeriod
	ind := KamaWithoutStorage{
		baseIndicator:        newBaseIndicator(lookback),
		baseFloatBounds:      newBaseFloatBounds(),
		periodCounter:        (timePeriod + 1) * -1,
		constantMax:          float64(2.0 / (30.0 + 1.0)),
		constantDiff:         float64((2.0 / (2.0 + 1.0)) - (2.0 / (30.0 + 1.0))),
		sumROC:               0.0,
		periodROC:            0.0,
		periodHistory:        list.New(),
		previousClose:        math.SmallestNonzeroFloat64,
		valueAvailableAction: valueAvailableAction,
		timePeriod:           timePeriod,
	}

	return &ind, nil
}

// A Kaufman Adaptive Moving Average Indicator (Kama)
type Kama struct {
	*KamaWithoutStorage
	selectData gotrade.DataSelectionFunc

	// public variables
	Data []float64
}

// NewKama creates a Kaufman Adaptive Moving Average Indicator (Kama) for online usage
func NewKama(timePeriod int, selectData gotrade.DataSelectionFunc) (indicator *Kama, err error) {
	ind := Kama{selectData: selectData}
	ind.KamaWithoutStorage, err = NewKamaWithoutStorage(timePeriod, func(dataItem float64, streamBarIndex int) {
		ind.Data = append(ind.Data, dataItem)
	})

	return &ind, err
}

// NewDefaultKama creates a Kaufman Adaptive Moving Average Indicator (Kama) for online usage with default parameters
//	- timePeriod: 25
func NewDefaultKama() (indicator *Kama, err error) {
	timePeriod := 25
	return NewKama(timePeriod, gotrade.UseClosePrice)
}

// NewKamaWithKnownSourceLength creates a Kaufman Adaptive Moving Average Indicator (Kama) for offline usage
func NewKamaWithKnownSourceLength(sourceLength int, timePeriod int, selectData gotrade.DataSelectionFunc) (indicator *Kama, err error) {
	ind, err := NewKama(timePeriod, selectData)
	ind.Data = make([]float64, 0, sourceLength-ind.GetLookbackPeriod())

	return ind, err
}

// NewDefaultKamaWithKnownSourceLength creates a Kaufman Adaptive Moving Average Indicator (Kama) for offline usage with default parameters
func NewDefaultKamaWithKnownSourceLength(sourceLength int) (indicator *Kama, err error) {
	ind, err := NewDefaultKama()
	ind.Data = make([]float64, 0, sourceLength-ind.GetLookbackPeriod())
	return ind, err
}

// NewKamaForStream creates a Kaufman Adaptive Moving Average Indicator (Kama) for online usage with a source data stream
func NewKamaForStream(priceStream *gotrade.DOHLCVStream, timePeriod int, selectData gotrade.DataSelectionFunc) (indicator *Kama, err error) {
	ind, err := NewKama(timePeriod, selectData)
	priceStream.AddTickSubscription(ind)
	return ind, err
}

// NewDefaultKamaForStream creates a Kaufman Adaptive Moving Average Indicator (Kama) for online usage with a source data stream
func NewDefaultKamaForStream(priceStream *gotrade.DOHLCVStream) (indicator *Kama, err error) {
	ind, err := NewDefaultKama()
	priceStream.AddTickSubscription(ind)
	return ind, err
}

// NewKamaForStreamWithKnownSourceLength creates a Kaufman Adaptive Moving Average Indicator (Kama) for offline usage with a source data stream
func NewKamaForStreamWithKnownSourceLength(sourceLength int, priceStream *gotrade.DOHLCVStream, timePeriod int, selectData gotrade.DataSelectionFunc) (indicator *Kama, err error) {
	ind, err := NewKamaWithKnownSourceLength(sourceLength, timePeriod, selectData)
	priceStream.AddTickSubscription(ind)
	return ind, err
}

// NewDefaultKamaForStreamWithKnownSourceLength creates a Kaufman Adaptive Moving Average Indicator (Kama) for offline usage with a source data stream
func NewDefaultKamaForStreamWithKnownSourceLength(sourceLength int, priceStream *gotrade.DOHLCVStream) (indicator *Kama, err error) {
	ind, err := NewDefaultKamaWithKnownSourceLength(sourceLength)
	priceStream.AddTickSubscription(ind)
	return ind, err
}

// ReceiveDOHLCVTick consumes a source data DOHLCV price tick
func (ind *Kama) ReceiveDOHLCVTick(tickData gotrade.DOHLCV, streamBarIndex int) {
	var selectedData = ind.selectData(tickData)
	ind.ReceiveTick(selectedData, streamBarIndex)
}

func (ind *KamaWithoutStorage) ReceiveTick(tickData float64, streamBarIndex int) {
	ind.periodCounter += 1
	ind.periodHistory.PushBack(tickData)

	if ind.periodCounter <= 0 {
		if ind.previousClose > math.SmallestNonzeroFloat64 {
			ind.sumROC += math.Abs(tickData - ind.previousClose)
		}
	}
	if ind.periodCounter == 0 {
		var er float64 = 0.0
		var sc float64 = 0.0
		var closeMinusN float64 = ind.periodHistory.Front().Value.(float64)
		ind.previousKama = ind.previousClose
		ind.periodROC = tickData - closeMinusN

		// calculate the efficiency ratio
		if ind.sumROC <= ind.periodROC || isZero(ind.sumROC) {
			er = 1.0
		} else {
			er = math.Abs(ind.periodROC / ind.sumROC)
		}

		sc = (er * ind.constantDiff) + ind.constantMax
		sc *= sc
		ind.previousKama = ((tickData - ind.previousKama) * sc) + ind.previousKama

		result := ind.previousKama

		// increment the number of results this indicator can be expected to return
		ind.dataLength += 1

		if ind.validFromBar == -1 {
			// set the streamBarIndex from which this indicator returns valid results
			ind.validFromBar = streamBarIndex
		}

		// update the maximum result value
		if result > ind.maxValue {
			ind.maxValue = result
		}

		// update the minimum result value
		if result < ind.minValue {
			ind.minValue = result
		}

		// notify of a new result value though the value available action
		ind.valueAvailableAction(result, streamBarIndex)

	} else if ind.periodCounter > 0 {

		var er float64 = 0.0
		var sc float64 = 0.0
		var closeMinusN float64 = ind.periodHistory.Front().Value.(float64)
		var closeMinusN1 float64 = ind.periodHistory.Front().Next().Value.(float64)
		ind.periodROC = tickData - closeMinusN1

		ind.sumROC -= math.Abs(closeMinusN1 - closeMinusN)
		ind.sumROC += math.Abs(tickData - ind.previousClose)

		// calculate the efficiency ratio
		if ind.sumROC <= ind.periodROC || isZero(ind.sumROC) {
			er = 1.0
		} else {
			er = math.Abs(ind.periodROC / ind.sumROC)
		}

		sc = (er * ind.constantDiff) + ind.constantMax
		sc *= sc
		ind.previousKama = ((tickData - ind.previousKama) * sc) + ind.previousKama

		result := ind.previousKama

		// increment the number of results this indicator can be expected to return
		ind.dataLength += 1

		// update the maximum result value
		if result > ind.maxValue {
			ind.maxValue = result
		}

		// update the minimum result value
		if result < ind.minValue {
			ind.minValue = result
		}

		// notify of a new result value though the value available action
		ind.valueAvailableAction(result, streamBarIndex)
	}

	ind.previousClose = tickData

	if ind.periodHistory.Len() > (ind.timePeriod + 1) {
		var first = ind.periodHistory.Front()
		ind.periodHistory.Remove(first)
	}

}

func isZero(value float64) bool {
	var epsilon float64 = 0.00000000000001
	return (((-epsilon) < value) && (value < epsilon))
}
