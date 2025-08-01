package remotewrite

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestApplyRelabeling(t *testing.T) {
	f := func(pcs *promrelabel.ParsedConfigs, sTss, sExpTss string) {
		rctx := &relabelCtx{}
		tss, expTss := parseSeries(sTss), parseSeries(sExpTss)
		gotTss := rctx.applyRelabeling(tss, pcs)
		if !reflect.DeepEqual(gotTss, expTss) {
			t.Fatalf("expected to have: \n%v;\ngot: \n%v", expTss, gotTss)
		}
	}

	f(nil, "up", "up")

	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- target_label: "foo"
  replacement: "aaa"
- action: labeldrop
  regex: "env.*"
`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f(pcs, `up{foo="baz", env="prod"}`, `up{foo="aaa"}`)

	oldVal := *usePromCompatibleNaming
	*usePromCompatibleNaming = true
	f(nil, `foo.bar`, `foo_bar`)
	*usePromCompatibleNaming = oldVal
}

func TestAppendExtraLabels(t *testing.T) {
	f := func(extraLabels []prompb.Label, sTss, sExpTss string) {
		t.Helper()
		rctx := &relabelCtx{}
		tss, expTss := parseSeries(sTss), parseSeries(sExpTss)
		rctx.appendExtraLabels(tss, extraLabels)
		if !reflect.DeepEqual(tss, expTss) {
			t.Fatalf("expected to have: \n%v;\ngot: \n%v", expTss, tss)
		}
	}

	f(nil, "up", "up")
	f([]prompb.Label{{Name: "foo", Value: "bar"}}, "up", `up{foo="bar"}`)
	f([]prompb.Label{{Name: "foo", Value: "bar"}}, `up{foo="baz"}`, `up{foo="bar"}`)
	f([]prompb.Label{{Name: "baz", Value: "qux"}}, `up{foo="baz"}`, `up{foo="baz",baz="qux"}`)

	oldVal := *usePromCompatibleNaming
	*usePromCompatibleNaming = true
	f([]prompb.Label{{Name: "foo.bar", Value: "baz"}}, "up", `up{foo.bar="baz"}`)
	*usePromCompatibleNaming = oldVal
}

func parseSeries(data string) []prompb.TimeSeries {
	var tss []prompb.TimeSeries
	tss = append(tss, prompb.TimeSeries{
		Labels: promutil.MustNewLabelsFromString(data).GetLabels(),
	})
	return tss
}
