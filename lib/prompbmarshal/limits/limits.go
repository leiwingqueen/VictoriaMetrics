package limits

import (
	"flag"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var (
	maxLabelsPerTimeseries = flag.Int("maxLabelsPerTimeseries", 30, "The maximum number of labels per time series to be accepted. Timeseries with superfluous labels are dropped. In this case the vm_series_dropped_total{reason=\"too_many_labels\"} metric at /metrics page is incremented")
	maxLabelNameLen        = flag.Int("maxLabelNameLen", 256, "The maximum length of label name in the accepted time series. Metrics with longer label name are dropped. In this case the vm_series_dropped_total{reason=\"too_long_label_name\"} metric at /metrics page is incremented")
	maxLabelValueLen       = flag.Int("maxLabelValueLen", 4*1024, "The maximum length of label values in the accepted time series. Metrics with longer label value are dropped. In this case the vm_series_dropped_total{reason=\"too_long_label_value\"} metric at /metrics page is incremented")
)

var (
	droppedSeriesWithTooManyLabelsLogTicker     = time.NewTicker(5 * time.Second)
	droppedSeriesWithTooLongLabelNameLogTicker  = time.NewTicker(5 * time.Second)
	droppedSeriesWithTooLongLabelValueLogTicker = time.NewTicker(5 * time.Second)
)

var (
	// droppedSeriesWithTooManyLabels is the number of dropped series with too many labels
	droppedSeriesWithTooManyLabels atomic.Uint64

	// droppedSeriesWithTooLongLabelName is the number of dropped series which contain labels with too long names
	droppedSeriesWithTooLongLabelName atomic.Uint64

	// droppedSeriesWithTooLongLabelValue is the number of dropped series which contain labels with too long values
	droppedSeriesWithTooLongLabelValue atomic.Uint64
)

var (
	_ = metrics.NewGauge(`vm_series_dropped_total{reason="too_many_labels"}`, func() float64 {
		return float64(droppedSeriesWithTooManyLabels.Load())
	})
	_ = metrics.NewGauge(`vm_series_dropped_total{reason="too_long_label_name"}`, func() float64 {
		return float64(droppedSeriesWithTooLongLabelName.Load())
	})
	_ = metrics.NewGauge(`vm_series_dropped_total{reason="too_long_label_value"}`, func() float64 {
		return float64(droppedSeriesWithTooLongLabelValue.Load())
	})
)

func trackDroppedSeriesWithTooManyLabels(labels []prompbmarshal.Label) {
	droppedSeriesWithTooManyLabels.Add(1)
	select {
	case <-droppedSeriesWithTooManyLabelsLogTicker.C:
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		// because LabelsToString() will be called with each trackDroppedSeriesWithTooManyLabels call.
		logger.Warnf("dropping series with %d labels for %s; either reduce the number of labels for this metric "+
			"or increase -maxLabelsPerTimeseries=%d command-line flag value",
			len(labels), LabelsToString(labels), *maxLabelsPerTimeseries)
	default:
	}
}

func trackDroppedSeriesWithTooLongLabelValue(l *prompbmarshal.Label, labels []prompbmarshal.Label) {
	droppedSeriesWithTooLongLabelValue.Add(1)
	select {
	case <-droppedSeriesWithTooLongLabelValueLogTicker.C:
		label := *l
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		// because LabelsToString() will be called with each trackDroppedSeriesWithTooLongLabelValue call.
		logger.Warnf("drop series with a value %s for label %s because its length=%d exceeds -maxLabelValueLen=%d; "+
			"original labels: %s; either reduce the label value length or increase -maxLabelValueLen command-line flag value",
			label.Value, label.Name, len(label.Value), *maxLabelValueLen, LabelsToString(labels))
	default:
	}
}

func trackDroppedSeriesWithTooLongLabelName(l *prompbmarshal.Label, labels []prompbmarshal.Label) {
	droppedSeriesWithTooLongLabelName.Add(1)
	select {
	case <-droppedSeriesWithTooLongLabelNameLogTicker.C:
		label := *l
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		// because LabelsToString() will be called with each trackDroppedSeriesWithTooLongLabelName call.
		logger.Warnf("drop series with a value for label %s because its length=%d exceeds -maxLabelNameLen=%d; "+
			"original labels: %s; either reduce the label value length or increase -maxLabelNameLen command-line flag value",
			label.Name, len(label.Name), *maxLabelNameLen, LabelsToString(labels))
	default:
	}
}

// LabelsToString converts labels to Prometheus-compatible string
func LabelsToString(labels []prompbmarshal.Label) string {
	labelsCopy := append([]prompbmarshal.Label{}, labels...)
	sort.Slice(labelsCopy, func(i, j int) bool {
		return string(labelsCopy[i].Name) < string(labelsCopy[j].Name)
	})
	var b []byte
	b = append(b, '{')
	for i, label := range labelsCopy {
		if len(label.Name) == 0 {
			b = append(b, "__name__"...)
		} else {
			b = append(b, label.Name...)
		}
		b = append(b, '=')
		b = strconv.AppendQuote(b, label.Value)
		if i < len(labels)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return string(b)
}

// ExceedingLabels checks if passed labels exceed one of the limits:
// * Maximum allowed labels limit
// * Maximum allowed label name length limit
// * Maximum allowed label value length limit
//
// increments metrics and shows warning in logs
func ExceedingLabels(labels []prompbmarshal.Label) bool {
	if len(labels) > *maxLabelsPerTimeseries {
		trackDroppedSeriesWithTooManyLabels(labels)
		return true
	}
	for _, l := range labels {
		if len(l.Name) > *maxLabelNameLen {
			trackDroppedSeriesWithTooLongLabelName(&l, labels)
			return true
		}
		if len(l.Value) > *maxLabelValueLen {
			trackDroppedSeriesWithTooLongLabelValue(&l, labels)
			return true
		}
	}
	return false
}

// AreLabelsExceeding checks if passed labels exceed one of the limits:
// * Maximum allowed labels limit
// * Maximum allowed label name length limit
// * Maximum allowed label value length limit
func AreLabelsExceeding(labels []prompbmarshal.Label) bool {
	if len(labels) > *maxLabelsPerTimeseries {
		return true
	}
	for _, l := range labels {
		if len(l.Name) > *maxLabelNameLen {
			return true
		}
		if len(l.Value) > *maxLabelValueLen {
			return true
		}
	}
	return false
}
